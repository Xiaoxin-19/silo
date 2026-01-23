package queues_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"silo/queues"
)

// intExtractor treats the integer value itself as the priority
func intExtractor(val int) int64 {
	return int64(val)
}

func TestBlockingPriorityQueue_BasicOperations(t *testing.T) {
	tests := []struct {
		name      string
		isMinHeap bool
		limit     int
		inputs    []int
		expected  []int
	}{
		{"MinHeap Unbounded", true, 0, []int{3, 1, 4, 2}, []int{1, 2, 3, 4}},
		{"MaxHeap Unbounded", false, 0, []int{3, 1, 4, 2}, []int{4, 3, 2, 1}},
		{"MinHeap Bounded", true, 10, []int{10, 5}, []int{5, 10}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bpq := queues.NewBlockingPriorityQueue[int](10, tt.isMinHeap, intExtractor, tt.limit)

			if bpq.Size() != 0 {
				t.Errorf("expected size 0, got %d", bpq.Size())
			}

			for _, v := range tt.inputs {
				bpq.TryEnqueue(v)
			}

			if bpq.Size() != len(tt.inputs) {
				t.Errorf("expected size %d, got %d", len(tt.inputs), bpq.Size())
			}

			for _, exp := range tt.expected {
				val, ok := bpq.TryDequeue()
				if !ok || val != exp {
					t.Errorf("want %d, got %d (ok=%v)", exp, val, ok)
				}
			}

			if bpq.Size() != 0 {
				t.Errorf("expected size 0, got %d", bpq.Size())
			}
		})
	}
}

func TestBlockingPriorityQueue_DequeueOrWait_ContextCancel(t *testing.T) {
	bpq := queues.NewBlockingPriorityQueue[int](10, true, intExtractor, 0)

	done := make(chan struct{})

	go func() {
		_, ok := bpq.DequeueOrWait(context.Background())
		if ok {
			t.Error("expected false (closed), got true")
		}
		close(done)
	}()

	// Ensure goroutine is scheduled and blocked
	time.Sleep(10 * time.Millisecond)

	// Action: Cancel/Close the queue
	bpq.Close()

	select {
	case <-done:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("DequeueOrWait did not return after Close()")
	}
}

func TestBlockingPriorityQueue_BlockingEnqueue_Resume(t *testing.T) {
	// Limit = 1
	bpq := queues.NewBlockingPriorityQueue[int](1, true, intExtractor, 1)

	// Fill the queue
	bpq.TryEnqueue(10)

	// Verify full
	if _, ok := bpq.TryEnqueue(20); ok {
		t.Fatal("queue should be full")
	}

	enqueued := make(chan int)

	// Start a blocking producer
	go func() {
		bpq.EnqueueOrWait(context.Background(), 99)
		enqueued <- 99
	}()

	// Give it a tiny bit of time to hit the block
	select {
	case <-enqueued:
		t.Fatal("EnqueueOrWait returned immediately, expected block")
	case <-time.After(10 * time.Millisecond):
		// Expected to timeout here (blocked)
	}

	// Unblock: Dequeue one item
	val, ok := bpq.TryDequeue()
	if !ok || val != 10 {
		t.Fatalf("TryDequeue failed, got %v, %v", val, ok)
	}

	// Verify producer unblocks
	select {
	case v := <-enqueued:
		if v != 99 {
			t.Errorf("got %d, want 99", v)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("producer did not unblock after dequeue")
	}
}

func TestBlockingPriorityQueue_Concurrency_Stress(t *testing.T) {
	// Capacity 10, but we shove 10,000 items through it
	limit := 10
	count := 10000
	bpq := queues.NewBlockingPriorityQueue[int](100, true, intExtractor, limit)

	var wg sync.WaitGroup
	wg.Add(2)

	// Producer
	go func() {
		defer wg.Done()
		for i := 0; i < count; i++ {
			bpq.EnqueueOrWait(context.Background(), i)
		}
	}()

	// Consumer
	receivedCount := 0
	go func() {
		defer wg.Done()
		for i := 0; i < count; i++ {
			val, ok := bpq.DequeueOrWait(context.Background())
			if !ok {
				t.Error("unexpected closed queue")
				return
			}
			_ = val
			receivedCount++
		}
	}()

	// Monitor for Deadlock
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if receivedCount != count {
			t.Errorf("lost data: sent %d, recv %d", count, receivedCount)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("DEADLOCK DETECTED: Concurrency test timed out")
	}
}

func TestBlockingPriorityQueue_ReadySignaling(t *testing.T) {
	bpq := queues.NewBlockingPriorityQueue[int](10, true, intExtractor, 0)

	// 1. Initially Empty -> Ready() should block
	select {
	case <-bpq.Ready():
		t.Fatal("Ready() should block when empty")
	default:
		// OK
	}

	// 2. Enqueue -> Ready() should signal
	bpq.TryEnqueue(1)

	// Verify signal exists (Check 1)
	select {
	case <-bpq.Ready():
		// OK, we consumed the signal here!
	case <-time.After(10 * time.Millisecond):
		t.Fatal("Ready() failed to signal after enqueue")
	}

	// 2b. Enqueue ANOTHER item. Size = 1 -> 2.

	bpq.TryEnqueue(2)
	// Now size = 2. Signal should be full.

	// Check 2: Signal is present
	select {
	case <-bpq.Ready():
		// OK, consumed.
	default:
		t.Fatal("Signal missing for second item")
	}

	// 3. Dequeue ONE item. Size 2 -> 1.
	// This operation MUST re-arm the signal because Size > 0.
	bpq.TryDequeue()

	// Check 3: Signal should be re-armed (Level Triggered!)
	select {
	case <-bpq.Ready():
		// OK! This proves maintainSignals() works.
	default:
		t.Fatal("Signal lost! Dequeue() should have re-armed signal because queue is not empty")
	}

	// 4. Dequeue last item. Size 1 -> 0.
	bpq.TryDequeue()

	// Check 4: Should block (Edge Triggered OFF)
	select {
	case <-bpq.Ready():
		t.Fatal("Ready() should block after emptying queue")
	default:
		// OK
	}
}
