package queues

import "sync"

type NotifyQueue[T any] struct {
	mu       sync.Mutex
	q        *ArrayQueue[T]
	notifyCh chan struct{}
}

// NewNotifyQueue creates a new NotifyQueue with the specified capacity.
func NewNotifyQueue[T any](capacity int) *NotifyQueue[T] {
	return &NotifyQueue[T]{
		q:        NewArrayQueue[T](capacity),
		notifyCh: make(chan struct{}, 1),
	}
}

func (nq *NotifyQueue[T]) Enqueue(value T) {
	nq.mu.Lock()
	defer nq.mu.Unlock()
	nq.q.Enqueue(value)
	// Ensure signal is present
	select {
	case nq.notifyCh <- struct{}{}:
	default:
	}
}

func (nq *NotifyQueue[T]) EnqueueBatch(values ...T) {
	nq.mu.Lock()
	defer nq.mu.Unlock()
	nq.q.EnqueueAll(values...)
	// Ensure signal is present
	select {
	case nq.notifyCh <- struct{}{}:
	default:
	}
}

func (nq *NotifyQueue[T]) Dequeue() (T, bool) {
	nq.mu.Lock()
	defer nq.mu.Unlock()
	val, ok := nq.q.Dequeue()

	// If the queue is not empty, ensure signal is present
	if nq.q.Size() != 0 {
		select {
		case nq.notifyCh <- struct{}{}:
		default:
		}
	}
	return val, ok
}

func (nq *NotifyQueue[T]) DequeueBatch(maxElements int) []T {
	nq.mu.Lock()
	defer nq.mu.Unlock()
	vals := nq.q.DequeueBatch(maxElements)

	// If the queue is not empty, ensure signal is present
	if nq.q.Size() != 0 {
		select {
		case nq.notifyCh <- struct{}{}:
		default:
		}
	}
	return vals
}

func (nq *NotifyQueue[T]) Size() int {
	nq.mu.Lock()
	defer nq.mu.Unlock()
	return nq.q.Size()
}

// Ready returns a channel that signals when the queue has data.
// If the channel is readable, the queue is guaranteed to be non-empty (at that moment).
func (nq *NotifyQueue[T]) Ready() <-chan struct{} {
	return nq.notifyCh
}
