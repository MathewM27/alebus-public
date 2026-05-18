package redis

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// ─────────────────────────────────────────────────────────────────────────────
// Metrics Types
// ─────────────────────────────────────────────────────────────────────────────

// RedisMetrics holds all observable metrics for Redis operations.
type RedisMetrics struct {
	// Write metrics
	WriteCount      int64 // Total write operations
	WriteErrors     int64 // Total write errors
	WriteLatencyP50 int64 // P50 latency in microseconds
	WriteLatencyP95 int64 // P95 latency in microseconds
	WriteLatencyP99 int64 // P99 latency in microseconds

	// Read metrics
	ReadCount      int64 // Total read operations
	ReadErrors     int64 // Total read errors
	ReadLatencyP50 int64 // P50 latency in microseconds
	ReadLatencyP95 int64 // P95 latency in microseconds
	ReadLatencyP99 int64 // P99 latency in microseconds

	// Memory metrics
	UsedMemoryBytes int64 // Total memory used by Redis
	UsedMemoryPeak  int64 // Peak memory usage
	MaxMemoryBytes  int64 // Configured maxmemory
	MemoryFragRatio float64
	BusStateCount   int64 // Number of bus state keys
	RouteIndexCount int64 // Number of route index keys
	GeoIndexSize    int64 // Size of geo index

	// Bus status metrics
	ActiveBusCount  int64 // Buses with recent updates
	OfflineBusCount int64 // Buses past OfflineThreshold
	TotalBusCount   int64 // Total buses in Redis

	// Replication metrics
	ConnectedReplicas int64
	ReplicationLag    int64 // In bytes

	// AOF metrics
	AOFEnabled       bool
	AOFRewriteInProg bool
	AOFLastRewriteMs int64

	// Timestamp
	CollectedAt time.Time
}

// ─────────────────────────────────────────────────────────────────────────────
// Latency Tracker
// ─────────────────────────────────────────────────────────────────────────────

// LatencyTracker tracks operation latencies using a circular buffer.
type LatencyTracker struct {
	mu       sync.Mutex
	samples  []int64 // Latency samples in microseconds
	position int
	count    int64

	// Counters (atomic)
	totalOps  int64
	totalErrs int64
	lastError atomic.Value // stores error string
	lastErrAt atomic.Value // stores time.Time
}

// NewLatencyTracker creates a new latency tracker with the given sample size.
func NewLatencyTracker(sampleSize int) *LatencyTracker {
	return &LatencyTracker{
		samples: make([]int64, sampleSize),
	}
}

// Record records an operation's latency.
func (t *LatencyTracker) Record(latency time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	us := latency.Microseconds()
	t.samples[t.position] = us
	t.position = (t.position + 1) % len(t.samples)
	if t.count < int64(len(t.samples)) {
		t.count++
	}
	atomic.AddInt64(&t.totalOps, 1)
}

// RecordError records an error occurrence.
func (t *LatencyTracker) RecordError(err error) {
	atomic.AddInt64(&t.totalErrs, 1)
	if err != nil {
		t.lastError.Store(err.Error())
		t.lastErrAt.Store(time.Now())
	}
}

// Percentile returns the latency at the given percentile (0-100).
func (t *LatencyTracker) Percentile(p float64) int64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.count == 0 {
		return 0
	}

	// Copy and sort samples
	n := int(t.count)
	if n > len(t.samples) {
		n = len(t.samples)
	}
	sorted := make([]int64, n)
	copy(sorted, t.samples[:n])
	sortInt64(sorted)

	// Calculate percentile index
	idx := int(float64(n-1) * p / 100.0)
	return sorted[idx]
}

// Stats returns current statistics.
func (t *LatencyTracker) Stats() (totalOps, totalErrs, p50, p95, p99 int64) {
	totalOps = atomic.LoadInt64(&t.totalOps)
	totalErrs = atomic.LoadInt64(&t.totalErrs)
	p50 = t.Percentile(50)
	p95 = t.Percentile(95)
	p99 = t.Percentile(99)
	return
}

