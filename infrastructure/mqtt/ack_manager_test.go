package mqtt

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────────────────────────────────────
// In-Order ACK Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestAckManager_InOrder(t *testing.T) {
	am := NewAckManager(DefaultAckManagerConfig())
	defer am.Stop()

	// Track ACKs sent
	var ackedSeqs []uint64
	var mu sync.Mutex

	// Register 5 messages
	for i := 0; i < 5; i++ {
		seq := uint64(i + 1)
		ackFn := func() error {
			mu.Lock()
			ackedSeqs = append(ackedSeqs, seq)
			mu.Unlock()
			return nil
		}
		recvSeq, ok := am.Register(ackFn)
		require.True(t, ok)
		assert.Equal(t, seq, recvSeq)
	}

	// Complete in order
	for i := 1; i <= 5; i++ {
		ok := am.Complete(uint64(i), AckDecisionACK)
		require.True(t, ok)
		am.Flush()
	}

	// Verify ACKs sent in order
	mu.Lock()
	assert.Equal(t, []uint64{1, 2, 3, 4, 5}, ackedSeqs)
	mu.Unlock()
}

func TestAckManager_OutOfOrder(t *testing.T) {
	am := NewAckManager(DefaultAckManagerConfig())
	defer am.Stop()

	// Track ACKs sent
	var ackedSeqs []uint64
	var mu sync.Mutex

	makeAckFn := func(seq uint64) AckFunc {
		return func() error {
			mu.Lock()
			ackedSeqs = append(ackedSeqs, seq)
			mu.Unlock()
			return nil
		}
	}

	// Register 5 messages
	for i := 1; i <= 5; i++ {
		_, ok := am.Register(makeAckFn(uint64(i)))
		require.True(t, ok)
	}

	// Complete out of order: 3, 1, 5, 2, 4
	am.Complete(3, AckDecisionACK)
	am.Flush()
	mu.Lock()
	assert.Empty(t, ackedSeqs) // Can't ACK 3 until 1,2 done
	mu.Unlock()

	am.Complete(1, AckDecisionACK)
	am.Flush()
	mu.Lock()
	assert.Equal(t, []uint64{1}, ackedSeqs) // Now 1 can go
	mu.Unlock()

	am.Complete(5, AckDecisionACK)
	am.Flush()
	mu.Lock()
	assert.Equal(t, []uint64{1}, ackedSeqs) // Still waiting for 2
	mu.Unlock()

	am.Complete(2, AckDecisionACK)
	am.Flush()
	mu.Lock()
	assert.Equal(t, []uint64{1, 2, 3}, ackedSeqs) // Now 2,3 can go
	mu.Unlock()

	am.Complete(4, AckDecisionACK)
	am.Flush()
	mu.Lock()
	assert.Equal(t, []uint64{1, 2, 3, 4, 5}, ackedSeqs) // Now 4,5 can go
	mu.Unlock()
}

// ─────────────────────────────────────────────────────────────────────────────
// Retry Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestAckManager_RetryThenAck(t *testing.T) {
	am := NewAckManager(DefaultAckManagerConfig())
	defer am.Stop()

	var ackedSeqs []uint64
	var mu sync.Mutex

	makeAckFn := func(seq uint64) AckFunc {
		return func() error {
			mu.Lock()
			ackedSeqs = append(ackedSeqs, seq)
			mu.Unlock()
			return nil
		}
	}

	// Register 3 messages
	for i := 1; i <= 3; i++ {
		am.Register(makeAckFn(uint64(i)))
	}

	// Message 1 needs retry
	am.Complete(1, AckDecisionRetry)
	am.Flush()

	// Message 2 completes successfully
	am.Complete(2, AckDecisionACK)
	am.Flush()

	mu.Lock()
	assert.Empty(t, ackedSeqs) // Blocked by message 1
	mu.Unlock()

	// Message 3 completes successfully
	am.Complete(3, AckDecisionACK)
	am.Flush()

	mu.Lock()
	assert.Empty(t, ackedSeqs) // Still blocked
	mu.Unlock()

	// Retry succeeds, update to ACK
	ok := am.UpdateRetryToAck(1)
	require.True(t, ok)
	am.Flush()

	mu.Lock()
	assert.Equal(t, []uint64{1, 2, 3}, ackedSeqs) // All unblocked
	mu.Unlock()
}

