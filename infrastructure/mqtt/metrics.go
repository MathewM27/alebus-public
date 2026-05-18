package mqtt

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ─────────────────────────────────────────────────────────────────────────────
// Prometheus Metrics — EMQX Ingestor Observability
// ─────────────────────────────────────────────────────────────────────────────
//
// Per Phase 3 blueprint, these metrics are mandatory for production:
//   - Counters: track message processing outcomes
//   - Gauges: track current state (queue depth, inflight workers)
//   - Histograms: track latency distributions
//
// ─────────────────────────────────────────────────────────────────────────────

const (
	namespace = "alebus"
	subsystem = "ingestor"
)

// Metrics holds all Prometheus metrics for the EMQX ingestor.
type Metrics struct {
	// Counters — message processing outcomes
	ReceivedTotal   prometheus.Counter
	AcceptedTotal   prometheus.Counter
	StaleTotal      prometheus.Counter
	InvalidTotal    prometheus.Counter
	InfraErrorTotal prometheus.Counter
	DroppedTotal    prometheus.Counter
	CoalescedTotal  prometheus.Counter
	RetriedTotal    prometheus.Counter

	// Gauges — current state
	QueueDepth      prometheus.GaugeFunc
	WorkersInflight prometheus.GaugeFunc
	PendingAcks     prometheus.GaugeFunc
	ConnectionState prometheus.Gauge

	// Histograms — latency distributions
	RedisLatency   prometheus.Histogram
	ProcessingTime prometheus.Histogram
	AckLatency     prometheus.Histogram

	// ─────────────────────────────────────────────────────────────────────────
	// GPS Enrichment Metrics (Phase 6 - Option B+)
	// ─────────────────────────────────────────────────────────────────────────

	// GPSEnrichmentTotal counts GPS enrichment outcomes by status.
	// Labels: status={success,no_assignment,route_not_found,invalid,infra_error,disabled}
	GPSEnrichmentTotal *prometheus.CounterVec

	// GPSEnrichmentLatency tracks GPS enrichment processing latency in ms.
	GPSEnrichmentLatency prometheus.Histogram

	// GPSEventAge tracks how old the GPS event was when processed (now - timestamp_ms) in ms.
	// This approximates end-to-end latency from device timestamp to ingestor processing.
	// NOTE: Requires reasonably accurate device clock.
	GPSEventAge prometheus.Histogram

	// GPSDeviceToServerAge tracks received_at_ms - device_ts_ms.
	// Negative values indicate the device timestamp is in the future.
	GPSDeviceToServerAge prometheus.Histogram

	// GPSDeviceFutureSkew tracks device_ts_ms - received_at_ms.
	// Positive values indicate device timestamp is in the future.
	GPSDeviceFutureSkew prometheus.Histogram

	// GPSOutOfOrderTotal counts best-effort out-of-order device timestamps per bus.
	GPSOutOfOrderTotal prometheus.Counter

	// GPSDuplicateDeviceTsTotal counts best-effort duplicate device timestamps per bus.
	GPSDuplicateDeviceTsTotal prometheus.Counter

	// GPSResolverConfidence tracks resolver confidence distribution (0-1).
	GPSResolverConfidence prometheus.Histogram

	// GPSResolverMode tracks active resolver modes by bus.
	// Labels: mode={BOOTSTRAP,TRACKING,REACQUIRE,TERMINAL_DWELL}
	GPSResolverMode *prometheus.GaugeVec

	// GPS Cache metrics
	GPSAssignmentCacheHit  prometheus.Counter
	GPSAssignmentCacheMiss prometheus.Counter
	GPSGeometryCacheHit    prometheus.Counter
	GPSGeometryCacheMiss   prometheus.Counter

	// GPSRetryExhausted counts GPS messages that exhausted retry budget.
	GPSRetryExhausted prometheus.Counter

	// Internal references for gauge functions
	queueRef  *BoundedQueue
	poolRef   *WorkerPool
	clientRef *MQTTClient

	// Registration tracking
	registered bool
	mu         sync.Mutex
}

