package queues

import "sync"

type NotifyQueue[T any] struct {
	bq        *BlockingQueue[T]
	notifyCh  chan struct{} // Buffered channel to signal readiness, size 1, never closed
	doneCh    chan struct{} // Closed when the queue is closed
	closeOnce sync.Once
}

// NewNotifyQueue creates a new NotifyQueue with the specified capacity.
// If limit <= 0, the queue is unbounded.
func NewNotifyQueue[T any](capacity int, limit int) *NotifyQueue[T] {
	return &NotifyQueue[T]{
		bq:       NewBlockingQueue[T](capacity, limit),
		notifyCh: make(chan struct{}, 1),
		doneCh:   make(chan struct{}),
	}
}

// Enqueue adds an element to the queue and submit a Ready signal.
// If the queue is closed, it returns an error.
// If try enqueue fails, it returns (false, nil).
func (nq *NotifyQueue[T]) TryEnqueue(value T) (bool, error) {
	success, err := nq.bq.TryEnqueue(value)
	// Ensure signal is present
	if success {
		nq.sendReadySignal()
	}
	return success, err
}

// Enqueue adds an element to the queue with blocking wait and submit a Ready signal.
// If the queue is closed, it returns an error.
// If try enqueue fails, it returns (false, nil).
func (nq *NotifyQueue[T]) EnqueueOrWait(value T) error {
	err := nq.bq.EnqueueOrWait(value)
	// Ensure signal is present
	if err == nil {
		nq.sendReadySignal()
	}
	return err
}

// EnqueueBatch adds multiple elements to the queue and submit a Ready signal.
// If the queue is closed, it returns an error.
// If try enqueue fails, it returns (false, nil).
func (nq *NotifyQueue[T]) TryEnqueueBatch(values ...T) (bool, error) {
	success, err := nq.bq.TryEnqueueBatch(values)
	// Ensure signal is present
	if success {
		nq.sendReadySignal()
	}
	return success, err
}

// EnqueueBatch adds multiple elements to the queue with blocking wait and submit a Ready signal.
// If the queue is closed, it returns an error.
// If try enqueue fails, it returns (false, nil).
func (nq *NotifyQueue[T]) EnqueueBatchOrWait(values ...T) error {
	err := nq.bq.EnqueueBatchOrWait(values)
	// Ensure signal is present
	if err == nil {
		nq.sendReadySignal()
	}
	return err
}

// Dequeue removes and returns an element from the queue.
// If the queue is empty, it returns false.
func (nq *NotifyQueue[T]) TryDequeue() (T, bool) {
	val, ok := nq.bq.TryDequeue()
	// If the queue is not empty, ensure signal is present
	if ok {
		nq.ensureNotifyCh()
	}
	return val, ok
}

// Dequeue removes and returns an element from the queue with blocking wait.
// If the queue is empty, it blocks until data is available.
func (nq *NotifyQueue[T]) DequeueOrWait() (T, bool) {
	val, ok := nq.bq.DequeueOrWait()
	// If the queue is not empty, ensure signal is present
	nq.ensureNotifyCh()
	return val, ok
}

// TryDequeueBatchInto removes up to len(dst) elements from the queue and copies them into dst.
// It returns the number of elements copied.
func (nq *NotifyQueue[T]) TryDequeueBatchInto(dst []T) int {
	count := nq.bq.TryDequeueBatchInto(dst)
	// If the queue is not empty, ensure signal is present
	if count > 0 {
		nq.ensureNotifyCh()
	}
	return count
}

// DequeueBatchOrWait removes up to len(dst) elements from the queue into dst with blocking wait.
// It returns the number of elements copied.
func (nq *NotifyQueue[T]) DequeueBatchOrWait(dst []T) int {
	count := nq.bq.DequeueBatchOrWait(dst)
	// If the queue is not empty, ensure signal is present
	nq.ensureNotifyCh()
	return count
}

// Size returns the current number of elements in the queue.
func (nq *NotifyQueue[T]) Size() int {
	return nq.bq.Size()
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
		nq.bq.Close()
		close(nq.doneCh)
	})
}

// IsClosed returns whether the queue has been closed.
func (nq *NotifyQueue[T]) IsClosed() bool {
	return nq.bq.IsClosed()
}

func (nq *NotifyQueue[T]) ensureNotifyCh() {
	if nq.bq.Size() > 0 {
		nq.sendReadySignal()
	}
}

func (nq *NotifyQueue[T]) sendReadySignal() {
	select {
	case nq.notifyCh <- struct{}{}:
	default:
	}
}
