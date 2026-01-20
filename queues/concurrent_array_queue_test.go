package queues_test

import (
	"silo/queues"
	"sync"
	"testing"
)

func TestConcurrentQueue_BasicOperations(t *testing.T) {
	tests := []struct {
		name     string
		actions  func(t *testing.T, cq *queues.ConcurrentQueue[int])
	}{
		{
			name: "Enqueue and Dequeue",
			actions: func(t *testing.T, cq *queues.ConcurrentQueue[int]) {
				cq.Enqueue(1)
				cq.Enqueue(2)
				if cq.Size() != 2 {
					t.Errorf("expected size 2, got %d", cq.Size())
				}
				
				val, ok := cq.Dequeue()
				if !ok || val != 1 {
					t.Errorf("expected 1, got %v", val)
				}
				
				val, ok = cq.Dequeue()
				if !ok || val != 2 {
					t.Errorf("expected 2, got %v", val)
				}
				
				if !cq.IsEmpty() {
					t.Error("queue should be empty")
				}
			},
		},
		{
			name: "EnqueueAll and Peek",
			actions: func(t *testing.T, cq *queues.ConcurrentQueue[int]) {
				cq.EnqueueAll(10, 20, 30)
				if cq.Size() != 3 {
					t.Errorf("expected size 3, got %d", cq.Size())
				}
				
				val, ok := cq.Peek()
				if !ok || val != 10 {
					t.Errorf("Peek expected 10, got %v", val)
				}
				
				// Peek shouldn't remove item
				if cq.Size() != 3 {
					t.Error("Peek removed item")
				}
			},
		},
		{
			name: "Empty Queue Operations",
			actions: func(t *testing.T, cq *queues.ConcurrentQueue[int]) {
				if _, ok := cq.Dequeue(); ok {
					t.Error("Dequeue on empty queue should return false")
				}
				if _, ok := cq.Peek(); ok {
					t.Error("Peek on empty queue should return false")
				}
			},
		},
		{
			name: "Clear",
			actions: func(t *testing.T, cq *queues.ConcurrentQueue[int]) {
				cq.Enqueue(1)
				cq.Clear()
				if !cq.IsEmpty() {
					t.Error("queue should be empty after Clear")
				}
			},
		},
		{
			name: "ResizeToFit",
			actions: func(t *testing.T, cq *queues.ConcurrentQueue[int]) {
				cq.Enqueue(1)
				cq.ResizeToFit()
				if cq.Size() != 1 {
					t.Errorf("expected size 1 after resize, got %d", cq.Size())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cq := queues.NewConcurrentQueue[int](10)
			tt.actions(t, cq)
		})
	}
}

func TestConcurrentQueue_Concurrency(t *testing.T) {
	cq := queues.NewConcurrentQueue[int](100)
	const (
		producers = 10
		items     = 1000
	)

	var wg sync.WaitGroup
	wg.Add(producers)

	// Producers
	for i := 0; i < producers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < items; j++ {
				cq.Enqueue(j)
			}
		}()
	}

	wg.Wait()

	expectedSize := producers * items
	if cq.Size() != expectedSize {
		t.Errorf("expected size %d, got %d", expectedSize, cq.Size())
	}

	// Concurrent Consume
	wg.Add(producers) // reusing same count for consumers
	for i := 0; i < producers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < items; j++ {
				if _, ok := cq.Dequeue(); !ok {
					t.Error("unexpected empty queue during consumption")
				}
			}
		}()
	}
	
	wg.Wait()
	
	if !cq.IsEmpty() {
		t.Errorf("queue not empty, size: %d", cq.Size())
	}
}
