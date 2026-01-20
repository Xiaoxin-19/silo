package queues

import "sync"

// BlockingQueue is a thread-safe FIFO queue that supports blocking waits
type BlockingQueue[T any] struct {
	mu       sync.Mutex
	notEmpty *sync.Cond
	notFull  *sync.Cond
	q        *ArrayQueue[T]
	limit    int
}

// NewBlockingQueue creates a new BlockingQueue with the specified capacity and limit.
// If limit <= 0, the queue is unbounded.
func NewBlockingQueue[T any](capacity int, limit int) *BlockingQueue[T] {
	bq := &BlockingQueue[T]{
		q:     NewArrayQueue[T](capacity),
		limit: limit,
	}
	bq.notEmpty = sync.NewCond(&bq.mu)
	bq.notFull = sync.NewCond(&bq.mu)
	return bq
}

func (bq *BlockingQueue[T]) EnqueueOrWait(value T) {
	bq.mu.Lock()
	defer bq.mu.Unlock()
	for bq.limit > 0 && bq.q.size >= bq.limit {
		bq.notFull.Wait()
	}
	bq.q.Enqueue(value)
	bq.notEmpty.Signal()
}

func (bq *BlockingQueue[T]) TryEnqueue(value T) bool {
	bq.mu.Lock()
	defer bq.mu.Unlock()
	if bq.limit > 0 && bq.q.size >= bq.limit {
		return false
	}
	bq.q.Enqueue(value)
	bq.notEmpty.Signal()
	return true
}

func (bq *BlockingQueue[T]) EnqueueBatchOrWait(values []T) {
	bq.mu.Lock()
	defer bq.mu.Unlock()
	for bq.limit > 0 && bq.q.size+len(values) > bq.limit {
		bq.notFull.Wait()
	}
	bq.q.EnqueueAll(values...)
	bq.notEmpty.Signal()
}

func (bq *BlockingQueue[T]) TryEnqueueBatch(values []T) bool {
	bq.mu.Lock()
	defer bq.mu.Unlock()
	if bq.limit > 0 && bq.q.size+len(values) > bq.limit {
		return false
	}
	bq.q.EnqueueAll(values...)
	bq.notEmpty.Signal()
	return true
}

func (bq *BlockingQueue[T]) TryDequeue() (T, bool) {
	bq.mu.Lock()
	defer bq.mu.Unlock()
	if bq.q.size == 0 {
		var zero T
		return zero, false
	}
	val, ok := bq.q.Dequeue()
	if bq.limit > 0 {
		bq.notFull.Signal()
	}
	return val, ok
}

func (bq *BlockingQueue[T]) DequeueOrWait() (T, bool) {
	bq.mu.Lock()
	defer bq.mu.Unlock()
	for bq.q.size == 0 {
		bq.notEmpty.Wait()
	}
	val, ok := bq.q.Dequeue()
	if bq.limit > 0 {
		bq.notFull.Signal()
	}
	return val, ok
}

func (bq *BlockingQueue[T]) DequeueBatchOrWait(maxItems int) []T {
	bq.mu.Lock()
	defer bq.mu.Unlock()
	for bq.q.size == 0 {
		bq.notEmpty.Wait()
	}
	values := bq.q.DequeueBatch(maxItems)
	if bq.limit > 0 {
		bq.notFull.Signal()
	}
	return values
}

func (bq *BlockingQueue[T]) TryDequeueBatch(maxItems int) []T {
	bq.mu.Lock()
	defer bq.mu.Unlock()
	if bq.q.size == 0 {
		return nil
	}
	values := bq.q.DequeueBatch(maxItems)
	if bq.limit > 0 {
		bq.notFull.Signal()
	}
	return values
}

func (bq *BlockingQueue[T]) Peek() (T, bool) {
	bq.mu.Lock()
	defer bq.mu.Unlock()
	return bq.q.Peek()
}

func (bq *BlockingQueue[T]) Size() int {
	bq.mu.Lock()
	defer bq.mu.Unlock()
	return bq.q.size
}

func (bq *BlockingQueue[T]) IsEmpty() bool {
	bq.mu.Lock()
	defer bq.mu.Unlock()
	return bq.q.size == 0
}

func (bq *BlockingQueue[T]) Clear() {
	bq.mu.Lock()
	defer bq.mu.Unlock()
	bq.q.Clear()
	bq.notFull.Broadcast()
}

func (bq *BlockingQueue[T]) ResizeToFit() {
	bq.mu.Lock()
	defer bq.mu.Unlock()
	bq.q.ResizeToFit()
	bq.notFull.Broadcast()
}
