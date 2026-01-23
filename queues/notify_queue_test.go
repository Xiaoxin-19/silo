package queues

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestNotifyQueue_Basic(t *testing.T) {
	q := NewNotifyQueue[int](10, 0) // Unbounded
	ctx := context.Background()
	// Test Enqueue and Signal
	if err := q.EnqueueOrWait(ctx, 1); err != nil {
		t.Fatalf("Failed to enqueue: %v", err)
	}

	select {
	case <-q.Ready():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected Ready signal")
	}

	// Test Dequeue
	val, ok := q.TryDequeue()
	if !ok || val != 1 {
		t.Fatalf("Expected 1, got %v (ok=%v)", val, ok)
	}

	// Queue empty, Ready should not trigger immediately (though spurious is allowed, we expect silence here mostly)
	select {
	case <-q.Ready():
		// Spurious wakeup is technically allowed, but in single thread it shouldn't happen with our logic
	default:
	}
}

func TestNotifyQueue_GracefulDraining(t *testing.T) {
	q := NewNotifyQueue[int](10, 0)
	ctx := context.Background()
	// Fill queue
	for i := 0; i < 5; i++ {
		_ = q.EnqueueOrWait(ctx, i)
	}

	// Close queue
	q.Close()

	// Check Done channel
	select {
	case <-q.Done():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected Done signal")
	}

	// Drain queue using Ready signal
	count := 0
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

Loop:
	for {
		select {
		case <-q.Ready():
			// Try to dequeue
			if val, ok := q.TryDequeue(); ok {
				if val != count {
					t.Errorf("Expected value %d, got %d", count, val)
				}
				count++
			} else if q.IsClosed() && q.Size() == 0 {
				// Empty and closed
				break Loop
			}
		case <-q.Done():
			// Even if closed, we must check if there is data left
			if q.Size() > 0 {
				continue
			}
			break Loop
		case <-ctx.Done():
			t.Fatal("Timeout waiting for drain")
		}
	}

	if count != 5 {
		t.Errorf("Expected to drain 5 items, got %d", count)
	}
}

func TestNotifyQueue_Concurrent(t *testing.T) {
	ctx := context.Background()
	q := NewNotifyQueue[int](100, 0)
	itemCount := 1000
	workers := 10

	var wg sync.WaitGroup
	wg.Add(workers)

	// Producers
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < itemCount/workers; j++ {
				q.EnqueueOrWait(ctx, j)
			}
		}()
	}

	receivedCount := 0
	done := make(chan struct{})

	// Consumer
	go func() {
		defer close(done)
		for {
			select {
			case <-q.Ready():
				batch := make([]int, 10)
				n := q.TryDequeueBatchInto(batch)
				receivedCount += n
				if receivedCount == itemCount {
					return
				}
			case <-time.After(2 * time.Second):
				return // Timeout
			}
		}
	}()

	wg.Wait() // Wait for producers
	<-done    // Wait for consumer

	if receivedCount != itemCount {
		t.Errorf("Expected %d items, got %d", itemCount, receivedCount)
	}
}

func TestNotifyQueue_BlockingLimit(t *testing.T) {
	ctx := context.Background()
	q := NewNotifyQueue[int](2, 2) // Limit 2

	q.EnqueueOrWait(ctx, 1)
	q.EnqueueOrWait(ctx, 2)

	// Next enqueue should block or fail (TryEnqueue returns false)
	added, _ := q.TryEnqueue(3)
	if added {
		t.Fatal("Expected TryEnqueue to fail on full queue")
	}

	// EnqueueOrWait should block
	doneCh := make(chan struct{})
	go func() {
		q.EnqueueOrWait(ctx, 3)
		close(doneCh)
	}()

	select {
	case <-doneCh:
		t.Fatal("EnqueueOrWait should be blocking")
	case <-time.After(100 * time.Millisecond):
		// Expected
	}

	// Dequeue one to unblock
	q.TryDequeue()

	select {
	case <-doneCh:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("EnqueueOrWait should have unblocked")
	}
}

func TestNotifyQueue_DoubleClose(t *testing.T) {
	q := NewNotifyQueue[int](10, 0)
	q.Close()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Close panic: %v", r)
		}
	}()
	q.Close() // Should not panic
}

func TestNotifyQueue_LevelTriggered(t *testing.T) {
	q := NewNotifyQueue[int](10, 0)

	// 1. Enqueue -> Signal
	q.TryEnqueue(1)
	select {
	case <-q.Ready():
	default:
		t.Fatal("Should have signal after enqueue")
	}

	// 2. Enqueue again -> Signal should remain/be refillable
	q.TryEnqueue(2)

	// Consume signal if present (it might be consumed by previous check if not careful,
	// but here we just want to ensure that after operations, signal is maintained)
	select {
	case <-q.Ready():
	default:
		// It's possible the previous select consumed it.
		// But TryEnqueue(2) should have tried to refill it.
	}

	// 3. Partial Dequeue -> Signal re-arms
	// Queue has 2 items. We remove 1. Size becomes 1.
	// maintainSignals should put a signal in notEmpty.
	q.TryDequeue()

	select {
	case <-q.Ready():
	case <-time.After(10 * time.Millisecond):
		t.Fatal("Signal should be re-armed after partial dequeue")
	}

	// 4. Empty Dequeue -> Signal cleared (consumed)
	q.TryDequeue() // Size 1 -> 0.

	select {
	case <-q.Ready():
		t.Fatal("Signal should be empty when queue is empty")
	default:
	}
}

func TestNotifyQueue_ContextCancel(t *testing.T) {
	q := NewNotifyQueue[int](1, 1) // Limit 1
	q.TryEnqueue(1)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// EnqueueOrWait should fail immediately on full queue
	if err := q.EnqueueOrWait(ctx, 2); err == nil {
		t.Error("Expected error on cancelled context")
	} else if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got %v", err)
	}

	// DequeueOrWait on empty queue with cancel
	qEmpty := NewNotifyQueue[int](1, 1)
	if _, ok := qEmpty.DequeueOrWait(ctx); ok {
		t.Error("Expected false/error on cancelled context")
	}
}

func TestNotifyQueue_BatchOperations(t *testing.T) {
	q := NewNotifyQueue[int](10, 5) // Limit 5

	// Batch Enqueue
	if ok, _ := q.TryEnqueueBatch(1, 2, 3); !ok {
		t.Error("TryEnqueueBatch failed")
	}

	// Batch Enqueue Overflow
	if ok, _ := q.TryEnqueueBatch(4, 5, 6); ok {
		t.Error("TryEnqueueBatch should fail when exceeding limit")
	}

	// Dequeue Batch
	buf := make([]int, 2)
	n := q.TryDequeueBatchInto(buf)
	if n != 2 {
		t.Errorf("Expected 2, got %d", n)
	}
	if buf[0] != 1 || buf[1] != 2 {
		t.Errorf("Unexpected values: %v", buf)
	}

	// Remaining: 1 (started with 3, removed 2)
	if q.Size() != 1 {
		t.Errorf("Expected size 1, got %d", q.Size())
	}
}

func TestNotifyQueue_ClosedBehavior(t *testing.T) {
	q := NewNotifyQueue[int](10, 0)
	q.Close()

	if _, err := q.TryEnqueue(1); err != ErrQueueClosed {
		t.Errorf("Expected ErrQueueClosed, got %v", err)
	}

	if err := q.EnqueueOrWait(context.Background(), 1); err != ErrQueueClosed {
		t.Errorf("Expected ErrQueueClosed, got %v", err)
	}
}
