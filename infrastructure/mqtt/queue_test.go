package mqtt

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────────────────────────────────────
// Queue Enqueue Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestBoundedQueue_Enqueue(t *testing.T) {
	q := NewBoundedQueue(5, "drop_new")

	// Enqueue items up to capacity
	for i := 0; i < 5; i++ {
		item := NewWorkItem(uint64(i), "bus-"+string(rune('A'+i)), []byte(`{}`))
		result := q.Enqueue(item)
		assert.Equal(t, EnqueueSuccess, result, "item %d should succeed", i)
	}

	assert.Equal(t, 5, q.Len())
	assert.Equal(t, 5, q.Cap())
}

func TestBoundedQueue_Full_DropNew(t *testing.T) {
	q := NewBoundedQueue(3, "drop_new")

	// Fill the queue
	for i := 0; i < 3; i++ {
		item := NewWorkItem(uint64(i), "bus-"+string(rune('A'+i)), []byte(`{}`))
		result := q.Enqueue(item)
		require.Equal(t, EnqueueSuccess, result)
	}

	// Try to enqueue when full
	item := NewWorkItem(4, "bus-D", []byte(`{}`))
	result := q.Enqueue(item)

	assert.Equal(t, EnqueueDropped, result)
	assert.Equal(t, 3, q.Len())
	assert.Equal(t, uint64(1), q.DroppedCount())
}

func TestBoundedQueue_Full_Coalesce(t *testing.T) {
	q := NewBoundedQueue(3, "coalesce_latest")

	// Fill the queue with different buses
	for i := 0; i < 3; i++ {
		item := NewWorkItem(uint64(i), "bus-"+string(rune('A'+i)), []byte(`{"v":1}`))
		result := q.Enqueue(item)
		require.Equal(t, EnqueueSuccess, result)
	}

	// Enqueue update for existing bus (should coalesce)
	item := NewWorkItem(4, "bus-B", []byte(`{"v":2}`))
	result := q.Enqueue(item)

	assert.Equal(t, EnqueueCoalesced, result)
	assert.Equal(t, 3, q.Len()) // Count unchanged
	assert.Equal(t, uint64(1), q.CoalescedCount())

	// Verify the payload was updated by dequeuing
	// First item should be bus-A
	dequeued, ok := q.TryDequeue()
	require.True(t, ok)
	assert.Equal(t, "bus-A", dequeued.BusID)

	// Second item should be bus-B with updated payload
	dequeued, ok = q.TryDequeue()
	require.True(t, ok)
	assert.Equal(t, "bus-B", dequeued.BusID)
	assert.Equal(t, []byte(`{"v":2}`), dequeued.RawPayload)
}

func TestBoundedQueue_Full_CoalesceNoMatch(t *testing.T) {
	q := NewBoundedQueue(3, "coalesce_latest")

	// Fill the queue
	for i := 0; i < 3; i++ {
		item := NewWorkItem(uint64(i), "bus-"+string(rune('A'+i)), []byte(`{}`))
		result := q.Enqueue(item)
		require.Equal(t, EnqueueSuccess, result)
	}

	// Enqueue for a NEW bus (no coalesce match, should drop)
	item := NewWorkItem(4, "bus-NEW", []byte(`{}`))
	result := q.Enqueue(item)

	assert.Equal(t, EnqueueDropped, result)
	assert.Equal(t, uint64(1), q.DroppedCount())
}

// ─────────────────────────────────────────────────────────────────────────────
// Queue Dequeue Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestBoundedQueue_Dequeue(t *testing.T) {
	q := NewBoundedQueue(10, "drop_new")

	// Enqueue some items
	for i := 0; i < 3; i++ {
		item := NewWorkItem(uint64(i), "bus-"+string(rune('A'+i)), []byte(`{}`))
		q.Enqueue(item)
	}

	ctx := context.Background()

	// Dequeue and verify FIFO order
	item, ok := q.Dequeue(ctx)
	require.True(t, ok)
	assert.Equal(t, "bus-A", item.BusID)
	assert.Equal(t, uint64(0), item.RecvSeq)

	item, ok = q.Dequeue(ctx)
	require.True(t, ok)
	assert.Equal(t, "bus-B", item.BusID)

	item, ok = q.Dequeue(ctx)
	require.True(t, ok)
	assert.Equal(t, "bus-C", item.BusID)

	assert.Equal(t, 0, q.Len())
}

func TestBoundedQueue_DequeueBlocks(t *testing.T) {
	q := NewBoundedQueue(10, "drop_new")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start dequeue in goroutine (should block)
	var wg sync.WaitGroup
	var dequeuedItem *WorkItem
	var dequeueOk bool

	wg.Add(1)
	go func() {
		defer wg.Done()
		dequeuedItem, dequeueOk = q.Dequeue(ctx)
	}()

	// Give it time to start blocking
	time.Sleep(20 * time.Millisecond)

	// Enqueue an item
	item := NewWorkItem(1, "bus-X", []byte(`{}`))
	q.Enqueue(item)

	wg.Wait()

	assert.True(t, dequeueOk)
	assert.NotNil(t, dequeuedItem)
	assert.Equal(t, "bus-X", dequeuedItem.BusID)
}

func TestBoundedQueue_DequeueContextCancelled(t *testing.T) {
	q := NewBoundedQueue(10, "drop_new")

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	var dequeuedItem *WorkItem
	var dequeueOk bool

	wg.Add(1)
	go func() {
		defer wg.Done()
		dequeuedItem, dequeueOk = q.Dequeue(ctx)
	}()

	// Give dequeue time to start blocking
	time.Sleep(20 * time.Millisecond)

	// Cancel context
	cancel()

	wg.Wait()

	assert.False(t, dequeueOk)
	assert.Nil(t, dequeuedItem)
}

