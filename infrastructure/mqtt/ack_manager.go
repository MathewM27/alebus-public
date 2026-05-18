package mqtt

import (
	"context"
	"sync"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// ACK Manager — Connection-Scoped Ordered Acknowledgements
// ─────────────────────────────────────────────────────────────────────────────
//
// Per Phase 3 blueprint and ADR-001:
//   - MQTT QoS 1 requires ACKs to be sent in order (by RecvSeq)
//   - Workers process messages out-of-order but ACKs must be sequential
//   - ACK manager is connection-scoped (reset on reconnect)
//   - Bounded pending map (entries evicted on timeout to prevent leak)
//
// Flow:
//   1. Message received → Register() assigns RecvSeq, stores PendingAck
//   2. Worker processes → calls Complete() with AckDecision
//   3. Flush loop → sends ACKs in order, respecting Retry decisions
//
// ─────────────────────────────────────────────────────────────────────────────

// AckFunc is the function signature for acknowledging an MQTT message.
// In production, this wraps paho.Client.Ack(*paho.Publish).
type AckFunc func() error

// PendingAck represents a message awaiting acknowledgement.
type PendingAck struct {
	RecvSeq    uint64
	AckFn      AckFunc     // Function to call to send PUBACK
	Decision   AckDecision // Set when worker completes
	Completed  bool        // True once worker has reported decision
	RegisterAt time.Time   // When this ack was registered
	CompleteAt time.Time   // When worker completed (for latency metrics)
}

// AckManager manages ordered MQTT acknowledgements for a single connection.
//
// It ensures ACKs are sent in strict RecvSeq order even when workers
// process messages concurrently and complete out-of-order.
type AckManager struct {
	mu sync.Mutex

	// Sequence tracking
	nextRecvSeq uint64                 // Next sequence to assign
	nextAckSeq  uint64                 // Next sequence to ACK
	pending     map[uint64]*PendingAck // RecvSeq → PendingAck

	// Configuration
	pendingTimeout time.Duration // Max time before evicting stale pending
	maxPendingSize int           // Max pending entries (prevents unbounded growth)

	// Lifecycle
	stopped bool
	stopCh  chan struct{}
	flushCh chan struct{} // Signals flush loop to check pending

	// Metrics
	ackedCount      uint64
	retriedCount    uint64
	evictedCount    uint64
	outOfOrderCount uint64
}

// AckManagerConfig configures the ACK manager.
type AckManagerConfig struct {
	// PendingTimeout is the maximum time a pending ack can wait before eviction.
	// Default: 30 seconds
	PendingTimeout time.Duration

	// MaxPendingSize is the maximum number of pending acks before oldest are evicted.
	// Default: 10000
	MaxPendingSize int
}

// DefaultAckManagerConfig returns sensible defaults.
func DefaultAckManagerConfig() AckManagerConfig {
	return AckManagerConfig{
		PendingTimeout: 30 * time.Second,
		MaxPendingSize: 10000,
	}
}

// NewAckManager creates a new connection-scoped ACK manager.
func NewAckManager(cfg AckManagerConfig) *AckManager {
	if cfg.PendingTimeout <= 0 {
		cfg.PendingTimeout = 30 * time.Second
	}
	if cfg.MaxPendingSize <= 0 {
		cfg.MaxPendingSize = 10000
	}

	am := &AckManager{
		nextRecvSeq:    1, // Start at 1 (0 is sentinel)
		nextAckSeq:     1,
		pending:        make(map[uint64]*PendingAck),
		pendingTimeout: cfg.PendingTimeout,
		maxPendingSize: cfg.MaxPendingSize,
		stopCh:         make(chan struct{}),
		flushCh:        make(chan struct{}, 1), // Buffered to avoid blocking
	}

	return am
}

// Register assigns a RecvSeq to an incoming message and tracks it for ACK.
//
// Returns:
//   - recvSeq: the assigned sequence number
//   - ok: false if the manager is stopped
//
// The caller must eventually call Complete(recvSeq, decision) or the entry
// will be evicted after pendingTimeout.
func (am *AckManager) Register(ackFn AckFunc) (recvSeq uint64, ok bool) {
	am.mu.Lock()
	defer am.mu.Unlock()

	if am.stopped {
		return 0, false
	}

	// Evict old entries if we're at capacity
	am.evictOldestIfNeeded()

	recvSeq = am.nextRecvSeq
	am.nextRecvSeq++

	am.pending[recvSeq] = &PendingAck{
		RecvSeq:    recvSeq,
		AckFn:      ackFn,
		Completed:  false,
		RegisterAt: time.Now(),
	}

	return recvSeq, true
}

// Complete marks a message as processed with the given decision.
//
// Parameters:
//   - recvSeq: the sequence number returned by Register
//   - decision: AckDecisionACK or AckDecisionRetry
//
// If decision is AckDecisionRetry, the ACK is withheld until the caller
// calls Complete again with AckDecisionACK (after successful retry).
//
// Returns false if recvSeq is unknown or manager is stopped.
func (am *AckManager) Complete(recvSeq uint64, decision AckDecision) bool {
	am.mu.Lock()
	defer am.mu.Unlock()

	if am.stopped {
		return false
	}

	pending, ok := am.pending[recvSeq]
	if !ok {
		return false // Already evicted or invalid
	}

	pending.Decision = decision
	pending.Completed = true
	pending.CompleteAt = time.Now()

	// Track out-of-order completions for metrics
	if recvSeq != am.nextAckSeq {
		am.outOfOrderCount++
	}

	// Signal flush loop
	am.signalFlush()

	return true
}

// Flush processes completed ACKs in order.
//
// This should be called periodically or when signaled. It sends ACKs
// for all consecutive completed entries starting from nextAckSeq.
//
// Returns the number of ACKs sent.
func (am *AckManager) Flush() int {
	am.mu.Lock()
	defer am.mu.Unlock()

	if am.stopped {
		return 0
	}

	acked := 0

	for {
		pending, ok := am.pending[am.nextAckSeq]
		if !ok {
			break // Gap in sequence
		}

		if !pending.Completed {
			break // Waiting for worker to complete
		}

		if pending.Decision == AckDecisionRetry {
			break // Worker requested retry, wait for next Complete call
		}

		// ACK decision — send PUBACK
		if pending.AckFn != nil {
			// Ignore error — if ACK fails, broker will redeliver
			_ = pending.AckFn()
		}

		delete(am.pending, am.nextAckSeq)
		am.nextAckSeq++
		am.ackedCount++
		acked++
	}

	return acked
}

// FlushLoop runs a continuous loop that flushes ACKs.
//
// Call this in a goroutine. It returns when Stop() is called or ctx is cancelled.
//
// Example:
//
//	go am.FlushLoop(ctx)
func (am *AckManager) FlushLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	evictTicker := time.NewTicker(1 * time.Second)
	defer evictTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-am.stopCh:
			return
		case <-am.flushCh:
			am.Flush()
		case <-ticker.C:
			am.Flush()
		case <-evictTicker.C:
			am.evictTimedOut()
		}
	}
}

