package queues_test

import (
	"silo/queues"
	"testing"
)

// Simple Task struct for testing
type Task struct {
	Name     string
	Priority int
}

func taskPriority(t Task) int64 {
	return int64(t.Priority)
}

func TestNewPriorityQueue_Validation(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewPriorityQueue should panic with nil extractor")
		}
	}()
	queues.NewPriorityQueue[int](10, true, nil)
}

func TestPriorityQueue_Ordering(t *testing.T) {
	tests := []struct {
		name      string
		isMinHeap bool
		inputs    []Task
		expected  []string // Expected Names in order
	}{
		{
			name:      "MinHeap Integers",
			isMinHeap: true,
			inputs: []Task{
				{"A", 3}, {"B", 1}, {"C", 4}, {"D", 2},
			},
			expected: []string{"B", "D", "A", "C"}, // 1, 2, 3, 4
		},
		{
			name:      "MaxHeap Integers",
			isMinHeap: false,
			inputs: []Task{
				{"A", 3}, {"B", 1}, {"C", 4}, {"D", 2},
			},
			expected: []string{"C", "A", "D", "B"}, // 4, 3, 2, 1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pq := queues.NewPriorityQueue[Task](10, tt.isMinHeap, taskPriority)

			for _, task := range tt.inputs {
				pq.Enqueue(task)
			}

			if pq.Size() != len(tt.inputs) {
				t.Errorf("expected size %d, got %d", len(tt.inputs), pq.Size())
			}

			for _, expName := range tt.expected {
				val, ok := pq.Dequeue()
				if !ok {
					t.Errorf("expected dequeue %s, got nothing", expName)
				}
				if val.Name != expName {
					t.Errorf("expected name %s, got %s (prio %d)", expName, val.Name, val.Priority)
				}
			}

			if !pq.IsEmpty() {
				t.Error("queue should be empty")
			}
		})
	}
}

func TestPriorityQueue_ItemOperations(t *testing.T) {
	pq := queues.NewPriorityQueue[Task](10, true, taskPriority) // MinHeap

	_ = pq.Enqueue(Task{"A", 10})
	itemB := pq.Enqueue(Task{"B", 20})
	itemC := pq.Enqueue(Task{"C", 30})

	// Current: A(10), B(20), C(30)

	// Test Remove: Remove B(20)
	pq.RemoveItem(itemB)

	if pq.Size() != 2 {
		t.Errorf("expected size 2 after remove, got %d", pq.Size())
	}

	// Test Update: Change C(30) to C(5) -> Should become new head
	// Note: In strict Go struct semantics, itemC.Value is a copy if not pointer.
	// But PriorityItem holds Value T. We update itemC.Value then call UpdateItem.
	// The implementation calls PriorityExtractor(item.Value).
	itemC.Value.Priority = 5
	pq.UpdateItem(itemC)

	// Expected order: C(5), A(10)

	val, _ := pq.Dequeue()
	if val.Name != "C" || val.Priority != 5 {
		t.Errorf("expected head C(5), got %s(%d)", val.Name, val.Priority)
	}

	val, _ = pq.Dequeue()
	if val.Name != "A" {
		t.Errorf("expected next A, got %s", val.Name)
	}
}

func TestPriorityQueue_InvalidOperations(t *testing.T) {
	pq := queues.NewPriorityQueue[int](10, true, func(i int) int64 { return int64(i) })
	item := pq.Enqueue(1)
	pq.Dequeue() // Queue is empty, item is removed from heap

	// Test Update on removed item
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("UpdateItem should panic on removed item")
			}
		}()
		pq.UpdateItem(item)
	}()

	// Test Remove on removed item
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("RemoveItem should panic on removed item")
			}
		}()
		pq.RemoveItem(item)
	}()
}

func TestPriorityQueue_PeekAndEmpty(t *testing.T) {
	pq := queues.NewPriorityQueue[int](10, true, func(i int) int64 { return int64(i) })

	if _, ok := pq.Peek(); ok {
		t.Error("Peek on empty should be false")
	}

	pq.Enqueue(100)
	if val, ok := pq.Peek(); !ok || val != 100 {
		t.Errorf("Peek expected 100, got %v", val)
	}

	// Peek shouldn't remove
	if pq.Size() != 1 {
		t.Error("Peek removed item")
	}
}