func TestBoundedQueue_TryDequeue(t *testing.T) {
	q := NewBoundedQueue(10, "drop_new")

	// Empty queue
	item, ok := q.TryDequeue()
	assert.False(t, ok)
	assert.Nil(t, item)

	// Add item
	q.Enqueue(NewWorkItem(1, "bus-A", []byte(`{}`)))

	// Try again
	item, ok = q.TryDequeue()
	assert.True(t, ok)
	assert.Equal(t, "bus-A", item.BusID)

	// Empty again
	item, ok = q.TryDequeue()
	assert.False(t, ok)
	assert.Nil(t, item)
}

// ─────────────────────────────────────────────────────────────────────────────
// Queue Shutdown Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestBoundedQueue_Shutdown(t *testing.T) {
	q := NewBoundedQueue(10, "drop_new")

	// Start multiple blocked dequeuers
	var wg sync.WaitGroup
	results := make(chan bool, 3)

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok := q.Dequeue(context.Background())
			results <- ok
		}()
	}

	// Give dequeuers time to start blocking
	time.Sleep(20 * time.Millisecond)

	// Close queue
	q.Close()

	wg.Wait()
	close(results)

	// All dequeuers should return false
	for ok := range results {
		assert.False(t, ok)
	}

	assert.True(t, q.IsClosed())
}

func TestBoundedQueue_EnqueueAfterClose(t *testing.T) {
	q := NewBoundedQueue(10, "drop_new")
	q.Close()

	item := NewWorkItem(1, "bus-A", []byte(`{}`))
	result := q.Enqueue(item)

	assert.Equal(t, EnqueueClosed, result)
}

func TestBoundedQueue_DrainAfterClose(t *testing.T) {
	q := NewBoundedQueue(10, "drop_new")

	// Add items before close
	q.Enqueue(NewWorkItem(1, "bus-A", []byte(`{}`)))
	q.Enqueue(NewWorkItem(2, "bus-B", []byte(`{}`)))

	q.Close()

	// Should still be able to drain existing items
	item, ok := q.Dequeue(context.Background())
	assert.True(t, ok)
	assert.Equal(t, "bus-A", item.BusID)

	item, ok = q.Dequeue(context.Background())
	assert.True(t, ok)
	assert.Equal(t, "bus-B", item.BusID)

	// Now should return false
	item, ok = q.Dequeue(context.Background())
	assert.False(t, ok)
	assert.Nil(t, item)
}

// ─────────────────────────────────────────────────────────────────────────────
// Queue Stats Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestBoundedQueue_Stats(t *testing.T) {
	q := NewBoundedQueue(5, "coalesce_latest")

	// Initial stats
	stats := q.Stats()
	assert.Equal(t, 0, stats.Depth)
	assert.Equal(t, 5, stats.Capacity)
	assert.Equal(t, uint64(0), stats.Dropped)
	assert.Equal(t, uint64(0), stats.Coalesced)

	// Add some items
	q.Enqueue(NewWorkItem(1, "bus-A", []byte(`{}`)))
	q.Enqueue(NewWorkItem(2, "bus-B", []byte(`{}`)))

	stats = q.Stats()
	assert.Equal(t, 2, stats.Depth)

	// Coalesce
	q.Enqueue(NewWorkItem(3, "bus-A", []byte(`{"updated":true}`)))

	stats = q.Stats()
	assert.Equal(t, 2, stats.Depth) // Still 2 (coalesced)
	assert.Equal(t, uint64(1), stats.Coalesced)
}

// ─────────────────────────────────────────────────────────────────────────────
// Concurrent Access Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestBoundedQueue_ConcurrentEnqueueDequeue(t *testing.T) {
	q := NewBoundedQueue(100, "drop_new")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	enqueueCount := 500
	dequeueCount := 0
	var mu sync.Mutex

	// Start enqueuers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < enqueueCount/5; j++ {
				item := NewWorkItem(uint64(workerID*100+j), "bus", []byte(`{}`))
				q.Enqueue(item)
			}
		}(i)
	}

	// Start dequeuers
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					if _, ok := q.TryDequeue(); ok {
						mu.Lock()
						dequeueCount++
						mu.Unlock()
					} else {
						time.Sleep(1 * time.Millisecond)
					}
				}
			}
		}()
	}

	// Wait for enqueuers to finish
	time.Sleep(100 * time.Millisecond)
	cancel()
	wg.Wait()

	// Drain remaining
	for {
		if _, ok := q.TryDequeue(); !ok {
			break
		}
		dequeueCount++
	}

	// Total dequeued + dropped should equal enqueued
	totalProcessed := dequeueCount + int(q.DroppedCount())
	assert.Equal(t, enqueueCount, totalProcessed)
}

// ─────────────────────────────────────────────────────────────────────────────
// Queue Policy Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestNewBoundedQueue_DefaultsInvalidInputs(t *testing.T) {
	// Invalid capacity defaults to 1
	q := NewBoundedQueue(0, "drop_new")
	assert.Equal(t, 1, q.Cap())

	// Invalid policy defaults to coalesce_latest
	q = NewBoundedQueue(10, "invalid_policy")
	assert.Equal(t, "coalesce_latest", q.dropPolicy)
}
