package queues

import (
	"sync"
)

type ConcurrentPriorityQueue[T any] struct {
	mu sync.Mutex
	pq *PriorityQueue[T]
}

func NewConcurrentPriorityQueue[T any](initCapacity int, isMinHeap bool, PriorityExtractor func(T) int64) *ConcurrentPriorityQueue[T] {
	return &ConcurrentPriorityQueue[T]{
		pq: NewPriorityQueue(initCapacity, isMinHeap, PriorityExtractor),
	}
}

func (cpq *ConcurrentPriorityQueue[T]) Enqueue(value T) *PriorityItem[T] {
	cpq.mu.Lock()
	defer cpq.mu.Unlock()
	return cpq.pq.Enqueue(value)
}

func (cpq *ConcurrentPriorityQueue[T]) Dequeue() (value T, ok bool) {
	cpq.mu.Lock()
	defer cpq.mu.Unlock()
	return cpq.pq.Dequeue()
}

func (cpq *ConcurrentPriorityQueue[T]) UpdateItem(item *PriorityItem[T]) {
	cpq.mu.Lock()
	defer cpq.mu.Unlock()
	cpq.pq.UpdateItem(item)
}

func (cpq *ConcurrentPriorityQueue[T]) RemoveItem(item *PriorityItem[T]) {
	cpq.mu.Lock()
	defer cpq.mu.Unlock()
	cpq.pq.RemoveItem(item)
}

func (cpq *ConcurrentPriorityQueue[T]) Peek() (value T, ok bool) {
	cpq.mu.Lock()
	defer cpq.mu.Unlock()
	return cpq.pq.Peek()
}

func (cpq *ConcurrentPriorityQueue[T]) Size() int {
	cpq.mu.Lock()
	defer cpq.mu.Unlock()
	return cpq.pq.Size()
}

func (cpq *ConcurrentPriorityQueue[T]) IsEmpty() bool {
	cpq.mu.Lock()
	defer cpq.mu.Unlock()
	return cpq.pq.IsEmpty()
}