// MetricsConfig configures the metrics collector.
type MetricsConfig struct {
	// Registry is the Prometheus registry to use.
	// If nil, prometheus.DefaultRegisterer is used.
	Registry prometheus.Registerer

	// LatencyBuckets defines the histogram buckets for latency metrics.
	// Default: [1, 5, 10, 25, 50, 100, 250, 500, 1000] ms
	LatencyBuckets []float64
}

// DefaultMetricsConfig returns sensible defaults.
func DefaultMetricsConfig() MetricsConfig {
	return MetricsConfig{
		Registry:       prometheus.DefaultRegisterer,
		LatencyBuckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000},
	}
}

// NewMetrics creates a new metrics collector.
//
// The queue, pool, and client parameters are optional and enable gauge metrics.
// Call Register() to register with Prometheus.
func NewMetrics(cfg MetricsConfig, queue *BoundedQueue, pool *WorkerPool, client *MQTTClient) *Metrics {
	if cfg.LatencyBuckets == nil {
		cfg.LatencyBuckets = DefaultMetricsConfig().LatencyBuckets
	}

	m := &Metrics{
		queueRef:  queue,
		poolRef:   pool,
		clientRef: client,
	}

	// Counters
	m.ReceivedTotal = promauto.With(cfg.Registry).NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "received_total",
		Help:      "Total number of messages received from MQTT",
	})

	m.AcceptedTotal = promauto.With(cfg.Registry).NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "accepted_total",
		Help:      "Total number of messages successfully written to Redis",
	})

	m.StaleTotal = promauto.With(cfg.Registry).NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "stale_total",
		Help:      "Total number of stale messages rejected",
	})

	m.InvalidTotal = promauto.With(cfg.Registry).NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "invalid_total",
		Help:      "Total number of invalid messages (JSON parse or validation failure)",
	})

	m.InfraErrorTotal = promauto.With(cfg.Registry).NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "infra_error_total",
		Help:      "Total number of infrastructure errors (dependency failures triggering retry/ACK)",
	})

	m.DroppedTotal = promauto.With(cfg.Registry).NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "dropped_total",
		Help:      "Total number of messages dropped due to queue overflow",
	})

	m.CoalescedTotal = promauto.With(cfg.Registry).NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "coalesced_total",
		Help:      "Total number of messages coalesced (overwritten by newer)",
	})

	m.RetriedTotal = promauto.With(cfg.Registry).NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "retried_total",
		Help:      "Total number of message processing retries",
	})

	// Gauges with functions (dynamic values)
	if queue != nil {
		m.QueueDepth = promauto.With(cfg.Registry).NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "queue_depth",
			Help:      "Current number of messages in the processing queue",
		}, func() float64 {
			return float64(queue.Len())
		})
	}

	if pool != nil {
		m.WorkersInflight = promauto.With(cfg.Registry).NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "workers_inflight",
			Help:      "Current number of workers actively processing messages",
		}, func() float64 {
			return float64(pool.Stats().Inflight)
		})
	}

	if client != nil {
		m.PendingAcks = promauto.With(cfg.Registry).NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "pending_acks",
			Help:      "Current number of pending MQTT acknowledgements",
		}, func() float64 {
			return float64(client.Stats().PendingAcks)
		})
	}

	m.ConnectionState = promauto.With(cfg.Registry).NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "connection_state",
		Help:      "MQTT connection state (0=disconnected, 1=connecting, 2=connected)",
	})

	// Histograms
	m.RedisLatency = promauto.With(cfg.Registry).NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "redis_latency_ms",
		Help:      "Redis Lua EVAL latency in milliseconds",
		Buckets:   cfg.LatencyBuckets,
	})

	m.ProcessingTime = promauto.With(cfg.Registry).NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "processing_time_ms",
		Help:      "Total message processing time in milliseconds (decode + validate + publish)",
		Buckets:   cfg.LatencyBuckets,
	})

	m.AckLatency = promauto.With(cfg.Registry).NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "ack_latency_ms",
		Help:      "Time from message receipt to ACK in milliseconds",
		Buckets:   cfg.LatencyBuckets,
	})

	// ─────────────────────────────────────────────────────────────────────────
	// GPS Enrichment Metrics (Phase 6 - Option B+)
	// ─────────────────────────────────────────────────────────────────────────

	m.GPSEnrichmentTotal = promauto.With(cfg.Registry).NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "gps_enrichment_total",
		Help:      "Total GPS enrichment attempts by status",
	}, []string{"status"})

	m.GPSEnrichmentLatency = promauto.With(cfg.Registry).NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "gps_enrichment_latency_ms",
		Help:      "GPS enrichment processing latency in milliseconds",
		Buckets:   cfg.LatencyBuckets,
	})

	// Separate buckets: GPS event age can reasonably be a few seconds.
	// Keep this distinct from per-operation latency buckets.
	ageBuckets := []float64{10, 25, 50, 100, 250, 500, 1000, 2000, 5000, 10000, 30000, 60000}
	m.GPSEventAge = promauto.With(cfg.Registry).NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "gps_event_age_ms",
		Help:      "Age of GPS event when processed (now - payload timestamp_ms) in milliseconds",
		Buckets:   ageBuckets,
	})

	// Symmetric buckets to allow negative/positive skew visibility.
	skewBuckets := []float64{-600000, -300000, -60000, -30000, -10000, -5000, -1000, -250, -100, -25, -10, 0, 10, 25, 100, 250, 1000, 5000, 10000, 30000, 60000, 300000, 600000}
	m.GPSDeviceToServerAge = promauto.With(cfg.Registry).NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "gps_device_to_server_age_ms",
		Help:      "Received-at minus device timestamp (received_at_ms - device_ts_ms) in milliseconds",
		Buckets:   skewBuckets,
	})

	m.GPSDeviceFutureSkew = promauto.With(cfg.Registry).NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "gps_device_future_skew_ms",
		Help:      "Device timestamp minus received-at (device_ts_ms - received_at_ms) in milliseconds",
		Buckets:   skewBuckets,
	})

	m.GPSOutOfOrderTotal = promauto.With(cfg.Registry).NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "gps_out_of_order_total",
		Help:      "Best-effort count of out-of-order device timestamps per bus",
	})

	m.GPSDuplicateDeviceTsTotal = promauto.With(cfg.Registry).NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "gps_duplicate_device_ts_total",
		Help:      "Best-effort count of duplicate device timestamps per bus",
	})

	m.GPSResolverConfidence = promauto.With(cfg.Registry).NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "gps_resolver_confidence",
		Help:      "GPS resolver confidence distribution (0-1)",
		Buckets:   []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
	})

	m.GPSResolverMode = promauto.With(cfg.Registry).NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "gps_resolver_mode",
		Help:      "Number of buses in each resolver mode",
	}, []string{"mode"})

	m.GPSAssignmentCacheHit = promauto.With(cfg.Registry).NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "gps_assignment_cache_hit_total",
		Help:      "Total assignment cache hits",
	})

	m.GPSAssignmentCacheMiss = promauto.With(cfg.Registry).NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "gps_assignment_cache_miss_total",
		Help:      "Total assignment cache misses",
	})

	m.GPSGeometryCacheHit = promauto.With(cfg.Registry).NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "gps_geometry_cache_hit_total",
		Help:      "Total geometry cache hits",
	})

	m.GPSGeometryCacheMiss = promauto.With(cfg.Registry).NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "gps_geometry_cache_miss_total",
		Help:      "Total geometry cache misses",
	})

	m.GPSRetryExhausted = promauto.With(cfg.Registry).NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "gps_retry_exhausted_total",
		Help:      "Total GPS messages that exhausted retry budget",
	})

	m.registered = true
	return m
}

