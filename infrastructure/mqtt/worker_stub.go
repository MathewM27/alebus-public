package mqtt

// WorkerPool is a placeholder for the omitted MQTT consumer worker pool. The
// concrete type — which orchestrates ingestion, retries, and ack ordering for
// inbound GPS messages — is not included in this public release. The stub is
// retained so the metrics collector can reference `*WorkerPool` and `WorkerPoolStats`.
type WorkerPool struct{}

// WorkerPoolStats is the gauge snapshot the metrics layer reads from a pool.
type WorkerPoolStats struct {
	Inflight int
}

// Stats returns the current snapshot. The stub always reports zero in-flight.
func (p *WorkerPool) Stats() WorkerPoolStats { return WorkerPoolStats{} }
