package mqtt

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────────────────────────────────────
// Metrics Construction Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestNewMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	cfg := MetricsConfig{
		Registry:       reg,
		LatencyBuckets: []float64{1, 5, 10, 50, 100},
	}

	m := NewMetrics(cfg, nil, nil, nil)

	require.NotNil(t, m)
	assert.NotNil(t, m.ReceivedTotal)
	assert.NotNil(t, m.AcceptedTotal)
	assert.NotNil(t, m.StaleTotal)
	assert.NotNil(t, m.InvalidTotal)
	assert.NotNil(t, m.InfraErrorTotal)
	assert.NotNil(t, m.DroppedTotal)
	assert.NotNil(t, m.CoalescedTotal)
	assert.NotNil(t, m.RedisLatency)
	assert.NotNil(t, m.ProcessingTime)
}

func TestNewMetrics_WithComponents(t *testing.T) {
	reg := prometheus.NewRegistry()
	cfg := MetricsConfig{Registry: reg}

	queue := NewBoundedQueue(100, "drop_new")
	// Note: pool and client would require more setup, testing with nil

	m := NewMetrics(cfg, queue, nil, nil)

	require.NotNil(t, m)
	assert.NotNil(t, m.QueueDepth)
}

// ─────────────────────────────────────────────────────────────────────────────
// Counter Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestMetrics_Counters(t *testing.T) {
	reg := prometheus.NewRegistry()
	cfg := MetricsConfig{Registry: reg}
	m := NewMetrics(cfg, nil, nil, nil)

	// Record various outcomes
	m.RecordOutcome(OutcomeAccepted, 10)
	m.RecordOutcome(OutcomeAccepted, 15)
	m.RecordOutcome(OutcomeStale, 5)
	m.RecordOutcome(OutcomeInvalid, 3)
	m.RecordOutcome(OutcomeInfraError, 20)
	m.RecordOutcome(OutcomeDropped, 0)
	m.RecordOutcome(OutcomeCoalesced, 0)
	m.RecordReceived()
	m.RecordReceived()
	m.RecordReceived()
	m.RecordRetry()

	// Verify counters
	assert.Equal(t, float64(2), testutil.ToFloat64(m.AcceptedTotal))
	assert.Equal(t, float64(1), testutil.ToFloat64(m.StaleTotal))
	assert.Equal(t, float64(1), testutil.ToFloat64(m.InvalidTotal))
	assert.Equal(t, float64(1), testutil.ToFloat64(m.InfraErrorTotal))
	assert.Equal(t, float64(1), testutil.ToFloat64(m.DroppedTotal))
	assert.Equal(t, float64(1), testutil.ToFloat64(m.CoalescedTotal))
	assert.Equal(t, float64(3), testutil.ToFloat64(m.ReceivedTotal))
	assert.Equal(t, float64(1), testutil.ToFloat64(m.RetriedTotal))
}

