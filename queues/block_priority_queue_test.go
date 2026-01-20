package queues_test

import (
	"silo/queues"
	"sync"
	"testing"
	"time"
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
		expected  []int // Expected dequeue order
	}{
		{
			name:      "MinHeap Unbounded",
			isMinHeap: true,
			limit:     0,
			inputs:    []int{3, 1, 4, 2},
			expected:  []int{1, 2, 3, 4},
		},
		{
			name:      "MaxHeap Unbounded",
			isMinHeap: false,
			limit:     0,
			inputs:    []int{3, 1, 4, 2},
			expected:  []int{4, 3, 2, 1},
		},
		{
			name:      "MinHeap Bounded (Not Full)",
			isMinHeap: true,
			limit:     10,
			inputs:    []int{10, 5},
			expected:  []int{5, 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bpq := queues.NewBlockingPriorityQueue[int](10, tt.isMinHeap, intExtractor, tt.limit)

			// Test TryEnqueue
			for _, v := range tt.inputs {
				item, ok := bpq.TryEnqueue(v)
				if !ok {
					t.Errorf("TryEnqueue failed for value %d", v)
				}
				if item == nil {
					t.Error("TryEnqueue returned nil item")
				}
			}

			if bpq.Len() != len(tt.inputs) {
				t.Errorf("expected len %d, got %d", len(tt.inputs), bpq.Len())
			}

			// Test Order
			for _, exp := range tt.expected {
				val, ok := bpq.TryDequeue()
				if !ok {
					t.Errorf("TryDequeue failed, expected %d", exp)
				}
				if val != exp {
					t.Errorf("expected dequeue value %d, got %d", exp, val)
				}
			}

			if bpq.Len() != 0 {
				t.Error("queue should be empty")
			}
		})
	}
}

func TestBlockingPriorityQueue_BoundedFull(t *testing.T) {
	// MinHeap, Limit 2
	bpq := queues.NewBlockingPriorityQueue[int](2, true, intExtractor, 2)

	bpq.TryEnqueue(10)
	bpq.TryEnqueue(20)

	// Queue is full
	if _, ok := bpq.TryEnqueue(30); ok {
		t.Error("TryEnqueue should fail when queue is full")
	}

	if bpq.Len() != 2 {
		t.Errorf("expected len 2, got %d", bpq.Len())
	}
}

func TestBlockingPriorityQueue_Empty(t *testing.T) {
	bpq := queues.NewBlockingPriorityQueue[int](5, true, intExtractor, 0)

	if _, ok := bpq.TryDequeue(); ok {
		t.Error("TryDequeue should fail on empty queue")
	}
}

func TestBlockingPriorityQueue_BlockingDequeue(t *testing.T) {
	bpq := queues.NewBlockingPriorityQueue[int](10, true, intExtractor, 0)
	done := make(chan int)

	go func() {
		val, _ := bpq.DequeueOrWait()
		done <- val
	}()

	time.Sleep(50 * time.Millisecond) // Ensure goroutine waits

	bpq.EnqueueOrWait(42)

	select {
	case val := <-done:
		if val != 42 {
			t.Errorf("expected 42, got %d", val)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for DequeueOrWait")
	}
}

func TestBlockingPriorityQueue_BlockingEnqueue(t *testing.T) {
	// Limit 1
	bpq := queues.NewBlockingPriorityQueue[int](1, true, intExtractor, 1)
	bpq.EnqueueOrWait(100)

	done := make(chan struct{})
	go func() {
		bpq.EnqueueOrWait(200)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond) // Ensure blocked

	select {
	case <-done:
		t.Error("EnqueueOrWait should have blocked")
	default:
		// OK
	}

	// Dequeue to make space
	val, ok := bpq.TryDequeue()
	if !ok || val != 100 {
		t.Errorf("expected dequeue 100, got %v", val)
	}

	select {
	case <-done:
		// OK, unblocked
		// Check that 200 is now in queue
		if bpq.Len() != 1 {
			t.Errorf("expected len 1, got %d", bpq.Len())
		}
		val, _ := bpq.TryDequeue()
		if val != 200 {
			t.Errorf("expected 200, got %d", val)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for EnqueueOrWait to unblock")
	}
}

func TestBlockingPriorityQueue_Concurrency(t *testing.T) {
	// MinHeap
	bpq := queues.NewBlockingPriorityQueue[int](100, true, intExtractor, 50)
	
	const count = 100
	var wg sync.WaitGroup
	wg.Add(2)

	// Producer
	go func() {
		defer wg.Done()
		for i := count; i > 0; i-- {
			bpq.EnqueueOrWait(i)
		}
	}()

	// Consumer
	received := make([]int, 0, count)
	go func() {
		defer wg.Done()
		for i := 0; i < count; i++ {
			val, _ := bpq.DequeueOrWait()
			received = append(received, val)
		}
	}()

	wg.Wait()

	if len(received) != count {
		t.Errorf("expected %d items, got %d", count, len(received))
	}
	
	// Since producer and consumer run concurrently, strict global order isn't guaranteed 
	// (consumer might pick up 100, then 99, while 98 hasn't been enqueued yet).
	// But we can check basic integrity or if we paused producer it would be sorted.
	// For this test, just ensuring no deadlock and data loss is sufficient.
	
	if bpq.Len() != 0 {
		t.Errorf("queue should be empty, got %d", bpq.Len())
	}
}
