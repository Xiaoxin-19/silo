package queues_test

import (
	"silo/queues"
	"sync"
	"testing"
)

func intExtractorCPQ(val int) int64 {
	return int64(val)
}

func TestConcurrentPriorityQueue_BasicOperations(t *testing.T) {
	tests := []struct {
		name      string
		isMinHeap bool
		inputs    []int
		expected  []int
	}{
		{
			name:      "MinHeap Order",
			isMinHeap: true,
			inputs:    []int{5, 1, 3, 2, 4},
			expected:  []int{1, 2, 3, 4, 5},
		},
		{
			name:      "MaxHeap Order",
			isMinHeap: false,
			inputs:    []int{5, 1, 3, 2, 4},
			expected:  []int{5, 4, 3, 2, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpq := queues.NewConcurrentPriorityQueue[int](10, tt.isMinHeap, intExtractorCPQ)

			if !cpq.IsEmpty() {
				t.Error("new queue should be empty")
			}

			// Enqueue
			for _, v := range tt.inputs {
				item := cpq.Enqueue(v)
				if item == nil {
					t.Error("Enqueue returned nil item")
				}
			}

			if cpq.Size() != len(tt.inputs) {
				t.Errorf("expected size %d, got %d", len(tt.inputs), cpq.Size())
			}

			// Peek
			val, ok := cpq.Peek()
			if !ok || val != tt.expected[0] {
				t.Errorf("Peek expected %d, got %v", tt.expected[0], val)
			}

			// Dequeue and Verify Order
			for _, exp := range tt.expected {
				val, ok := cpq.Dequeue()
				if !ok {
					t.Errorf("Dequeue failed, expected %d", exp)
				}
				if val != exp {
					t.Errorf("expected %d, got %d", exp, val)
				}
			}

			if !cpq.IsEmpty() {
				t.Error("queue should be empty after draining")
			}
		})
	}
}

func TestConcurrentPriorityQueue_ItemOperations(t *testing.T) {
	// Test UpdateItem and RemoveItem via thread-safe wrapper

	// MinHeap
	cpq := queues.NewConcurrentPriorityQueue[int](10, true, intExtractorCPQ)

	item1 := cpq.Enqueue(10)
	item2 := cpq.Enqueue(20)
	cpq.Enqueue(30)

	// Remove item2 (20)
	cpq.RemoveItem(item2)

	if cpq.Size() != 2 {
		t.Errorf("expected size 2 after removal, got %d", cpq.Size())
	}

	// Update item1 (10 -> 40), now order should be 30, 40
	item1.Value = 40
	cpq.UpdateItem(item1)

	val, _ := cpq.Dequeue()
	if val != 30 {
		t.Errorf("expected 30, got %d", val)
	}

	val, _ = cpq.Dequeue()
	if val != 40 {
		t.Errorf("expected 40, got %d", val)
	}
}

func TestConcurrentPriorityQueue_Empty(t *testing.T) {
	cpq := queues.NewConcurrentPriorityQueue[int](10, true, intExtractorCPQ)

	if _, ok := cpq.Dequeue(); ok {
		t.Error("Dequeue on empty queue should return false")
	}
	if _, ok := cpq.Peek(); ok {
		t.Error("Peek on empty queue should return false")
	}
}

func TestConcurrentPriorityQueue_Concurrency(t *testing.T) {
	// Simple concurrent producer-consumer test
	cpq := queues.NewConcurrentPriorityQueue[int](100, true, intExtractorCPQ)
	const count = 1000
	var wg sync.WaitGroup
	wg.Add(2)

	// Producer
	go func() {
		defer wg.Done()
		for i := 0; i < count; i++ {
			cpq.Enqueue(i)
		}
	}()

	// Consumer
	consumedCount := 0
	go func() {
		defer wg.Done()
		for consumedCount < count {
			if _, ok := cpq.Dequeue(); ok {
				consumedCount++
			}
		}
	}()

	wg.Wait()

	if !cpq.IsEmpty() {
		t.Errorf("queue not empty, size: %d", cpq.Size())
	}
}