// RecordOutcome records the outcome of processing a message.
// This is the main entry point for recording metrics after processing.
func (m *Metrics) RecordOutcome(outcome MessageOutcome, latencyMs float64) {
	switch outcome {
	case OutcomeAccepted:
		m.AcceptedTotal.Inc()
	case OutcomeGPSSuccess:
		// GPS success also results in a Redis write (via live bus ingestion).
		m.AcceptedTotal.Inc()
	case OutcomeStale:
		m.StaleTotal.Inc()
	case OutcomeInvalid:
		m.InvalidTotal.Inc()
	case OutcomeInfraError:
		m.InfraErrorTotal.Inc()
	case OutcomeDropped:
		m.DroppedTotal.Inc()
	case OutcomeCoalesced:
		m.CoalescedTotal.Inc()
	}

	if latencyMs > 0 {
		m.ProcessingTime.Observe(latencyMs)
	}
}

// RecordReceived increments the received counter.
func (m *Metrics) RecordReceived() {
	m.ReceivedTotal.Inc()
}

// RecordRedisLatency records a Redis operation latency.
func (m *Metrics) RecordRedisLatency(latencyMs float64) {
	m.RedisLatency.Observe(latencyMs)
}

// RecordAckLatency records the time from receipt to ACK.
func (m *Metrics) RecordAckLatency(latencyMs float64) {
	m.AckLatency.Observe(latencyMs)
}