func TestMetrics_RecordOutcome_AllTypes(t *testing.T) {
	tests := []struct {
		outcome MessageOutcome
		counter string
	}{
		{OutcomeAccepted, "accepted"},
		{OutcomeStale, "stale"},
		{OutcomeInvalid, "invalid"},
		{OutcomeInfraError, "infra_error"},
		{OutcomeDropped, "dropped"},
		{OutcomeCoalesced, "coalesced"},
	}

	for _, tt := range tests {
		t.Run(tt.counter, func(t *testing.T) {
			reg := prometheus.NewRegistry()
			cfg := MetricsConfig{Registry: reg}
			m := NewMetrics(cfg, nil, nil, nil)

			m.RecordOutcome(tt.outcome, 10)

			// Verify the correct counter was incremented
			// We check by gathering metrics
			families, _ := reg.Gather()
			found := false
			for _, f := range families {
				if f.GetName() == "alebus_ingestor_"+tt.counter+"_total" {
					found = true
					assert.Equal(t, float64(1), f.GetMetric()[0].GetCounter().GetValue())
				}
			}
			assert.True(t, found, "counter %s not found", tt.counter)
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Gauge Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestMetrics_Gauges_QueueDepth(t *testing.T) {
	reg := prometheus.NewRegistry()
	cfg := MetricsConfig{Registry: reg}

	queue := NewBoundedQueue(100, "drop_new")
	_ = NewMetrics(cfg, queue, nil, nil) // Registers GaugeFunc with queue.Len()

	// Initial depth should be 0
	families, _ := reg.Gather()
	var depth float64
	for _, f := range families {
		if f.GetName() == "alebus_ingestor_queue_depth" {
			depth = f.GetMetric()[0].GetGauge().GetValue()
		}
	}
	assert.Equal(t, float64(0), depth)

	// Add items
	queue.Enqueue(NewWorkItem(1, "bus-1", []byte(`{}`)))
	queue.Enqueue(NewWorkItem(2, "bus-2", []byte(`{}`)))

	// Depth should reflect actual queue size
	families, _ = reg.Gather()
	for _, f := range families {
		if f.GetName() == "alebus_ingestor_queue_depth" {
			depth = f.GetMetric()[0].GetGauge().GetValue()
		}
	}
	assert.Equal(t, float64(2), depth)
}

func TestMetrics_ConnectionState(t *testing.T) {
	reg := prometheus.NewRegistry()
	cfg := MetricsConfig{Registry: reg}
	m := NewMetrics(cfg, nil, nil, nil)

	// Set connected
	m.SetConnectionState(StateConnected)
	assert.Equal(t, float64(2), testutil.ToFloat64(m.ConnectionState))

	// Set disconnected
	m.SetConnectionState(StateDisconnected)
	assert.Equal(t, float64(0), testutil.ToFloat64(m.ConnectionState))

	// Set connecting
	m.SetConnectionState(StateConnecting)
	assert.Equal(t, float64(1), testutil.ToFloat64(m.ConnectionState))
}

// ─────────────────────────────────────────────────────────────────────────────
// Histogram Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestMetrics_Histogram_RedisLatency(t *testing.T) {
	reg := prometheus.NewRegistry()
	cfg := MetricsConfig{
		Registry:       reg,
		LatencyBuckets: []float64{1, 5, 10, 50, 100},
	}
	m := NewMetrics(cfg, nil, nil, nil)

	// Record some latencies
	m.RecordRedisLatency(2)   // bucket: 5
	m.RecordRedisLatency(8)   // bucket: 10
	m.RecordRedisLatency(15)  // bucket: 50
	m.RecordRedisLatency(75)  // bucket: 100
	m.RecordRedisLatency(150) // bucket: +Inf

	// Verify histogram has observations
	families, _ := reg.Gather()
	for _, f := range families {
		if f.GetName() == "alebus_ingestor_redis_latency_ms" {
			hist := f.GetMetric()[0].GetHistogram()
			assert.Equal(t, uint64(5), hist.GetSampleCount())
			// Sum should be 2+8+15+75+150 = 250
			assert.Equal(t, float64(250), hist.GetSampleSum())
		}
	}
}

func TestMetrics_Histogram_ProcessingTime(t *testing.T) {
	reg := prometheus.NewRegistry()
	cfg := MetricsConfig{Registry: reg}
	m := NewMetrics(cfg, nil, nil, nil)

	// RecordOutcome also records processing time
	m.RecordOutcome(OutcomeAccepted, 25.5)
	m.RecordOutcome(OutcomeStale, 12.3)

	// Verify observations
	families, _ := reg.Gather()
	for _, f := range families {
		if f.GetName() == "alebus_ingestor_processing_time_ms" {
			hist := f.GetMetric()[0].GetHistogram()
			assert.Equal(t, uint64(2), hist.GetSampleCount())
		}
	}
}

func TestMetrics_Histogram_AckLatency(t *testing.T) {
	reg := prometheus.NewRegistry()
	cfg := MetricsConfig{Registry: reg}
	m := NewMetrics(cfg, nil, nil, nil)

	m.RecordAckLatency(5.0)
	m.RecordAckLatency(10.0)
	m.RecordAckLatency(15.0)

	families, _ := reg.Gather()
	for _, f := range families {
		if f.GetName() == "alebus_ingestor_ack_latency_ms" {
			hist := f.GetMetric()[0].GetHistogram()
			assert.Equal(t, uint64(3), hist.GetSampleCount())
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Null Metrics Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestNullMetrics(t *testing.T) {
	m := NullMetrics{}

	// All methods should be no-ops (no panic)
	m.RecordOutcome(OutcomeAccepted, 10)
	m.RecordReceived()
	m.RecordRedisLatency(5)
	m.RecordAckLatency(3)
	m.RecordRetry()
	m.SetConnectionState(StateConnected)
}

// ─────────────────────────────────────────────────────────────────────────────
// Default Config Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestDefaultMetricsConfig(t *testing.T) {
	cfg := DefaultMetricsConfig()

	assert.NotNil(t, cfg.Registry)
	assert.NotEmpty(t, cfg.LatencyBuckets)
	assert.Contains(t, cfg.LatencyBuckets, float64(100))
}

// ─────────────────────────────────────────────────────────────────────────────
// Integration with Outcome Callback
// ─────────────────────────────────────────────────────────────────────────────

func TestMetrics_AsWorkerPoolCallback(t *testing.T) {
	reg := prometheus.NewRegistry()
	cfg := MetricsConfig{Registry: reg}
	m := NewMetrics(cfg, nil, nil, nil)

	// Simulate the callback that would be set on WorkerPool
	callback := func(outcome MessageOutcome, latency time.Duration) {
		m.RecordOutcome(outcome, float64(latency.Milliseconds()))
	}

	// Simulate processing outcomes
	callback(OutcomeAccepted, 10*time.Millisecond)
	callback(OutcomeAccepted, 15*time.Millisecond)
	callback(OutcomeStale, 5*time.Millisecond)

	assert.Equal(t, float64(2), testutil.ToFloat64(m.AcceptedTotal))
	assert.Equal(t, float64(1), testutil.ToFloat64(m.StaleTotal))
}

// ─────────────────────────────────────────────────────────────────────────────
// GPS Enrichment Metrics Tests (Phase 6 - Option B+)
// ─────────────────────────────────────────────────────────────────────────────

func TestMetrics_GPSEnrichmentTotal(t *testing.T) {
	reg := prometheus.NewRegistry()
	cfg := MetricsConfig{Registry: reg}
	m := NewMetrics(cfg, nil, nil, nil)

	// Record various GPS outcomes
	m.RecordGPSOutcome(GPSStatusSuccess, 10)
	m.RecordGPSOutcome(GPSStatusSuccess, 15)
	m.RecordGPSOutcome(GPSStatusNoAssignment, 5)
	m.RecordGPSOutcome(GPSStatusRouteNotFound, 3)
	m.RecordGPSOutcome(GPSStatusInvalid, 2)
	m.RecordGPSOutcome(GPSStatusInfraError, 8)
	m.RecordGPSOutcome(GPSStatusDisabled, 1)

	// Verify counters by label
	assert.Equal(t, float64(2), testutil.ToFloat64(m.GPSEnrichmentTotal.WithLabelValues("success")))
	assert.Equal(t, float64(1), testutil.ToFloat64(m.GPSEnrichmentTotal.WithLabelValues("no_assignment")))
	assert.Equal(t, float64(1), testutil.ToFloat64(m.GPSEnrichmentTotal.WithLabelValues("route_not_found")))
	assert.Equal(t, float64(1), testutil.ToFloat64(m.GPSEnrichmentTotal.WithLabelValues("invalid")))
	assert.Equal(t, float64(1), testutil.ToFloat64(m.GPSEnrichmentTotal.WithLabelValues("infra_error")))
	assert.Equal(t, float64(1), testutil.ToFloat64(m.GPSEnrichmentTotal.WithLabelValues("disabled")))
}

func TestMetrics_GPSEnrichmentLatency(t *testing.T) {
	reg := prometheus.NewRegistry()
	cfg := MetricsConfig{Registry: reg}
	m := NewMetrics(cfg, nil, nil, nil)

	// Record latencies
	m.RecordGPSOutcome(GPSStatusSuccess, 5)
	m.RecordGPSOutcome(GPSStatusSuccess, 10)
	m.RecordGPSOutcome(GPSStatusSuccess, 15)

	// Verify histogram has observations
	families, _ := reg.Gather()
	for _, f := range families {
		if f.GetName() == "alebus_ingestor_gps_enrichment_latency_ms" {
			hist := f.GetMetric()[0].GetHistogram()
			assert.Equal(t, uint64(3), hist.GetSampleCount())
			// Sum should be 5+10+15 = 30
			assert.Equal(t, float64(30), hist.GetSampleSum())
		}
	}
}

func TestMetrics_GPSResolverConfidence(t *testing.T) {
	reg := prometheus.NewRegistry()
	cfg := MetricsConfig{Registry: reg}
	m := NewMetrics(cfg, nil, nil, nil)

	// Record confidence values
	m.RecordGPSResolverConfidence(0.95)
	m.RecordGPSResolverConfidence(0.85)
	m.RecordGPSResolverConfidence(0.75)
	m.RecordGPSResolverConfidence(0.50)

	// Verify histogram
	families, _ := reg.Gather()
	for _, f := range families {
		if f.GetName() == "alebus_ingestor_gps_resolver_confidence" {
			hist := f.GetMetric()[0].GetHistogram()
			assert.Equal(t, uint64(4), hist.GetSampleCount())
		}
	}
}

func TestMetrics_GPSResolverMode(t *testing.T) {
	reg := prometheus.NewRegistry()
	cfg := MetricsConfig{Registry: reg}
	m := NewMetrics(cfg, nil, nil, nil)

	// Set mode counts
	m.SetGPSResolverModeCount("BOOTSTRAP", 5)
	m.SetGPSResolverModeCount("TRACKING", 10)
	m.SetGPSResolverModeCount("REACQUIRE", 2)

	// Verify gauges
	assert.Equal(t, float64(5), testutil.ToFloat64(m.GPSResolverMode.WithLabelValues("BOOTSTRAP")))
	assert.Equal(t, float64(10), testutil.ToFloat64(m.GPSResolverMode.WithLabelValues("TRACKING")))
	assert.Equal(t, float64(2), testutil.ToFloat64(m.GPSResolverMode.WithLabelValues("REACQUIRE")))

	// Test inc/dec
	m.IncGPSResolverMode("TRACKING")
	assert.Equal(t, float64(11), testutil.ToFloat64(m.GPSResolverMode.WithLabelValues("TRACKING")))

	m.DecGPSResolverMode("TRACKING")
	assert.Equal(t, float64(10), testutil.ToFloat64(m.GPSResolverMode.WithLabelValues("TRACKING")))
}

func TestMetrics_GPSCacheCounters(t *testing.T) {
	reg := prometheus.NewRegistry()
	cfg := MetricsConfig{Registry: reg}
	m := NewMetrics(cfg, nil, nil, nil)

	// Record cache hits/misses
	m.RecordGPSAssignmentCacheHit()
	m.RecordGPSAssignmentCacheHit()
	m.RecordGPSAssignmentCacheMiss()

	m.RecordGPSGeometryCacheHit()
	m.RecordGPSGeometryCacheHit()
	m.RecordGPSGeometryCacheHit()
	m.RecordGPSGeometryCacheMiss()

	// Verify counters
	assert.Equal(t, float64(2), testutil.ToFloat64(m.GPSAssignmentCacheHit))
	assert.Equal(t, float64(1), testutil.ToFloat64(m.GPSAssignmentCacheMiss))
	assert.Equal(t, float64(3), testutil.ToFloat64(m.GPSGeometryCacheHit))
	assert.Equal(t, float64(1), testutil.ToFloat64(m.GPSGeometryCacheMiss))
}

func TestMetrics_GPSRetryExhausted(t *testing.T) {
	reg := prometheus.NewRegistry()
	cfg := MetricsConfig{Registry: reg}
	m := NewMetrics(cfg, nil, nil, nil)

	m.RecordGPSRetryExhausted()
	m.RecordGPSRetryExhausted()
	m.RecordGPSRetryExhausted()

	assert.Equal(t, float64(3), testutil.ToFloat64(m.GPSRetryExhausted))
}

func TestMapOutcomeToGPSStatus(t *testing.T) {
	tests := []struct {
		outcome  MessageOutcome
		expected GPSEnrichmentStatus
	}{
		{OutcomeGPSSuccess, GPSStatusSuccess},
		{OutcomeGPSNoAssignment, GPSStatusNoAssignment},
		{OutcomeGPSRouteNotFound, GPSStatusRouteNotFound},
		{OutcomeGPSInvalid, GPSStatusInvalid},
		{OutcomeGPSReplayIgnored, GPSStatusReplayIgnored},
		{OutcomeGPSFutureHardRejected, GPSStatusFutureHardRejected},
		{OutcomeInfraError, GPSStatusInfraError},
		{OutcomeGPSEnrichmentDisabled, GPSStatusDisabled},
		{OutcomeUnknownTopic, GPSStatusUnknownTopic},
		// Default case - any unknown outcome maps to invalid
		{OutcomeAccepted, GPSStatusInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.outcome.String(), func(t *testing.T) {
			result := MapOutcomeToGPSStatus(tt.outcome)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNullMetrics_GPS(t *testing.T) {
	m := NullMetrics{}

	// All GPS methods should be no-ops (no panic)
	m.RecordGPSOutcome(GPSStatusSuccess, 10)
	m.RecordGPSResolverConfidence(0.95)
	m.SetGPSResolverModeCount("TRACKING", 5)
	m.IncGPSResolverMode("TRACKING")
	m.DecGPSResolverMode("TRACKING")
	m.RecordGPSAssignmentCacheHit()
	m.RecordGPSAssignmentCacheMiss()
	m.RecordGPSGeometryCacheHit()
	m.RecordGPSGeometryCacheMiss()
	m.RecordGPSRetryExhausted()
}

func TestMetrics_GPSOutcome_ZeroLatency(t *testing.T) {
	reg := prometheus.NewRegistry()
	cfg := MetricsConfig{Registry: reg}
	m := NewMetrics(cfg, nil, nil, nil)

	// Record with zero latency - should still increment counter but not latency histogram
	m.RecordGPSOutcome(GPSStatusSuccess, 0)

	assert.Equal(t, float64(1), testutil.ToFloat64(m.GPSEnrichmentTotal.WithLabelValues("success")))

	// Verify no latency observation
	families, _ := reg.Gather()
	for _, f := range families {
		if f.GetName() == "alebus_ingestor_gps_enrichment_latency_ms" {
			hist := f.GetMetric()[0].GetHistogram()
			assert.Equal(t, uint64(0), hist.GetSampleCount())
		}
	}
}

func TestMetrics_GPSRegistration(t *testing.T) {
	reg := prometheus.NewRegistry()
	cfg := MetricsConfig{Registry: reg}
	m := NewMetrics(cfg, nil, nil, nil)

	require.NotNil(t, m.GPSEnrichmentTotal)
	require.NotNil(t, m.GPSEnrichmentLatency)
	require.NotNil(t, m.GPSResolverConfidence)
	require.NotNil(t, m.GPSResolverMode)
	require.NotNil(t, m.GPSAssignmentCacheHit)
	require.NotNil(t, m.GPSAssignmentCacheMiss)
	require.NotNil(t, m.GPSGeometryCacheHit)
	require.NotNil(t, m.GPSGeometryCacheMiss)
	require.NotNil(t, m.GPSRetryExhausted)
}
