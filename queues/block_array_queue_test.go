package queues_test

import (
	"silo/queues"
	"sync"
	"testing"
	"time"
)

func TestBlockingQueue_BasicOperations(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
		limit    int
		actions  func(t *testing.T, bq *queues.BlockingQueue[int])
	}{
		{
			name:     "Unbounded Queue Operations",
			capacity: 10,
			limit:    0,
			actions: func(t *testing.T, bq *queues.BlockingQueue[int]) {
				if !bq.IsEmpty() {
					t.Error("new queue should be empty")
				}
				if bq.Size() != 0 {
					t.Errorf("expected size 0, got %d", bq.Size())
				}

				// TryEnqueue
				if ok, _ := bq.TryEnqueue(1); !ok {
					t.Error("TryEnqueue should succeed on unbounded queue")
				}
				bq.EnqueueOrWait(2)

				if bq.Size() != 2 {
					t.Errorf("expected size 2, got %d", bq.Size())
				}

				// TryDequeue
				val, ok := bq.TryDequeue()
				if !ok || val != 1 {
					t.Errorf("expected dequeue 1, got %v (ok=%v)", val, ok)
				}

				// DequeueOrWait (should not block as data exists)
				val, ok = bq.DequeueOrWait()
				if !ok || val != 2 {
					t.Errorf("expected dequeue 2, got %v (ok=%v)", val, ok)
				}

				if !bq.IsEmpty() {
					t.Error("queue should be empty after draining")
				}
			},
		},
		{
			name:     "Bounded Queue Full",
			capacity: 2,
			limit:    2,
			actions: func(t *testing.T, bq *queues.BlockingQueue[int]) {
				_, _ = bq.TryEnqueue(1)
				_, _ = bq.TryEnqueue(2)

				// Queue is full
				if ok, _ := bq.TryEnqueue(3); ok {
					t.Error("TryEnqueue should fail when bounded queue is full")
				}

				if bq.Size() != 2 {
					t.Errorf("expected size 2, got %d", bq.Size())
				}
			},
		},
		{
			name:     "Empty Queue Dequeue",
			capacity: 5,
			limit:    0,
			actions: func(t *testing.T, bq *queues.BlockingQueue[int]) {
				_, ok := bq.TryDequeue()
				if ok {
					t.Error("TryDequeue should fail on empty queue")
				}
			},
		},
		{
			name:     "Clear Operation",
			capacity: 5,
			limit:    0,
			actions: func(t *testing.T, bq *queues.BlockingQueue[int]) {
				_, _ = bq.TryEnqueue(1)
				_, _ = bq.TryEnqueue(2)
				bq.Clear()
				if !bq.IsEmpty() {
					t.Error("queue should be empty after Clear")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bq := queues.NewBlockingQueue[int](tt.capacity, tt.limit)
			tt.actions(t, bq)
		})
	}
}

func TestBlockingQueue_BlockingDequeue(t *testing.T) {
	bq := queues.NewBlockingQueue[int](10, 0)

	// Channel to signal completion of dequeue
	done := make(chan int)

	go func() {
		// This should block until main thread enqueues
		val, _ := bq.DequeueOrWait()
		done <- val
	}()

	// Ensure goroutine has likely started and blocked
	time.Sleep(50 * time.Millisecond)

	// Enqueue value
	bq.EnqueueOrWait(100)

	select {
	case val := <-done:
		if val != 100 {
			t.Errorf("expected dequeued value 100, got %d", val)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for DequeueOrWait to unblock")
	}
}

func TestBlockingQueue_BlockingEnqueue(t *testing.T) {
	// Bounded queue with limit 1
	bq := queues.NewBlockingQueue[int](1, 1)

	// Fill the queue
	bq.EnqueueOrWait(1)

	done := make(chan bool)

	go func() {
		// This should block because limit is 1
		bq.EnqueueOrWait(2)
		done <- true
	}()

	// Ensure goroutine has likely blocked
	time.Sleep(50 * time.Millisecond)

	select {
	case <-done:
		t.Error("EnqueueOrWait should have blocked")
	default:
		// Expected behavior
	}

	// Make space
	_, _ = bq.TryDequeue()

	select {
	case <-done:
		// Success, unblocked
		if bq.Size() != 1 {
			t.Errorf("expected size 1, got %d", bq.Size())
		}
		val, _ := bq.TryDequeue()
		if val != 2 {
			t.Errorf("expected value 2, got %d", val)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for EnqueueOrWait to unblock")
	}
}

func TestBlockingQueue_Concurrency(t *testing.T) {
	// Many producers, many consumers
	bq := queues.NewBlockingQueue[int](100, 50) // limit 50
	const (
		producers = 10
		consumers = 10
		items     = 100
	)

	var wg sync.WaitGroup
	wg.Add(producers + consumers)

	// Consumers
	consumedCounts := make([]int, consumers)
	for i := 0; i < consumers; i++ {
		go func(id int) {
			defer wg.Done()
			count := 0
			for j := 0; j < items; j++ {
				_, ok := bq.DequeueOrWait()
				if ok {
					count++
				}
			}
			consumedCounts[id] = count
		}(i)
	}

	// Producers
	for i := 0; i < producers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < items; j++ {
				bq.EnqueueOrWait(j)
			}
		}()
	}

	wg.Wait()

	if !bq.IsEmpty() {
		t.Errorf("expected empty queue, got size %d", bq.Size())
	}
}

func TestBlockingQueue_ResizeToFit(t *testing.T) {
	bq := queues.NewBlockingQueue[int](16, 0)
	_, _ = bq.TryEnqueue(1)
	_, _ = bq.TryEnqueue(2)

	// Just verify it doesn't deadlock or panic
	bq.ResizeToFit()

	if bq.Size() != 2 {
		t.Errorf("expected size 2, got %d", bq.Size())
	}
}