// RecordRetry increments the retry counter.
func (m *Metrics) RecordRetry() {
	m.RetriedTotal.Inc()
}

// SetConnectionState updates the connection state gauge.
func (m *Metrics) SetConnectionState(state ConnectionState) {
	m.ConnectionState.Set(float64(state))
}

// SetClient sets or updates the MQTT client reference.
// This is used for dynamic gauge metrics that read from the client.
func (m *Metrics) SetClient(client *MQTTClient) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clientRef = client
}

// UpdateQueueStats updates queue-related metrics from queue stats.
func (m *Metrics) UpdateQueueStats(stats QueueStats) {
	// Dropped and coalesced are updated via counters, but we can sync
	// with queue stats if needed for accuracy
}

// ─────────────────────────────────────────────────────────────────────────────
// GPS Enrichment Metrics Recording (Phase 6 - Option B+)
// ─────────────────────────────────────────────────────────────────────────────

// GPSEnrichmentStatus represents the GPS enrichment outcome for metrics labeling.
type GPSEnrichmentStatus string

const (
	GPSStatusSuccess               GPSEnrichmentStatus = "success"
	GPSStatusNoAssignment          GPSEnrichmentStatus = "no_assignment"
	GPSStatusRouteNotFound         GPSEnrichmentStatus = "route_not_found"
	GPSStatusInvalid               GPSEnrichmentStatus = "invalid"
	GPSStatusInfraError            GPSEnrichmentStatus = "infra_error"
	GPSStatusDisabled              GPSEnrichmentStatus = "disabled"
	GPSStatusUnknownTopic          GPSEnrichmentStatus = "unknown_topic"
	GPSStatusStagedRolloutFiltered GPSEnrichmentStatus = "staged_rollout_filtered"
	GPSStatusReplayIgnored         GPSEnrichmentStatus = "replay_ignored"
	GPSStatusFutureHardRejected    GPSEnrichmentStatus = "future_hard_rejected"
)

// RecordGPSOutcome records a GPS enrichment outcome with latency.
// This is the main entry point for GPS-specific metrics.
func (m *Metrics) RecordGPSOutcome(status GPSEnrichmentStatus, latencyMs float64) {
	m.GPSEnrichmentTotal.WithLabelValues(string(status)).Inc()
	if latencyMs > 0 {
		m.GPSEnrichmentLatency.Observe(latencyMs)
	}
}

// RecordGPSResolverConfidence records a resolver confidence value (0-1).
func (m *Metrics) RecordGPSResolverConfidence(confidence float64) {
	m.GPSResolverConfidence.Observe(confidence)
}

// SetGPSResolverModeCount sets the count of buses in a specific resolver mode.
func (m *Metrics) SetGPSResolverModeCount(mode string, count float64) {
	m.GPSResolverMode.WithLabelValues(mode).Set(count)
}

// IncGPSResolverMode increments the count for a resolver mode.
func (m *Metrics) IncGPSResolverMode(mode string) {
	m.GPSResolverMode.WithLabelValues(mode).Inc()
}

// DecGPSResolverMode decrements the count for a resolver mode.
func (m *Metrics) DecGPSResolverMode(mode string) {
	m.GPSResolverMode.WithLabelValues(mode).Dec()
}

// RecordGPSAssignmentCacheHit records an assignment cache hit.
func (m *Metrics) RecordGPSAssignmentCacheHit() {
	m.GPSAssignmentCacheHit.Inc()
}

