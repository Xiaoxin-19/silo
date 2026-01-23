package queues

import (
	"context"
	"sync"
)

type BlockingPriorityQueue[T any] struct {
	mu    sync.Mutex
	pq    *PriorityQueue[T]
	limit int

	// Signaling Channels (Buffered = 1 for Level-Triggered Semantics)
	notEmpty chan struct{}
	notFull  chan struct{}
	done     chan struct{} // Broadcast close
}

func NewBlockingPriorityQueue[T any](initCapacity int, isMinHeap bool, extractor func(T) int64, limit int) *BlockingPriorityQueue[T] {
	return &BlockingPriorityQueue[T]{
		pq:       NewPriorityQueue(initCapacity, isMinHeap, extractor),
		limit:    limit,
		notEmpty: make(chan struct{}, 1),
		notFull:  make(chan struct{}, 1),
		done:     make(chan struct{}),
	}
}

// maintainSignals ensures the signal channels reflect the queue's truth.
// MUST be called with lock held.
func (q *BlockingPriorityQueue[T]) maintainSignals() {
	// 1. Notify Consumers if data exists
	if q.pq.Size() > 0 {
		select {
		case q.notEmpty <- struct{}{}:
		default:
		}
	}
	// 2. Notify Producers if space exists
	if q.limit <= 0 || q.pq.Size() < q.limit {
		select {
		case q.notFull <- struct{}{}:
		default:
		}
	}
}

func (q *BlockingPriorityQueue[T]) EnqueueOrWait(ctx context.Context, value T) (*PriorityItem[T], error) {
	for {
		item, ok := q.TryEnqueue(value)
		if ok {
			return item, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-q.done:
			return nil, ErrQueueClosed
		case <-q.notFull:
			// Retry
		}
	}
}

func (q *BlockingPriorityQueue[T]) TryEnqueue(value T) (*PriorityItem[T], bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.limit > 0 && q.pq.Size() >= q.limit {
		return nil, false
	}

	item := q.pq.Enqueue(value)
	q.maintainSignals()
	return item, true
}

func (q *BlockingPriorityQueue[T]) DequeueOrWait(ctx context.Context) (T, bool) {
	for {
		val, ok := q.TryDequeue()
		if ok {
			return val, true
		}
		select {
		case <-ctx.Done():
			var zero T
			return zero, false
		case <-q.done:
			var zero T
			return zero, false
		case <-q.notEmpty:
			// Retry
		}
	}
}

func (q *BlockingPriorityQueue[T]) TryDequeue() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.pq.Size() == 0 {
		var zero T
		return zero, false
	}

	val, ok := q.pq.Dequeue()
	q.maintainSignals()
	return val, ok
}

func (q *BlockingPriorityQueue[T]) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.pq.Size()
}

func (q *BlockingPriorityQueue[T]) IsEmpty() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.pq.IsEmpty()
}

func (q *BlockingPriorityQueue[T]) Peek() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.pq.Peek()
}

// Ready allows usage in external select statements (e.g. Batcher)
func (q *BlockingPriorityQueue[T]) Ready() <-chan struct{} {
	return q.notEmpty
}

func (q *BlockingPriorityQueue[T]) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	select {
	case <-q.done:
		return // already closed
	default:
		close(q.done)
	}
}
