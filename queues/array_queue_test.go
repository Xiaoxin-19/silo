package queues_test

import (
	"silo/queues"
	"testing"
)

func TestNewArrayQueue(t *testing.T) {
	tests := []struct {
		name             string
		initialCapacity  int
	}{
		{"Negative capacity", -1},
		{"Zero capacity", 0},
		{"Capacity 1", 1},
		{"Capacity 2", 2},
		{"Capacity 3 (round up)", 3},
		{"Capacity 8", 8},
		{"Capacity 9 (round up)", 9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := queues.NewArrayQueue[int](tt.initialCapacity)
			// Cannot check internal capacity in black-box test
			if q.Size() != 0 {
				t.Errorf("expected size 0, got %d", q.Size())
			}
			if !q.IsEmpty() {
				t.Error("expected queue to be empty")
			}
		})
	}
}

func TestArrayQueue_Enqueue_Dequeue(t *testing.T) {
	q := queues.NewArrayQueue[int](4)

	// Test Enqueue and wrap-around
	// Fill: [1, 2, 3, 4]
	for i := 1; i <= 4; i++ {
		q.Enqueue(i)
	}

	if q.Size() != 4 {
		t.Errorf("expected size 4, got %d", q.Size())
	}
	
	// Dequeue 2 items: [_, _, 3, 4] (head at index 2)
	if v, ok := q.Dequeue(); !ok || v != 1 {
		t.Errorf("expected 1, got %v", v)
	}
	if v, ok := q.Dequeue(); !ok || v != 2 {
		t.Errorf("expected 2, got %v", v)
	}

	// Enqueue causing wrap-around: [5, 6, 3, 4]
	q.Enqueue(5)
	q.Enqueue(6)

	if q.Size() != 4 {
		t.Errorf("expected size 4, got %d", q.Size())
	}
	
	// Verify order with Peek and Dequeue
	if v, ok := q.Peek(); !ok || v != 3 {
		t.Errorf("Peek expected 3, got %v", v)
	}

	// Trigger resize (doubling) from wrap-around state
	// Current: [5, 6, 3, 4] (head=2), Add 7 -> should resize to 8 and unwrap
	q.Enqueue(7)

	// Cannot check internal buffer length directly
	if q.Size() != 5 {
		t.Errorf("expected size 5, got %d", q.Size())
	}

	// Verify all elements after resize
	expected := []int{3, 4, 5, 6, 7}
	for _, exp := range expected {
		if v, ok := q.Dequeue(); !ok || v != exp {
			t.Errorf("expected %d, got %v (ok=%v)", exp, v, ok)
		}
	}

	if !q.IsEmpty() {
		t.Error("queue should be empty")
	}
}

func TestArrayQueue_EnqueueAll(t *testing.T) {
	q := queues.NewArrayQueue[int](4)

	// EnqueueAll that fits
	q.EnqueueAll(1, 2)
	if q.Size() != 2 {
		t.Errorf("expected size 2, got %d", q.Size())
	}

	// EnqueueAll triggering resize
	q.EnqueueAll(3, 4, 5) // Total 5 items
	if q.Size() != 5 {
		t.Errorf("expected size 5, got %d", q.Size())
	}

	// Drain
	q.Clear()
	
	// Test wrap-around copy in EnqueueAll
	// Setup: cap 4, [_, _, 1, 2] (head=2)
	q = queues.NewArrayQueue[int](4)
	q.Enqueue(1)
	q.Enqueue(2)
	q.Dequeue() // remove 1
	q.Dequeue() // remove 2
	q.Clear() 
	
	// Manually construct wrap scenario
	q.Enqueue(100)
	q.Enqueue(200)
	q.Dequeue() // head moves to index 1
	
	// Add 3 items: 300, 400, 500. 
	q.EnqueueAll(300, 400, 500)
	
	if q.Size() != 4 {
		t.Errorf("expected size 4, got %d", q.Size())
	}
	
	expected := []int{200, 300, 400, 500}
	for _, exp := range expected {
		if v, ok := q.Dequeue(); !ok || v != exp {
			t.Errorf("expected %d, got %v", exp, v)
		}
	}
}

func TestArrayQueue_EmptyOperations(t *testing.T) {
	q := queues.NewArrayQueue[string](10)

	if _, ok := q.Dequeue(); ok {
		t.Error("Dequeue on empty queue should return false")
	}

	if _, ok := q.Peek(); ok {
		t.Error("Peek on empty queue should return false")
	}

	q.ResizeToFit() // Should not panic
}

func TestArrayQueue_Clear(t *testing.T) {
	q := queues.NewArrayQueue[int](8)
	q.EnqueueAll(1, 2, 3)
	q.Clear()

	if q.Size() != 0 {
		t.Errorf("expected size 0 after clear, got %d", q.Size())
	}
	if !q.IsEmpty() {
		t.Error("expected IsEmpty true after clear")
	}
	
	// Cannot check internal buffer zeroing directly
}

func TestArrayQueue_ResizeToFit(t *testing.T) {
	q := queues.NewArrayQueue[int](16)
	q.Enqueue(1)
	q.Enqueue(2)
	q.Enqueue(3)

	// Current size 3. Next power of 2 is 4.
	q.ResizeToFit()

	// Cannot check capacity directly
	if q.Size() != 3 {
		t.Errorf("expected size 3, got %d", q.Size())
	}

	// Verify content integrity
	expected := []int{1, 2, 3}
	for _, exp := range expected {
		val, ok := q.Dequeue()
		if !ok || val != exp {
			t.Errorf("expected %d, got %v", exp, val)
		}
	}
}

func TestArrayQueue_WrapAroundResize(t *testing.T) {
	q := queues.NewArrayQueue[int](4)
	// [1, 2, 3, 4]
	q.EnqueueAll(1, 2, 3, 4)
	q.Dequeue() // remove 1
	q.Dequeue() // remove 2
	// [_, _, 3, 4] head=2
	q.Enqueue(5) 
	q.Enqueue(6)
	// [5, 6, 3, 4] head=2 (wrapped)
	
	q.Enqueue(7) 
	// Trigger resize to 8. Should unwrap.
	
	// Cannot check capacity directly
	
	expected := []int{3, 4, 5, 6, 7}
	for i, exp := range expected {
		val, ok := q.Dequeue()
		if !ok || val != exp {
			t.Errorf("step %d: expected %d, got %v", i, exp, val)
		}
	}
}
