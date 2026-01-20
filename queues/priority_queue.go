package queues

import (
	"container/heap"
)

type PriorityItem[T any] struct {
	Value    T
	index    int
	priority int64
}

type internalHeap[T any] struct {
	data              []*PriorityItem[T]
	isMinHeap         bool
	PriorityExtractor func(T) int64
}

func (ih *internalHeap[T]) Len() int {
	return len(ih.data)
}

func (ih *internalHeap[T]) Less(i, j int) bool {
	switch {
	case ih.isMinHeap:
		return ih.data[i].priority < ih.data[j].priority
	default:
		return ih.data[i].priority > ih.data[j].priority
	}
}

func (ih *internalHeap[T]) Swap(i, j int) {
	ih.data[i], ih.data[j] = ih.data[j], ih.data[i]
	ih.data[i].index = i
	ih.data[j].index = j
}

func (ih *internalHeap[T]) Push(x any) {
	idx := len(ih.data)
	item := x.(*PriorityItem[T])
	ih.data = append(ih.data, item)
	ih.data[idx].index = idx
}

func (ih *internalHeap[T]) Pop() any {
	old := ih.data
	n := len(old)
	lastItem := old[n-1]

	// avoid memory leak
	var zero *PriorityItem[T]
	lastItem.index = -1
	old[n-1] = zero

	// shrink slice
	ih.data = old[0 : n-1]
	return lastItem
}

type PriorityQueue[T any] struct {
	heap *internalHeap[T]
}

// NewPriorityQueue creates a new PriorityQueue with the specified initial capacity and heap type.
// If isMinHeap is true, it creates a min-heap; otherwise, it creates a max-heap.
// PriorityExtractor is a function that extracts the priority value from an element of type T.
func NewPriorityQueue[T any](initCapacity int, isMinHeap bool, PriorityExtractor func(T) int64) *PriorityQueue[T] {
	if initCapacity < 0 {
		initCapacity = 0
	}
	if PriorityExtractor == nil {
		panic("silo.PriorityQueue: PriorityExtractor function cannot be nil")
	}
	innerHeap := internalHeap[T]{
		data:              make([]*PriorityItem[T], 0, initCapacity),
		isMinHeap:         isMinHeap,
		PriorityExtractor: PriorityExtractor,
	}
	heap.Init(&innerHeap)

	return &PriorityQueue[T]{
		heap: &innerHeap,
	}
}

func (pq *PriorityQueue[T]) Enqueue(value T) *PriorityItem[T] {
	item := PriorityItem[T]{
		Value:    value,
		priority: pq.heap.PriorityExtractor(value),
	}
	heap.Push(pq.heap, &item)
	return &item
}

func (pq *PriorityQueue[T]) Dequeue() (value T, ok bool) {
	if pq.heap.Len() == 0 {
		return value, false
	}
	return heap.Pop(pq.heap).(*PriorityItem[T]).Value, true
}

func (pq *PriorityQueue[T]) Peek() (value T, ok bool) {
	if pq.heap.Len() == 0 {
		return value, false
	}
	return pq.heap.data[0].Value, true
}

func (pq *PriorityQueue[T]) UpdateItem(item *PriorityItem[T]) {
	if item.index < 0 || item.index >= pq.heap.Len() {
		panic("silo.PriorityQueue: UpdateItem called with invalid PriorityItem")
	}
	item.priority = pq.heap.PriorityExtractor(item.Value)
	heap.Fix(pq.heap, item.index)
}

func (pq *PriorityQueue[T]) RemoveItem(item *PriorityItem[T]) {
	if item.index < 0 || item.index >= pq.heap.Len() {
		panic("silo.PriorityQueue: RemoveItem called with invalid PriorityItem")
	}
	heap.Remove(pq.heap, item.index)
}

func (pq *PriorityQueue[T]) Size() int {
	return pq.heap.Len()
}

func (pq *PriorityQueue[T]) IsEmpty() bool {
	return pq.heap.Len() == 0
}
