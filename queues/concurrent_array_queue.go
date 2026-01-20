package queues

import "sync"

// ConcurrentQueue is a thread-safe wrapper for ArrayQueue (non-blocking)
type ConcurrentQueue[T any] struct {
	mu sync.Mutex
	q  *ArrayQueue[T]
}

func NewConcurrentQueue[T any](capacity int) *ConcurrentQueue[T] {
	return &ConcurrentQueue[T]{
		q: NewArrayQueue[T](capacity),
	}
}

func (cq *ConcurrentQueue[T]) Enqueue(value T) {
	cq.mu.Lock()
	defer cq.mu.Unlock()
	cq.q.Enqueue(value)
}

func (cq *ConcurrentQueue[T]) EnqueueBatch(values ...T) {
	cq.mu.Lock()
	defer cq.mu.Unlock()
	cq.q.EnqueueAll(values...)
}

func (cq *ConcurrentQueue[T]) Dequeue() (T, bool) {
	cq.mu.Lock()
	defer cq.mu.Unlock()
	return cq.q.Dequeue()
}

func (cq *ConcurrentQueue[T]) DequeueBatch(maxElements int) []T {
	cq.mu.Lock()
	defer cq.mu.Unlock()
	return cq.q.DequeueBatch(maxElements)
}

func (cq *ConcurrentQueue[T]) Size() int {
	cq.mu.Lock()
	defer cq.mu.Unlock()
	return cq.q.Size()
}

func (cq *ConcurrentQueue[T]) IsEmpty() bool {
	cq.mu.Lock()
	defer cq.mu.Unlock()
	return cq.q.Size() == 0
}

func (cq *ConcurrentQueue[T]) Clear() {
	cq.mu.Lock()
	defer cq.mu.Unlock()
	cq.q.Clear()
}

func (cq *ConcurrentQueue[T]) Peek() (T, bool) {
	cq.mu.Lock()
	defer cq.mu.Unlock()
	return cq.q.Peek()
}

func (cq *ConcurrentQueue[T]) EnqueueAll(values ...T) {
	cq.mu.Lock()
	defer cq.mu.Unlock()
	cq.q.EnqueueAll(values...)
}

func (cq *ConcurrentQueue[T]) ResizeToFit() {
	cq.mu.Lock()
	defer cq.mu.Unlock()
	cq.q.ResizeToFit()
}
