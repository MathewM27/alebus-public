package redis

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Phase 5: Observability at Scale - Prometheus-Compatible Metrics
// ─────────────────────────────────────────────────────────────────────────────
//
// This module provides fine-grained metrics for production monitoring at 10k+ buses.
// Metrics are designed to be scraped by Prometheus and visualized in Grafana.
//
// Key metrics:
//   - Latency percentiles (P50, P95, P99) per operation type
//   - Throughput (ops/sec) per handler, route, and region
//   - Resource usage (memory, connections, CPU)
//   - Bus state metrics (active, offline, by route)
//   - Error rates and categorization

// ─────────────────────────────────────────────────────────────────────────────
// Operation Types
// ─────────────────────────────────────────────────────────────────────────────

// OperationType categorizes Redis operations for metrics.
type OperationType string

const (
	OpTypeWrite       OperationType = "write"
	OpTypeRead        OperationType = "read"
	OpTypeLuaScript   OperationType = "lua_script"
	OpTypeGeoAdd      OperationType = "geo_add"
	OpTypeGeoQuery    OperationType = "geo_query"
	OpTypeHTTPHandler OperationType = "http_handler"
	OpTypePipeline    OperationType = "pipeline"
	OpTypeTransaction OperationType = "transaction"
)

// ErrorCategory categorizes errors for better observability.
type ErrorCategory string

const (
	ErrorCategoryRedis      ErrorCategory = "redis"
	ErrorCategoryLua        ErrorCategory = "lua"
	ErrorCategoryClient     ErrorCategory = "client"
	ErrorCategoryTimeout    ErrorCategory = "timeout"
	ErrorCategoryValidation ErrorCategory = "validation"
	ErrorCategoryNetwork    ErrorCategory = "network"
)

// ─────────────────────────────────────────────────────────────────────────────
// Prometheus-Compatible Metric Types
// ─────────────────────────────────────────────────────────────────────────────

// PrometheusMetric represents a metric that can be exported to Prometheus.
type PrometheusMetric struct {
	Name         string
	Help         string
	Type         string // counter, gauge, histogram
	Labels       map[string]string
	Value        float64
	Buckets      []float64 // for histograms
	Observations []float64 // for histograms
}

// MetricLabels holds common labels for metrics tagging.
type MetricLabels struct {
	RouteID   string
	BusID     string
	Region    string
	Handler   string
	Operation string
	Status    string
}

// ─────────────────────────────────────────────────────────────────────────────
// Enhanced Latency Tracker with Histograms
// ─────────────────────────────────────────────────────────────────────────────

// HistogramBuckets defines standard latency buckets in microseconds.
var HistogramBuckets = []float64{
	100,    // 0.1ms
	250,    // 0.25ms
	500,    // 0.5ms
	1000,   // 1ms
	2500,   // 2.5ms
	5000,   // 5ms
	10000,  // 10ms
	25000,  // 25ms
	50000,  // 50ms
	100000, // 100ms
}

// LatencyHistogram tracks latency with histogram buckets for Prometheus.
type LatencyHistogram struct {
	mu           sync.Mutex
	operation    OperationType
	buckets      []float64
	bucketCounts []int64
	sum          float64
	count        int64
	min          float64
	max          float64
	samples      []float64 // Rolling window for percentiles
	sampleSize   int
	position     int
}

// NewLatencyHistogram creates a new latency histogram.
func NewLatencyHistogram(op OperationType, buckets []float64, sampleSize int) *LatencyHistogram {
	if buckets == nil {
		buckets = HistogramBuckets
	}
	return &LatencyHistogram{
		operation:    op,
		buckets:      buckets,
		bucketCounts: make([]int64, len(buckets)+1),
		min:          math.MaxFloat64,
		max:          0,
		samples:      make([]float64, sampleSize),
		sampleSize:   sampleSize,
	}
}

// Observe records a latency observation in microseconds.
func (h *LatencyHistogram) Observe(latencyUs float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Update histogram buckets
	for i, bucket := range h.buckets {
		if latencyUs <= bucket {
			h.bucketCounts[i]++
			break
		}
		if i == len(h.buckets)-1 {
			h.bucketCounts[len(h.buckets)]++ // +Inf bucket
		}
	}

	// Update statistics
	h.sum += latencyUs
	h.count++
	if latencyUs < h.min {
		h.min = latencyUs
	}
	if latencyUs > h.max {
		h.max = latencyUs
	}

	// Rolling window for percentiles
	h.samples[h.position] = latencyUs
	h.position = (h.position + 1) % h.sampleSize
}