// RecordGPSAssignmentCacheMiss records an assignment cache miss.
func (m *Metrics) RecordGPSAssignmentCacheMiss() {
	m.GPSAssignmentCacheMiss.Inc()
}

// RecordGPSGeometryCacheHit records a geometry cache hit.
func (m *Metrics) RecordGPSGeometryCacheHit() {
	m.GPSGeometryCacheHit.Inc()
}

// RecordGPSGeometryCacheMiss records a geometry cache miss.
func (m *Metrics) RecordGPSGeometryCacheMiss() {
	m.GPSGeometryCacheMiss.Inc()
}

// RecordGPSRetryExhausted records when a GPS message exhausts retry budget.
func (m *Metrics) RecordGPSRetryExhausted() {
	m.GPSRetryExhausted.Inc()
}

// MapOutcomeToGPSStatus maps a MessageOutcome to GPSEnrichmentStatus.
func MapOutcomeToGPSStatus(outcome MessageOutcome) GPSEnrichmentStatus {
	switch outcome {
	case OutcomeGPSSuccess:
		return GPSStatusSuccess
	case OutcomeGPSNoAssignment:
		return GPSStatusNoAssignment
	case OutcomeGPSRouteNotFound:
		return GPSStatusRouteNotFound
	case OutcomeGPSInvalid:
		return GPSStatusInvalid
	case OutcomeGPSReplayIgnored:
		return GPSStatusReplayIgnored
	case OutcomeGPSFutureHardRejected:
		return GPSStatusFutureHardRejected
	case OutcomeInfraError:
		return GPSStatusInfraError
	case OutcomeGPSEnrichmentDisabled:
		return GPSStatusDisabled
	case OutcomeGPSStagedRolloutFiltered:
		return GPSStatusStagedRolloutFiltered
	case OutcomeUnknownTopic:
		return GPSStatusUnknownTopic
	default:
		return GPSStatusInvalid
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Null Metrics (for testing without Prometheus)
// ─────────────────────────────────────────────────────────────────────────────

// NullMetrics is a no-op implementation for testing.
type NullMetrics struct{}

// RecordOutcome is a no-op.
func (NullMetrics) RecordOutcome(outcome MessageOutcome, latencyMs float64) {}

// RecordReceived is a no-op.
func (NullMetrics) RecordReceived() {}

// RecordRedisLatency is a no-op.
func (NullMetrics) RecordRedisLatency(latencyMs float64) {}

// RecordAckLatency is a no-op.
func (NullMetrics) RecordAckLatency(latencyMs float64) {}

// RecordRetry is a no-op.
func (NullMetrics) RecordRetry() {}

// SetConnectionState is a no-op.
func (NullMetrics) SetConnectionState(state ConnectionState) {}

// ─────────────────────────────────────────────────────────────────────────────
// GPS Enrichment NullMetrics (Phase 6 - Option B+)
// ─────────────────────────────────────────────────────────────────────────────

// RecordGPSOutcome is a no-op.
func (NullMetrics) RecordGPSOutcome(status GPSEnrichmentStatus, latencyMs float64) {}

// RecordGPSResolverConfidence is a no-op.
func (NullMetrics) RecordGPSResolverConfidence(confidence float64) {}

// SetGPSResolverModeCount is a no-op.
func (NullMetrics) SetGPSResolverModeCount(mode string, count float64) {}

// IncGPSResolverMode is a no-op.
func (NullMetrics) IncGPSResolverMode(mode string) {}

// DecGPSResolverMode is a no-op.
func (NullMetrics) DecGPSResolverMode(mode string) {}

// RecordGPSAssignmentCacheHit is a no-op.
func (NullMetrics) RecordGPSAssignmentCacheHit() {}

// RecordGPSAssignmentCacheMiss is a no-op.
func (NullMetrics) RecordGPSAssignmentCacheMiss() {}

// RecordGPSGeometryCacheHit is a no-op.
func (NullMetrics) RecordGPSGeometryCacheHit() {}

// RecordGPSGeometryCacheMiss is a no-op.
func (NullMetrics) RecordGPSGeometryCacheMiss() {}

// RecordGPSRetryExhausted is a no-op.
func (NullMetrics) RecordGPSRetryExhausted() {}