func TestAckManager_RetryBlocks(t *testing.T) {
	am := NewAckManager(DefaultAckManagerConfig())
	defer am.Stop()

	var ackCount int
	ackFn := func() error {
		ackCount++
		return nil
	}

	am.Register(ackFn)
	am.Register(ackFn)

	// First message retries
	am.Complete(1, AckDecisionRetry)
	am.Complete(2, AckDecisionACK)
	am.Flush()

	// Neither should be ACKed (1 blocking)
	assert.Equal(t, 0, ackCount)
	assert.Equal(t, 2, am.PendingCount())
}

// ─────────────────────────────────────────────────────────────────────────────
// Connection-Scoped Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestAckManager_ConnectionScoped(t *testing.T) {
	am := NewAckManager(DefaultAckManagerConfig())

	var ackCount int
	ackFn := func() error {
		ackCount++
		return nil
	}

	// Register some messages
	am.Register(ackFn)
	am.Register(ackFn)

	// Stop (simulates disconnect)
	am.Stop()

	// Completing after stop should fail
	ok := am.Complete(1, AckDecisionACK)
	assert.False(t, ok)

	// Flush should do nothing
	flushed := am.Flush()
	assert.Equal(t, 0, flushed)

	// No ACKs should have been sent
	assert.Equal(t, 0, ackCount)

	// Register should fail
	_, ok = am.Register(ackFn)
	assert.False(t, ok)

	assert.True(t, am.IsStopped())
}

func TestAckManager_StopClearsPending(t *testing.T) {
	am := NewAckManager(DefaultAckManagerConfig())

	ackFn := func() error { return nil }

	am.Register(ackFn)
	am.Register(ackFn)
	am.Register(ackFn)

	assert.Equal(t, 3, am.PendingCount())

	am.Stop()

	assert.Equal(t, 0, am.PendingCount())
}

// ─────────────────────────────────────────────────────────────────────────────
// Bounded Pending Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestAckManager_BoundedPending(t *testing.T) {
	cfg := AckManagerConfig{
		PendingTimeout: 1 * time.Hour, // No timeout eviction
		MaxPendingSize: 5,             // Small limit
	}
	am := NewAckManager(cfg)
	defer am.Stop()

	ackFn := func() error { return nil }

	// Register up to limit
	for i := 0; i < 5; i++ {
		_, ok := am.Register(ackFn)
		require.True(t, ok)
	}

	assert.Equal(t, 5, am.PendingCount())

	// Register more — should evict oldest
	_, ok := am.Register(ackFn)
	require.True(t, ok)

	// Still at max (one evicted)
	assert.Equal(t, 5, am.PendingCount())

	stats := am.Stats()
	assert.Equal(t, uint64(1), stats.Evicted)
}

func TestAckManager_TimeoutEviction(t *testing.T) {
	cfg := AckManagerConfig{
		PendingTimeout: 50 * time.Millisecond,
		MaxPendingSize: 100,
	}
	am := NewAckManager(cfg)
	defer am.Stop()

	ackFn := func() error { return nil }

	// Register messages
	am.Register(ackFn)
	am.Register(ackFn)

	assert.Equal(t, 2, am.PendingCount())

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// Trigger eviction
	am.evictTimedOut()

	assert.Equal(t, 0, am.PendingCount())

	stats := am.Stats()
	assert.Equal(t, uint64(2), stats.Evicted)
}

// ─────────────────────────────────────────────────────────────────────────────
// FlushLoop Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestAckManager_FlushLoop(t *testing.T) {
	am := NewAckManager(DefaultAckManagerConfig())

	var ackCount atomic.Int32
	ackFn := func() error {
		ackCount.Add(1)
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start flush loop
	go am.FlushLoop(ctx)

	// Register and complete
	am.Register(ackFn)
	am.Register(ackFn)
	am.Complete(1, AckDecisionACK)
	am.Complete(2, AckDecisionACK)

	// Wait for flush loop to process
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, int32(2), ackCount.Load())

	// Stop
	am.Stop()
}

func TestAckManager_FlushLoopStopsOnContext(t *testing.T) {
	am := NewAckManager(DefaultAckManagerConfig())

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		am.FlushLoop(ctx)
		close(done)
	}()

	// Cancel context
	cancel()

	// FlushLoop should exit
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("FlushLoop did not exit on context cancellation")
	}
}