// Simple insertion sort for int64 slices (efficient for small arrays)
func sortInt64(a []int64) {
	for i := 1; i < len(a); i++ {
		j := i
		for j > 0 && a[j-1] > a[j] {
			a[j-1], a[j] = a[j], a[j-1]
			j--
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Metrics Collector
// ─────────────────────────────────────────────────────────────────────────────

// MetricsCollector collects Redis metrics for observability.
type MetricsCollector struct {
	client *redis.Client

	// Latency trackers
	writeTracker *LatencyTracker
	readTracker  *LatencyTracker
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector(c *Client) *MetricsCollector {
	return &MetricsCollector{
		client:       c.Underlying(),
		writeTracker: NewLatencyTracker(1000), // Last 1000 samples
		readTracker:  NewLatencyTracker(1000),
	}
}

// WriteTracker returns the write latency tracker for recording write operations.
func (m *MetricsCollector) WriteTracker() *LatencyTracker {
	return m.writeTracker
}

// ReadTracker returns the read latency tracker for recording read operations.
func (m *MetricsCollector) ReadTracker() *LatencyTracker {
	return m.readTracker
}

// Collect gathers all Redis metrics.
func (m *MetricsCollector) Collect(ctx context.Context) (*RedisMetrics, error) {
	metrics := &RedisMetrics{
		CollectedAt: time.Now(),
	}

	// Get latency stats
	metrics.WriteCount, metrics.WriteErrors,
		metrics.WriteLatencyP50, metrics.WriteLatencyP95, metrics.WriteLatencyP99 =
		m.writeTracker.Stats()

	metrics.ReadCount, metrics.ReadErrors,
		metrics.ReadLatencyP50, metrics.ReadLatencyP95, metrics.ReadLatencyP99 =
		m.readTracker.Stats()

	// Get Redis INFO
	info, err := m.client.Info(ctx).Result()
	if err != nil {
		return nil, err
	}
	m.parseInfo(info, metrics)

	// Count bus keys
	busCount, activeCount, offlineCount, err := m.countBusStates(ctx)
	if err != nil {
		return nil, err
	}
	metrics.TotalBusCount = busCount
	metrics.ActiveBusCount = activeCount
	metrics.OfflineBusCount = offlineCount
	metrics.BusStateCount = busCount

	// Count route indexes
	routeCount, err := m.countKeys(ctx, RouteIndexKeyPrefix+"*")
	if err != nil {
		return nil, err
	}
	metrics.RouteIndexCount = routeCount

	// Get geo index size
	geoSize, err := m.client.ZCard(ctx, BusGeoKey).Result()
	if err != nil && err != redis.Nil {
		return nil, err
	}
	metrics.GeoIndexSize = geoSize

	return metrics, nil
}

// parseInfo parses Redis INFO output into metrics.
func (m *MetricsCollector) parseInfo(info string, metrics *RedisMetrics) {
	lines := strings.Split(info, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key, value := parts[0], parts[1]

		switch key {
		// Memory
		case "used_memory":
			metrics.UsedMemoryBytes, _ = strconv.ParseInt(value, 10, 64)
		case "used_memory_peak":
			metrics.UsedMemoryPeak, _ = strconv.ParseInt(value, 10, 64)
		case "maxmemory":
			metrics.MaxMemoryBytes, _ = strconv.ParseInt(value, 10, 64)
		case "mem_fragmentation_ratio":
			metrics.MemoryFragRatio, _ = strconv.ParseFloat(value, 64)

		// Replication
		case "connected_slaves":
			metrics.ConnectedReplicas, _ = strconv.ParseInt(value, 10, 64)
		case "master_repl_offset":
			// Replication lag requires comparing master and slave offsets
			// For simplicity, we just track the offset
			metrics.ReplicationLag, _ = strconv.ParseInt(value, 10, 64)

		// AOF
		case "aof_enabled":
			metrics.AOFEnabled = value == "1"
		case "aof_rewrite_in_progress":
			metrics.AOFRewriteInProg = value == "1"
		case "aof_last_rewrite_time_sec":
			ms, _ := strconv.ParseInt(value, 10, 64)
			metrics.AOFLastRewriteMs = ms * 1000
		}
	}
}

// countBusStates counts total, active, and offline bus states.
func (m *MetricsCollector) countBusStates(ctx context.Context) (total, active, offline int64, err error) {
	pattern := BusStateKeyPrefix + "*" + BusStateKeySuffix
	threshold := time.Now().Add(-OfflineThreshold)

	var cursor uint64
	for {
		keys, nextCursor, scanErr := m.client.Scan(ctx, cursor, pattern, 100).Result()
		if scanErr != nil {
			return 0, 0, 0, scanErr
		}

		for _, key := range keys {
			total++

			// Check updated_at timestamp
			updatedAt, getErr := m.client.HGet(ctx, key, fieldUpdatedAt).Result()
			if getErr != nil && getErr != redis.Nil {
				continue
			}

			if ts, parseErr := strconv.ParseInt(updatedAt, 10, 64); parseErr == nil {
				updateTime := time.UnixMilli(ts)
				if updateTime.After(threshold) {
					active++
				} else {
					offline++
				}
			} else {
				offline++ // Treat unparseable as offline
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return total, active, offline, nil
}

// countKeys counts keys matching a pattern.
func (m *MetricsCollector) countKeys(ctx context.Context, pattern string) (int64, error) {
	var count int64
	var cursor uint64

	for {
		keys, nextCursor, err := m.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return 0, err
		}
		count += int64(len(keys))
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return count, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Health Checker
// ─────────────────────────────────────────────────────────────────────────────

// HealthStatus represents the health state of Redis.
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// HealthCheck represents a Redis health check result.
type HealthCheck struct {
	Status         HealthStatus
	Latency        time.Duration
	MemoryPercent  float64
	ConnectedNodes int
	Message        string
	CheckedAt      time.Time
}

// HealthChecker performs Redis health checks.
type HealthChecker struct {
	client           *redis.Client
	maxLatency       time.Duration
	maxMemoryPercent float64
}

// NewHealthChecker creates a new health checker.
func NewHealthChecker(c *Client) *HealthChecker {
	return &HealthChecker{
		client:           c.Underlying(),
		maxLatency:       100 * time.Millisecond,
		maxMemoryPercent: 85.0,
	}
}

// Check performs a health check.
func (h *HealthChecker) Check(ctx context.Context) *HealthCheck {
	check := &HealthCheck{
		CheckedAt: time.Now(),
		Status:    HealthStatusHealthy,
	}

	// Ping test
	start := time.Now()
	err := h.client.Ping(ctx).Err()
	check.Latency = time.Since(start)

	if err != nil {
		check.Status = HealthStatusUnhealthy
		check.Message = "ping failed: " + err.Error()
		return check
	}

	// Check latency
	if check.Latency > h.maxLatency {
		check.Status = HealthStatusDegraded
		check.Message = "high latency"
	}

	// Get memory info
	info, err := h.client.Info(ctx, "memory").Result()
	if err == nil {
		var usedMem, maxMem int64
		for _, line := range strings.Split(info, "\n") {
			parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
			if len(parts) == 2 {
				switch parts[0] {
				case "used_memory":
					usedMem, _ = strconv.ParseInt(parts[1], 10, 64)
				case "maxmemory":
					maxMem, _ = strconv.ParseInt(parts[1], 10, 64)
				}
			}
		}
		if maxMem > 0 {
			check.MemoryPercent = float64(usedMem) / float64(maxMem) * 100
			if check.MemoryPercent > h.maxMemoryPercent {
				check.Status = HealthStatusDegraded
				check.Message = "high memory usage"
			}
		}
	}

	// Get replica count
	replicaInfo, err := h.client.Info(ctx, "replication").Result()
	if err == nil {
		for _, line := range strings.Split(replicaInfo, "\n") {
			parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
			if len(parts) == 2 && parts[0] == "connected_slaves" {
				check.ConnectedNodes, _ = strconv.Atoi(parts[1])
			}
		}
	}

	return check
}
