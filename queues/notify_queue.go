package queues

import "sync"

type NotifyQueue[T any] struct {
	mu       sync.Mutex
	q        *ArrayQueue[T]
	notEmpty *sync.Cond
	notFull  *sync.Cond
	limit    int
	closed   bool

	notifyCh  chan struct{} // Buffered channel to signal readiness, size 1, never closed
	doneCh    chan struct{} // Closed when the queue is closed
	closeOnce sync.Once
}

// NewNotifyQueue creates a new NotifyQueue with the specified capacity.
// If limit <= 0, the queue is unbounded.
func NewNotifyQueue[T any](capacity int, limit int) *NotifyQueue[T] {
	nq := &NotifyQueue[T]{
		q:        NewArrayQueue[T](capacity),
		limit:    limit,
		notifyCh: make(chan struct{}, 1),
		doneCh:   make(chan struct{}),
	}
	nq.notEmpty = sync.NewCond(&nq.mu)
	nq.notFull = sync.NewCond(&nq.mu)
	return nq
}

// signal ensures the notifyCh has a signal if the queue is not empty.
// Must be called with lock held.
func (nq *NotifyQueue[T]) signal() {
	if nq.q.Size() > 0 {
		select {
		case nq.notifyCh <- struct{}{}:
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
	nq.notEmpty.Signal()
	nq.signal()
	return true, nil
}

// Enqueue adds an element to the queue with blocking wait and submit a Ready signal.
// If the queue is closed, it returns an error.
// If try enqueue fails, it returns (false, nil).
func (nq *NotifyQueue[T]) EnqueueOrWait(value T) error {
	nq.mu.Lock()
	defer nq.mu.Unlock()
	if nq.closed {
		return ErrQueueClosed
	}
	for nq.limit > 0 && nq.q.Size() >= nq.limit {
		nq.notFull.Wait()
		if nq.closed {
			return ErrQueueClosed
		}
	}
	nq.q.Enqueue(value)
	nq.notEmpty.Signal()
	nq.signal()
	return nil
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
	nq.notEmpty.Signal()
	nq.signal()
	return true, nil
}

// EnqueueBatch adds multiple elements to the queue with blocking wait and submit a Ready signal.
// If the queue is closed, it returns an error.
// If try enqueue fails, it returns (false, nil).
func (nq *NotifyQueue[T]) EnqueueBatchOrWait(values ...T) error {
	nq.mu.Lock()
	defer nq.mu.Unlock()
	if nq.closed {
		return ErrQueueClosed
	}
	for nq.limit > 0 && nq.q.Size()+len(values) > nq.limit {
		nq.notFull.Wait()
		if nq.closed {
			return ErrQueueClosed
		}
	}
	nq.q.EnqueueAll(values...)
	nq.notEmpty.Signal()
	nq.signal()
	return nil
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
	if nq.limit > 0 {
		nq.notFull.Signal()
	}
	nq.signal() // Re-signal if more items remain
	return val, ok
}

// Dequeue removes and returns an element from the queue with blocking wait.
// If the queue is empty, it blocks until data is available.
func (nq *NotifyQueue[T]) DequeueOrWait() (T, bool) {
	nq.mu.Lock()
	defer nq.mu.Unlock()
	for nq.q.Size() == 0 {
		nq.notEmpty.Wait()
		if nq.closed {
			var zero T
			return zero, false
		}
	}
	val, ok := nq.q.Dequeue()
	if nq.limit > 0 {
		nq.notFull.Signal()
	}
	nq.signal()
	return val, ok
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
	if nq.limit > 0 {
		nq.notFull.Signal()
	}
	nq.signal()
	return count
}

// DequeueBatchOrWait removes up to len(dst) elements from the queue into dst with blocking wait.
// It returns the number of elements copied.
func (nq *NotifyQueue[T]) DequeueBatchOrWait(dst []T) int {
	nq.mu.Lock()
	defer nq.mu.Unlock()
	for nq.q.Size() == 0 {
		nq.notEmpty.Wait()
		if nq.closed {
			return 0
		}
	}
	count := nq.q.DequeueBatchInto(dst)
	if nq.limit > 0 {
		nq.notFull.Signal()
	}
	nq.signal()
	return count
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
	return nq.notifyCh
}

// Done returns a channel that is closed when the queue is closed.
func (nq *NotifyQueue[T]) Done() <-chan struct{} {
	return nq.doneCh
}

// Close closes the queue. After closing, no more elements can be enqueued.
// Dequeue operations can still be performed until the queue is empty.
func (nq *NotifyQueue[T]) Close() {
	nq.closeOnce.Do(func() {
		nq.mu.Lock()
		defer nq.mu.Unlock()
		if nq.closed {
			return
		}
		nq.closed = true
		nq.notFull.Broadcast()
		nq.notEmpty.Broadcast()
		close(nq.doneCh)
	})
}

// IsClosed returns whether the queue has been closed.
func (nq *NotifyQueue[T]) IsClosed() bool {
	nq.mu.Lock()
	defer nq.mu.Unlock()
	return nq.closed
}
