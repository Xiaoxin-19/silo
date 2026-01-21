package queues

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNotifyQueue_Basic(t *testing.T) {
	q := NewNotifyQueue[int](10, 0) // Unbounded

	// Test Enqueue and Signal
	if err := q.EnqueueOrWait(1); err != nil {
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
		// t.Log("Spurious wakeup detected (acceptable)")
	default:
	}
}

func TestNotifyQueue_GracefulDraining(t *testing.T) {
	q := NewNotifyQueue[int](10, 0)

	// Fill queue
	for i := 0; i < 5; i++ {
		_ = q.EnqueueOrWait(i)
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
				q.EnqueueOrWait(j)
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
	q := NewNotifyQueue[int](2, 2) // Limit 2

	q.EnqueueOrWait(1)
	q.EnqueueOrWait(2)

	// Next enqueue should block or fail (TryEnqueue returns false)
	added, _ := q.TryEnqueue(3)
	if added {
		t.Fatal("Expected TryEnqueue to fail on full queue")
	}

	// EnqueueOrWait should block
	doneCh := make(chan struct{})
	go func() {
		q.EnqueueOrWait(3)
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
