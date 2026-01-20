package queues

import "sync"

// BlockingPriorityQueue is a thread-safe priority queue that supports blocking enqueue and dequeue operations.
type BlockingPriorityQueue[T any] struct {
	mu sync.Mutex
	pq *PriorityQueue[T]

	notEmpty *sync.Cond
	notFull  *sync.Cond

	// limit <=0 represents unbounded
	limit int
}

// NewBlockingPriorityQueue creates a new BlockingPriorityQueue with the specified initial capacity, heap type, priority extractor, and limit.
// If isMinHeap is true, it creates a min-heap; otherwise, it creates a max-heap.
// PriorityExtractor is a function that extracts the priority value from an element of type T.
// If limit <= 0, the queue is unbounded.
func NewBlockingPriorityQueue[T any](initCapacity int, isMinHeap bool, extractor func(T) int64, limit int) *BlockingPriorityQueue[T] {
	bpq := &BlockingPriorityQueue[T]{
		pq:    NewPriorityQueue(initCapacity, isMinHeap, extractor),
		limit: limit,
	}

	bpq.notEmpty = sync.NewCond(&bpq.mu)
	bpq.notFull = sync.NewCond(&bpq.mu)
	return bpq
}

// EnqueueOrWait blocking enqueue until there is space
func (q *BlockingPriorityQueue[T]) EnqueueOrWait(value T) *PriorityItem[T] {
	q.mu.Lock()
	defer q.mu.Unlock()

	for q.limit > 0 && q.pq.Size() >= q.limit {
		q.notFull.Wait()
	}

	item := q.pq.Enqueue(value)

	q.notEmpty.Signal()
	return item
}

// DequeueOrWait blocking dequeue until there is data
func (q *BlockingPriorityQueue[T]) DequeueOrWait() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for q.pq.Size() == 0 {
		q.notEmpty.Wait()
	}

	val, ok := q.pq.Dequeue()

	if q.limit > 0 {
		q.notFull.Signal()
	}

	return val, ok
}

// TryEnqueue non-blocking enqueue, returns false if full
func (q *BlockingPriorityQueue[T]) TryEnqueue(value T) (*PriorityItem[T], bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.limit > 0 && q.pq.Size() >= q.limit {
		return nil, false
	}

	item := q.pq.Enqueue(value)
	q.notEmpty.Signal()
	return item, true
}

// TryDequeue non-blocking dequeue, returns false if empty
func (q *BlockingPriorityQueue[T]) TryDequeue() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.pq.Size() == 0 {
		var zero T
		return zero, false
	}

	val, ok := q.pq.Dequeue()
	if q.limit > 0 {
		q.notFull.Signal()
	}
	return val, ok
}

// Len returns the thread-safe length of the queue
func (q *BlockingPriorityQueue[T]) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.pq.Size()
}