// Stop stops the ACK manager and prevents further ACKs.
//
// This is connection-scoped: call Stop when the MQTT connection drops.
// Any pending ACKs are discarded (broker will redeliver on reconnect).
func (am *AckManager) Stop() {
	am.mu.Lock()
	defer am.mu.Unlock()

	if am.stopped {
		return
	}

	am.stopped = true
	close(am.stopCh)

	// Clear pending — broker will redeliver unacked messages on reconnect
	am.pending = make(map[uint64]*PendingAck)
}

// IsStopped returns true if the manager has been stopped.
func (am *AckManager) IsStopped() bool {
	am.mu.Lock()
	defer am.mu.Unlock()
	return am.stopped
}

// PendingCount returns the number of pending (unacked) messages.
func (am *AckManager) PendingCount() int {
	am.mu.Lock()
	defer am.mu.Unlock()
	return len(am.pending)
}

// Stats returns ACK manager statistics.
type AckManagerStats struct {
	Pending     int    // Current pending ACKs
	Acked       uint64 // Total ACKs sent
	Retried     uint64 // Total retry decisions
	Evicted     uint64 // Total evicted (timed out)
	OutOfOrder  uint64 // Completions received out-of-order
	NextRecvSeq uint64 // Next sequence to assign
	NextAckSeq  uint64 // Next sequence to ACK
}

// Stats returns current statistics.
func (am *AckManager) Stats() AckManagerStats {
	am.mu.Lock()
	defer am.mu.Unlock()

	return AckManagerStats{
		Pending:     len(am.pending),
		Acked:       am.ackedCount,
		Retried:     am.retriedCount,
		Evicted:     am.evictedCount,
		OutOfOrder:  am.outOfOrderCount,
		NextRecvSeq: am.nextRecvSeq,
		NextAckSeq:  am.nextAckSeq,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ─────────────────────────────────────────────────────────────────────────────

// signalFlush signals the flush loop to check pending ACKs.
// Non-blocking (drops signal if channel is full).
func (am *AckManager) signalFlush() {
	select {
	case am.flushCh <- struct{}{}:
	default:
	}
}

// evictOldestIfNeeded removes oldest entry if at capacity.
// Caller must hold am.mu.
func (am *AckManager) evictOldestIfNeeded() {
	if len(am.pending) < am.maxPendingSize {
		return
	}

	// Find oldest by RegisterAt
	var oldest *PendingAck
	for _, p := range am.pending {
		if oldest == nil || p.RegisterAt.Before(oldest.RegisterAt) {
			oldest = p
		}
	}

	if oldest != nil {
		delete(am.pending, oldest.RecvSeq)
		am.evictedCount++

		// If we evicted an entry before nextAckSeq, advance nextAckSeq
		if oldest.RecvSeq == am.nextAckSeq {
			am.advanceNextAckSeq()
		}
	}
}

// evictTimedOut removes entries that have exceeded pendingTimeout.
func (am *AckManager) evictTimedOut() {
	am.mu.Lock()
	defer am.mu.Unlock()

	if am.stopped {
		return
	}

	now := time.Now()
	cutoff := now.Add(-am.pendingTimeout)

	for seq, p := range am.pending {
		if p.RegisterAt.Before(cutoff) {
			delete(am.pending, seq)
			am.evictedCount++
		}
	}

	// Advance nextAckSeq past any gaps
	am.advanceNextAckSeq()
}

// advanceNextAckSeq skips over any missing sequence numbers.
// Caller must hold am.mu.
func (am *AckManager) advanceNextAckSeq() {
	// Advance past gaps (evicted entries)
	for am.nextAckSeq < am.nextRecvSeq {
		if _, ok := am.pending[am.nextAckSeq]; ok {
			break
		}
		am.nextAckSeq++
	}
}

// UpdateRetryToAck changes a Retry decision to ACK after successful retry.
// This allows the flush loop to proceed past this entry.
//
// Returns false if recvSeq is unknown or not in Retry state.
func (am *AckManager) UpdateRetryToAck(recvSeq uint64) bool {
	am.mu.Lock()
	defer am.mu.Unlock()

	if am.stopped {
		return false
	}

	pending, ok := am.pending[recvSeq]
	if !ok {
		return false
	}

	if !pending.Completed || pending.Decision != AckDecisionRetry {
		return false
	}

	pending.Decision = AckDecisionACK
	am.signalFlush()

	return true
}
