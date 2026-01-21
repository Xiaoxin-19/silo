package queues

type Queue[T any] interface {
	// puts an element at the end of the queue
	Enqueue(value T)
	// puts multiple elements at the end of the queue
	EnqueueAll(values ...T)
	// removes and returns the element at the front of the queue
	Dequeue() (value T, ok bool)
	// batch removes and returns up to len(dst) elements from the front of the queue into dst
	DequeueBatchInto(dst []T) int
	// returns the element at the front of the queue without removing it
	Peek() (value T, ok bool)
	// returns the number of elements in the queue
	Size() int
	// returns true if the queue is empty
	IsEmpty() bool
	// removes all elements from the queue
	Clear()
}