// Percentile calculates the latency at the given percentile (0-100).
func (h *LatencyHistogram) Percentile(p float64) float64 {
	h.mu.Lock()
	defer h.mu.Unlock()

	n := int(h.count)
	if n == 0 {
		return 0
	}
	if n > h.sampleSize {
		n = h.sampleSize
	}

	sorted := make([]float64, n)
	copy(sorted, h.samples[:n])
	sort.Float64s(sorted)

	idx := int(float64(n-1) * p / 100.0)
	return sorted[idx]
}

// Stats returns histogram statistics.
func (h *LatencyHistogram) Stats() (count int64, sum, min, max, p50, p95, p99 float64) {
	h.mu.Lock()
	count = h.count
	sum = h.sum
	min = h.min
	max = h.max
	h.mu.Unlock()

	if count == 0 {
		min = 0
	}

	p50 = h.Percentile(50)
	p95 = h.Percentile(95)
	p99 = h.Percentile(99)
	return
}

// BucketCounts returns a copy of bucket counts.
func (h *LatencyHistogram) BucketCounts() []int64 {
	h.mu.Lock()
	defer h.mu.Unlock()

	counts := make([]int64, len(h.bucketCounts))
	copy(counts, h.bucketCounts)
	return counts
}

// ─────────────────────────────────────────────────────────────────────────────
// Throughput Tracker
// ─────────────────────────────────────────────────────────────────────────────

// ThroughputTracker tracks operations per second over time windows.
type ThroughputTracker struct {
	mu          sync.Mutex
	windowSize  time.Duration
	bucketSize  time.Duration
	buckets     []int64
	bucketTimes []time.Time
	totalOps    int64
	startTime   time.Time
}

// NewThroughputTracker creates a new throughput tracker.
func NewThroughputTracker(windowSize, bucketSize time.Duration) *ThroughputTracker {
	numBuckets := int(windowSize / bucketSize)
	if numBuckets < 1 {
		numBuckets = 1
	}
	return &ThroughputTracker{
		windowSize:  windowSize,
		bucketSize:  bucketSize,
		buckets:     make([]int64, numBuckets),
		bucketTimes: make([]time.Time, numBuckets),
		startTime:   time.Now(),
	}
}

// Record records an operation.
func (t *ThroughputTracker) Record() {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	bucketIdx := int(now.Sub(t.startTime)/t.bucketSize) % len(t.buckets)

	// Reset bucket if it's from a previous window
	if t.bucketTimes[bucketIdx].Add(t.windowSize).Before(now) {
		t.buckets[bucketIdx] = 0
	}

	t.buckets[bucketIdx]++
	t.bucketTimes[bucketIdx] = now
	t.totalOps++
}

// OpsPerSecond returns the current operations per second.
func (t *ThroughputTracker) OpsPerSecond() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-t.windowSize)
	var total int64

	for i, ts := range t.bucketTimes {
		if ts.After(cutoff) {
			total += t.buckets[i]
		}
	}

	return float64(total) / t.windowSize.Seconds()
}

// TotalOps returns the total operations since start.
func (t *ThroughputTracker) TotalOps() int64 {
	return atomic.LoadInt64(&t.totalOps)
}

// ─────────────────────────────────────────────────────────────────────────────
// Enhanced Metrics Collector for Scale
// ─────────────────────────────────────────────────────────────────────────────

// ScaleMetricsCollector provides comprehensive metrics for 10k+ bus scale.
type ScaleMetricsCollector struct {
	client *Client

	// Latency histograms by operation type
	latencyHistograms map[OperationType]*LatencyHistogram
	histogramMu       sync.RWMutex

	// Throughput trackers by handler/operation
	throughputTrackers map[string]*ThroughputTracker
	throughputMu       sync.RWMutex

	// Error counters by category
	errorCounts   map[ErrorCategory]*int64
	errorCountsMu sync.RWMutex

	// Route-level metrics
	routeMetrics   map[string]*RouteMetrics
	routeMetricsMu sync.RWMutex

	// Time series data (last 30 minutes for recent trends)
	timeSeriesData   []TimeSeriesPoint
	timeSeriesMaxAge time.Duration
	timeSeriesMu     sync.RWMutex

	// Collection settings
	collectionInterval time.Duration
	startTime          time.Time
}

// RouteMetrics holds per-route metrics.
type RouteMetrics struct {
	RouteID       string
	ActiveBuses   int64
	OfflineBuses  int64
	TotalBuses    int64
	LastUpdate    time.Time
	UpdatesPerSec float64
	AvgLatencyUs  float64
}

