package queues

import (
	"context"
	"sync"
)

type NotifyQueue[T any] struct {
	mu       sync.Mutex
	q        *ArrayQueue[T]
	notEmpty chan struct{}
	notFull  chan struct{}
	limit    int
	closed   bool
	doneCh   chan struct{} // Closed when the queue is closed
}

// NewNotifyQueue creates a new NotifyQueue with the specified capacity.
// If limit <= 0, the queue is unbounded.
func NewNotifyQueue[T any](capacity int, limit int) *NotifyQueue[T] {
	nq := &NotifyQueue[T]{
		q:        NewArrayQueue[T](capacity),
		limit:    limit,
		notEmpty: make(chan struct{}, 1),
		notFull:  make(chan struct{}, 1),
		doneCh:   make(chan struct{}),
	}
	return nq
}

// signal ensures the notifyCh has a signal if the queue is not empty.
// Must be called with lock held.
func (nq *NotifyQueue[T]) maintainSignals() {
	if nq.q.Size() > 0 {
		select {
		case nq.notEmpty <- struct{}{}:
		default:
		}
	}

	if nq.limit <= 0 || nq.q.Size() < nq.limit {
		select {
		case nq.notFull <- struct{}{}:
		default:
		}
	}
}

// Enqueue adds an element to the queue and submit a Ready signal.
// If the queue is closed, it returns an error.
// If try enqueue fails, it returns (false, nil).
func (nq *NotifyQueue[T]) TryEnqueue(value T) (bool, error) {
	nq.mu.Lock()
	defer nq.mu.Unlock()
	if nq.closed {
		return false, ErrQueueClosed
	}
	if nq.limit > 0 && nq.q.Size() >= nq.limit {
		return false, nil
	}
	nq.q.Enqueue(value)
	nq.maintainSignals()
	return true, nil
}

// Enqueue adds an element to the queue with blocking wait and submit a Ready signal.
// If the queue is closed, it returns an error.
// If try enqueue fails, it returns (false, nil).
func (nq *NotifyQueue[T]) EnqueueOrWait(ctx context.Context, value T) error {
	for {
		added, err := nq.TryEnqueue(value)
		if err != nil {
			return err
		}
		if added {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-nq.doneCh:
			return ErrQueueClosed
		case <-nq.notFull:
			// Retry
		}
	}
}

// EnqueueBatch adds multiple elements to the queue and submit a Ready signal.
// If the queue is closed, it returns an error.
// If try enqueue fails, it returns (false, nil).
func (nq *NotifyQueue[T]) TryEnqueueBatch(values ...T) (bool, error) {
	nq.mu.Lock()
	defer nq.mu.Unlock()
	if nq.closed {
		return false, ErrQueueClosed
	}
	if nq.limit > 0 && nq.q.Size()+len(values) > nq.limit {
		return false, nil
	}
	nq.q.EnqueueAll(values...)
	nq.maintainSignals()
	return true, nil
}

// EnqueueBatch adds multiple elements to the queue with blocking wait and submit a Ready signal.
// If the queue is closed, it returns an error.
// If try enqueue fails, it returns (false, nil).
func (nq *NotifyQueue[T]) EnqueueBatchOrWait(ctx context.Context, values ...T) error {
	for {
		added, err := nq.TryEnqueueBatch(values...)
		if err != nil {
			return err
		}
		if added {
			return nil
		}

		select {
		case <-nq.doneCh:
			return ErrQueueClosed
		case <-ctx.Done():
			return ctx.Err()
		case <-nq.notFull:
			// Retry
		}
	}
}

// Dequeue removes and returns an element from the queue.
// If the queue is empty, it returns false.
func (nq *NotifyQueue[T]) TryDequeue() (T, bool) {
	nq.mu.Lock()
	defer nq.mu.Unlock()
	if nq.q.Size() == 0 {
		var zero T
		return zero, false
	}
	val, ok := nq.q.Dequeue()
	nq.maintainSignals()
	return val, ok
}

// Dequeue removes and returns an element from the queue with blocking wait.
// If the queue is empty, it blocks until data is available.
func (nq *NotifyQueue[T]) DequeueOrWait(ctx context.Context) (T, bool) {
	for {
		val, ok := nq.TryDequeue()
		if ok {
			return val, true
		}
		select {
		case <-ctx.Done():
			var zero T
			return zero, false
		case <-nq.doneCh:
			var zero T
			return zero, false
		case <-nq.notEmpty:
			// Retry
		}
	}
}

// TryDequeueBatchInto removes up to len(dst) elements from the queue and copies them into dst.
// It returns the number of elements copied.
func (nq *NotifyQueue[T]) TryDequeueBatchInto(dst []T) int {
	nq.mu.Lock()
	defer nq.mu.Unlock()
	if nq.q.Size() == 0 {
		return 0
	}
	count := nq.q.DequeueBatchInto(dst)
	nq.maintainSignals()
	return count
}

// DequeueBatchOrWait removes up to len(dst) elements from the queue into dst with blocking wait.
// It returns the number of elements copied.
func (nq *NotifyQueue[T]) DequeueBatchOrWait(ctx context.Context, dst []T) int {
	for {
		count := nq.TryDequeueBatchInto(dst)
		if count > 0 {
			return count
		}
		select {
		case <-ctx.Done():
			return 0
		case <-nq.doneCh:
			return 0
		case <-nq.notEmpty:
			// Retry
		}
	}
}

// Size returns the current number of elements in the queue.
func (nq *NotifyQueue[T]) Size() int {
	nq.mu.Lock()
	defer nq.mu.Unlock()
	return nq.q.Size()
}

// Ready returns a channel that signals when the queue has data.
// If the channel is readable, the queue is not guaranteed to have data, because of possible races.
func (nq *NotifyQueue[T]) Ready() <-chan struct{} {
	return nq.notEmpty
}

// Done returns a channel that is closed when the queue is closed.
func (nq *NotifyQueue[T]) Done() <-chan struct{} {
	return nq.doneCh
}

// Close closes the queue. After closing, no more elements can be enqueued.
// Dequeue operations can still be performed until the queue is empty.
func (nq *NotifyQueue[T]) Close() {
	nq.mu.Lock()
	defer nq.mu.Unlock()
	if nq.closed {
		return
	}
	nq.closed = true
	close(nq.doneCh)
}

// IsClosed returns whether the queue has been closed.
func (nq *NotifyQueue[T]) IsClosed() bool {
	nq.mu.Lock()
	defer nq.mu.Unlock()
	return nq.closed
}