func TestAckManager_FlushLoopStopsOnStop(t *testing.T) {
	am := NewAckManager(DefaultAckManagerConfig())

	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		am.FlushLoop(ctx)
		close(done)
	}()

	// Stop manager
	am.Stop()

	// FlushLoop should exit
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("FlushLoop did not exit on Stop")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Stats Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestAckManager_Stats(t *testing.T) {
	am := NewAckManager(DefaultAckManagerConfig())
	defer am.Stop()

	ackFn := func() error { return nil }

	// Initial stats
	stats := am.Stats()
	assert.Equal(t, 0, stats.Pending)
	assert.Equal(t, uint64(0), stats.Acked)
	assert.Equal(t, uint64(1), stats.NextRecvSeq)
	assert.Equal(t, uint64(1), stats.NextAckSeq)

	// Register and complete
	am.Register(ackFn)
	am.Register(ackFn)
	am.Complete(1, AckDecisionACK)
	am.Flush()

	stats = am.Stats()
	assert.Equal(t, 1, stats.Pending)       // One still pending
	assert.Equal(t, uint64(1), stats.Acked) // One ACKed
	assert.Equal(t, uint64(3), stats.NextRecvSeq)
	assert.Equal(t, uint64(2), stats.NextAckSeq)
}

func TestAckManager_OutOfOrderMetric(t *testing.T) {
	am := NewAckManager(DefaultAckManagerConfig())
	defer am.Stop()

	ackFn := func() error { return nil }

	am.Register(ackFn)
	am.Register(ackFn)
	am.Register(ackFn)

	// Complete out of order
	am.Complete(2, AckDecisionACK) // Out of order (nextAckSeq=1, received 2)
	am.Complete(1, AckDecisionACK) // In order (nextAckSeq=1, received 1)
	am.Flush()                     // Advances nextAckSeq to 3
	am.Complete(3, AckDecisionACK) // In order (nextAckSeq=3, received 3)

	stats := am.Stats()
	assert.Equal(t, uint64(1), stats.OutOfOrder)
}

// ─────────────────────────────────────────────────────────────────────────────
// Edge Cases
// ─────────────────────────────────────────────────────────────────────────────

func TestAckManager_CompleteUnknownSeq(t *testing.T) {
	am := NewAckManager(DefaultAckManagerConfig())
	defer am.Stop()

	ok := am.Complete(999, AckDecisionACK)
	assert.False(t, ok)
}

func TestAckManager_UpdateRetryUnknown(t *testing.T) {
	am := NewAckManager(DefaultAckManagerConfig())
	defer am.Stop()

	ok := am.UpdateRetryToAck(999)
	assert.False(t, ok)
}

func TestAckManager_UpdateRetryNotInRetryState(t *testing.T) {
	am := NewAckManager(DefaultAckManagerConfig())
	defer am.Stop()

	ackFn := func() error { return nil }
	am.Register(ackFn)

	// Complete with ACK, not Retry
	am.Complete(1, AckDecisionACK)

	// UpdateRetryToAck should fail
	ok := am.UpdateRetryToAck(1)
	assert.False(t, ok)
}

func TestAckManager_NilAckFn(t *testing.T) {
	am := NewAckManager(DefaultAckManagerConfig())
	defer am.Stop()

	// Register with nil ackFn
	recvSeq, ok := am.Register(nil)
	require.True(t, ok)
	assert.Equal(t, uint64(1), recvSeq)

	// Complete and flush should not panic
	am.Complete(1, AckDecisionACK)
	flushed := am.Flush()

	assert.Equal(t, 1, flushed)
}

// ─────────────────────────────────────────────────────────────────────────────
// Concurrent Access Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestAckManager_ConcurrentCompleteFlush(t *testing.T) {
	am := NewAckManager(DefaultAckManagerConfig())
	defer am.Stop()

	var ackCount atomic.Int32
	ackFn := func() error {
		ackCount.Add(1)
		return nil
	}

	// Register 100 messages
	for i := 0; i < 100; i++ {
		am.Register(ackFn)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start flush loop
	go am.FlushLoop(ctx)

	// Complete concurrently from multiple goroutines
	var wg sync.WaitGroup
	for i := 1; i <= 100; i++ {
		wg.Add(1)
		go func(seq uint64) {
			defer wg.Done()
			am.Complete(seq, AckDecisionACK)
		}(uint64(i))
	}

	wg.Wait()

	// Wait for flush loop
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, int32(100), ackCount.Load())
}