// TimeSeriesPoint represents a metrics snapshot at a point in time.
type TimeSeriesPoint struct {
	Timestamp       time.Time
	WriteOps        int64
	ReadOps         int64
	WriteLatencyP99 float64
	ReadLatencyP99  float64
	ActiveBuses     int64
	OfflineBuses    int64
	MemoryUsedMB    float64
	OpsPerSecond    float64
}

// NewScaleMetricsCollector creates a new collector for scale observability.
func NewScaleMetricsCollector(client *Client) *ScaleMetricsCollector {
	c := &ScaleMetricsCollector{
		client:             client,
		latencyHistograms:  make(map[OperationType]*LatencyHistogram),
		throughputTrackers: make(map[string]*ThroughputTracker),
		errorCounts:        make(map[ErrorCategory]*int64),
		routeMetrics:       make(map[string]*RouteMetrics),
		timeSeriesData:     make([]TimeSeriesPoint, 0, 360), // 30 min at 5s intervals
		timeSeriesMaxAge:   30 * time.Minute,
		collectionInterval: 5 * time.Second,
		startTime:          time.Now(),
	}

	// Initialize latency histograms for each operation type
	for _, op := range []OperationType{OpTypeWrite, OpTypeRead, OpTypeLuaScript, OpTypeGeoAdd, OpTypeGeoQuery, OpTypeHTTPHandler, OpTypePipeline} {
		c.latencyHistograms[op] = NewLatencyHistogram(op, HistogramBuckets, 10000)
	}

	// Initialize error counters
	for _, cat := range []ErrorCategory{ErrorCategoryRedis, ErrorCategoryLua, ErrorCategoryClient, ErrorCategoryTimeout, ErrorCategoryValidation, ErrorCategoryNetwork} {
		zero := int64(0)
		c.errorCounts[cat] = &zero
	}

	return c
}

// RecordLatency records a latency observation for an operation type.
func (c *ScaleMetricsCollector) RecordLatency(op OperationType, latency time.Duration) {
	c.histogramMu.RLock()
	h, ok := c.latencyHistograms[op]
	c.histogramMu.RUnlock()

	if ok {
		h.Observe(float64(latency.Microseconds()))
	}
}

// RecordLatencyWithLabels records latency with additional labels for tagging.
func (c *ScaleMetricsCollector) RecordLatencyWithLabels(op OperationType, latency time.Duration, labels MetricLabels) {
	c.RecordLatency(op, latency)

	// Also track route-level metrics if RouteID provided
	if labels.RouteID != "" {
		c.updateRouteLatency(labels.RouteID, float64(latency.Microseconds()))
	}
}

// RecordThroughput records an operation for throughput tracking.
func (c *ScaleMetricsCollector) RecordThroughput(handlerName string) {
	c.throughputMu.Lock()
	tracker, ok := c.throughputTrackers[handlerName]
	if !ok {
		tracker = NewThroughputTracker(1*time.Minute, 1*time.Second)
		c.throughputTrackers[handlerName] = tracker
	}
	c.throughputMu.Unlock()

	tracker.Record()
}

// RecordError records an error by category.
func (c *ScaleMetricsCollector) RecordError(category ErrorCategory) {
	c.errorCountsMu.RLock()
	counter, ok := c.errorCounts[category]
	c.errorCountsMu.RUnlock()

	if ok {
		atomic.AddInt64(counter, 1)
	}
}

// updateRouteLatency updates latency metrics for a specific route.
func (c *ScaleMetricsCollector) updateRouteLatency(routeID string, latencyUs float64) {
	c.routeMetricsMu.Lock()
	defer c.routeMetricsMu.Unlock()

	rm, ok := c.routeMetrics[routeID]
	if !ok {
		rm = &RouteMetrics{RouteID: routeID}
		c.routeMetrics[routeID] = rm
	}

	// Exponential moving average for latency
	if rm.AvgLatencyUs == 0 {
		rm.AvgLatencyUs = latencyUs
	} else {
		alpha := 0.1
		rm.AvgLatencyUs = alpha*latencyUs + (1-alpha)*rm.AvgLatencyUs
	}
	rm.LastUpdate = time.Now()
}

// UpdateRouteStatus updates bus counts for a route.
func (c *ScaleMetricsCollector) UpdateRouteStatus(routeID string, active, offline, total int64) {
	c.routeMetricsMu.Lock()
	defer c.routeMetricsMu.Unlock()

	rm, ok := c.routeMetrics[routeID]
	if !ok {
		rm = &RouteMetrics{RouteID: routeID}
		c.routeMetrics[routeID] = rm
	}

	rm.ActiveBuses = active
	rm.OfflineBuses = offline
	rm.TotalBuses = total
	rm.LastUpdate = time.Now()
}

// CollectSnapshot gathers a comprehensive metrics snapshot.
func (c *ScaleMetricsCollector) CollectSnapshot(ctx context.Context) (*ScaleMetricsSnapshot, error) {
	snapshot := &ScaleMetricsSnapshot{
		Timestamp: time.Now(),
		Uptime:    time.Since(c.startTime),
	}

	// Collect latency stats
	snapshot.LatencyStats = make(map[OperationType]LatencyStats)
	c.histogramMu.RLock()
	for op, h := range c.latencyHistograms {
		count, sum, min, max, p50, p95, p99 := h.Stats()
		snapshot.LatencyStats[op] = LatencyStats{
			Count: count,
			Sum:   sum,
			Min:   min,
			Max:   max,
			P50:   p50,
			P95:   p95,
			P99:   p99,
		}
	}
	c.histogramMu.RUnlock()

	// Collect throughput stats
	snapshot.ThroughputStats = make(map[string]float64)
	c.throughputMu.RLock()
	for name, tracker := range c.throughputTrackers {
		snapshot.ThroughputStats[name] = tracker.OpsPerSecond()
	}
	c.throughputMu.RUnlock()

	// Collect error counts
	snapshot.ErrorCounts = make(map[ErrorCategory]int64)
	c.errorCountsMu.RLock()
	for cat, counter := range c.errorCounts {
		snapshot.ErrorCounts[cat] = atomic.LoadInt64(counter)
	}
	c.errorCountsMu.RUnlock()

	// Collect route metrics
	snapshot.RouteMetrics = make([]RouteMetrics, 0)
	c.routeMetricsMu.RLock()
	for _, rm := range c.routeMetrics {
		snapshot.RouteMetrics = append(snapshot.RouteMetrics, *rm)
	}
	c.routeMetricsMu.RUnlock()

	// Collect Redis metrics
	if c.client != nil {
		metricsCollector := NewMetricsCollector(c.client)
		if redisMetrics, err := metricsCollector.Collect(ctx); err == nil {
			snapshot.RedisMetrics = redisMetrics
		}
	}

	// Add to time series
	c.addTimeSeriesPoint(snapshot)

	return snapshot, nil
}

// addTimeSeriesPoint adds a point to the time series data.
func (c *ScaleMetricsCollector) addTimeSeriesPoint(snapshot *ScaleMetricsSnapshot) {
	c.timeSeriesMu.Lock()
	defer c.timeSeriesMu.Unlock()

	point := TimeSeriesPoint{
		Timestamp: snapshot.Timestamp,
	}

	if stats, ok := snapshot.LatencyStats[OpTypeWrite]; ok {
		point.WriteOps = stats.Count
		point.WriteLatencyP99 = stats.P99
	}
	if stats, ok := snapshot.LatencyStats[OpTypeRead]; ok {
		point.ReadOps = stats.Count
		point.ReadLatencyP99 = stats.P99
	}
	if snapshot.RedisMetrics != nil {
		point.ActiveBuses = snapshot.RedisMetrics.ActiveBusCount
		point.OfflineBuses = snapshot.RedisMetrics.OfflineBusCount
		point.MemoryUsedMB = float64(snapshot.RedisMetrics.UsedMemoryBytes) / 1024 / 1024
	}

	// Calculate total ops/sec
	for _, ops := range snapshot.ThroughputStats {
		point.OpsPerSecond += ops
	}

	c.timeSeriesData = append(c.timeSeriesData, point)

	// Trim old data
	cutoff := time.Now().Add(-c.timeSeriesMaxAge)
	for len(c.timeSeriesData) > 0 && c.timeSeriesData[0].Timestamp.Before(cutoff) {
		c.timeSeriesData = c.timeSeriesData[1:]
	}
}

// GetTimeSeriesData returns recent time series data.
func (c *ScaleMetricsCollector) GetTimeSeriesData() []TimeSeriesPoint {
	c.timeSeriesMu.RLock()
	defer c.timeSeriesMu.RUnlock()

	result := make([]TimeSeriesPoint, len(c.timeSeriesData))
	copy(result, c.timeSeriesData)
	return result
}

// ─────────────────────────────────────────────────────────────────────────────
// Metrics Snapshot
// ─────────────────────────────────────────────────────────────────────────────

// LatencyStats holds latency statistics for an operation type.
type LatencyStats struct {
	Count int64
	Sum   float64
	Min   float64
	Max   float64
	P50   float64
	P95   float64
	P99   float64
}

// ScaleMetricsSnapshot represents a complete metrics snapshot.
type ScaleMetricsSnapshot struct {
	Timestamp       time.Time
	Uptime          time.Duration
	LatencyStats    map[OperationType]LatencyStats
	ThroughputStats map[string]float64
	ErrorCounts     map[ErrorCategory]int64
	RouteMetrics    []RouteMetrics
	RedisMetrics    *RedisMetrics
}

// ─────────────────────────────────────────────────────────────────────────────
// Prometheus Export Format
// ─────────────────────────────────────────────────────────────────────────────

// ExportPrometheusMetrics exports metrics in Prometheus text format.
func (c *ScaleMetricsCollector) ExportPrometheusMetrics(ctx context.Context) string {
	snapshot, err := c.CollectSnapshot(ctx)
	if err != nil {
		return fmt.Sprintf("# Error collecting metrics: %v\n", err)
	}

	var output string

	// Latency histograms
	for op, stats := range snapshot.LatencyStats {
		output += fmt.Sprintf("# HELP alebus_redis_%s_latency_microseconds Latency for %s operations\n", op, op)
		output += fmt.Sprintf("# TYPE alebus_redis_%s_latency_microseconds summary\n", op)
		output += fmt.Sprintf("alebus_redis_%s_latency_microseconds{quantile=\"0.5\"} %.2f\n", op, stats.P50)
		output += fmt.Sprintf("alebus_redis_%s_latency_microseconds{quantile=\"0.95\"} %.2f\n", op, stats.P95)
		output += fmt.Sprintf("alebus_redis_%s_latency_microseconds{quantile=\"0.99\"} %.2f\n", op, stats.P99)
		output += fmt.Sprintf("alebus_redis_%s_latency_microseconds_sum %.2f\n", op, stats.Sum)
		output += fmt.Sprintf("alebus_redis_%s_latency_microseconds_count %d\n", op, stats.Count)
	}

	// Throughput gauges
	output += "# HELP alebus_ops_per_second Current operations per second\n"
	output += "# TYPE alebus_ops_per_second gauge\n"
	for handler, ops := range snapshot.ThroughputStats {
		output += fmt.Sprintf("alebus_ops_per_second{handler=\"%s\"} %.2f\n", handler, ops)
	}

	// Error counters
	output += "# HELP alebus_errors_total Total errors by category\n"
	output += "# TYPE alebus_errors_total counter\n"
	for cat, count := range snapshot.ErrorCounts {
		output += fmt.Sprintf("alebus_errors_total{category=\"%s\"} %d\n", cat, count)
	}

	// Bus metrics
	if snapshot.RedisMetrics != nil {
		output += "# HELP alebus_buses_active Number of active buses\n"
		output += "# TYPE alebus_buses_active gauge\n"
		output += fmt.Sprintf("alebus_buses_active %d\n", snapshot.RedisMetrics.ActiveBusCount)

		output += "# HELP alebus_buses_offline Number of offline buses\n"
		output += "# TYPE alebus_buses_offline gauge\n"
		output += fmt.Sprintf("alebus_buses_offline %d\n", snapshot.RedisMetrics.OfflineBusCount)

		output += "# HELP alebus_buses_total Total buses tracked\n"
		output += "# TYPE alebus_buses_total gauge\n"
		output += fmt.Sprintf("alebus_buses_total %d\n", snapshot.RedisMetrics.TotalBusCount)

		output += "# HELP alebus_redis_memory_bytes Redis memory usage\n"
		output += "# TYPE alebus_redis_memory_bytes gauge\n"
		output += fmt.Sprintf("alebus_redis_memory_bytes %d\n", snapshot.RedisMetrics.UsedMemoryBytes)

		output += "# HELP alebus_redis_replicas Connected Redis replicas\n"
		output += "# TYPE alebus_redis_replicas gauge\n"
		output += fmt.Sprintf("alebus_redis_replicas %d\n", snapshot.RedisMetrics.ConnectedReplicas)
	}

	// Route-level metrics
	output += "# HELP alebus_route_active_buses Active buses per route\n"
	output += "# TYPE alebus_route_active_buses gauge\n"
	for _, rm := range snapshot.RouteMetrics {
		output += fmt.Sprintf("alebus_route_active_buses{route=\"%s\"} %d\n", rm.RouteID, rm.ActiveBuses)
	}

	return output
}
