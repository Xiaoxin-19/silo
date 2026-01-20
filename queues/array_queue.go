package queues

import "math/bits"

// ArrayQueue is a generic queue implementation using a circular array (ring buffer).
// It supports efficient enqueue and dequeue operations with amortized O(1) time complexity.
type ArrayQueue[T any] struct {
	buf  []T // backing array, length == capacity (power of two)
	head int // index of the first element
	size int // number of elements in the queue
	mask int // capacity - 1, used for fast modulo: idx & mask
}

// NewArrayQueue creates a new ArrayQueue with the specified initial capacity.
func NewArrayQueue[T any](initialCapacity int) *ArrayQueue[T] {
	if initialCapacity <= 0 {
		initialCapacity = 16
	}

	// compute capacity as the next power of two >= initialCapacity
	var capacity int
	if initialCapacity <= 1 {
		capacity = 1
	} else {
		// use math/bits to quickly compute (see the single line code above)
		capacity = 1 << uint(bits.Len(uint(initialCapacity-1)))
	}

	//  allocate the underlying slice and initialize mask (capacity is a power of two, mask = capacity-1)
	buf := make([]T, capacity)
	mask := capacity - 1

	return &ArrayQueue[T]{
		buf:  buf,
		head: 0,
		size: 0,
		mask: mask,
	}
}

// resize resizes the underlying buffer.
// If isShrink is true, it shrinks the buffer to fit the current size.
// If isShrink is false, it doubles the buffer capacity.
// capDiff is the difference in capacity to accommodate (used when growing).
// all capacities are powers of two.
func (aq *ArrayQueue[T]) resize(capDiff int, isShrink bool) {
	var newCapacity int
	switch {
	case isShrink && aq.size == 0:
		newCapacity = 1
	case isShrink:
		newCapacity = 1 << uint(bits.Len(uint(aq.size-1)))
	default:
		newCapacity = 1 << uint(bits.Len(uint(aq.size+capDiff-1)))
	}

	newBuf := make([]T, newCapacity)

	if aq.head+aq.size <= len(aq.buf) {
		// if not wrapped around
		copy(newBuf, aq.buf[aq.head:aq.head+aq.size])
	} else {
		// wrapped around
		// copy head to end
		n := copy(newBuf, aq.buf[aq.head:])
		// copy from start to tail
		tailPos := (aq.head + aq.size) & aq.mask
		copy(newBuf[n:], aq.buf[:tailPos])
	}

	clear(aq.buf)
	// update fields
	aq.buf = newBuf
	aq.head = 0
	aq.mask = newCapacity - 1
}

func (aq *ArrayQueue[T]) Enqueue(value T) {
	if aq.size == len(aq.buf) {
		aq.resize(1, false)
	}
	aq.buf[(aq.head+aq.size)&aq.mask] = value
	aq.size++
}

func (aq *ArrayQueue[T]) EnqueueAll(values ...T) {
	n := len(values)
	if aq.size+n > len(aq.buf) {
		aq.resize(n, false)
	}
	tail := (aq.head + aq.size) & aq.mask
	if tail+n <= len(aq.buf) {
		copy(aq.buf[tail:], values)
	} else {
		// wrapped around
		part1Len := len(aq.buf) - tail
		copy(aq.buf[tail:], values[:part1Len])
		copy(aq.buf, values[part1Len:])
	}
	aq.size += n
}

func (aq *ArrayQueue[T]) Dequeue() (value T, ok bool) {
	if aq.size == 0 {
		return value, false
	}
	value = aq.buf[aq.head]
	var zero T
	aq.buf[aq.head] = zero // clear reference
	aq.head = (aq.head + 1) & aq.mask
	aq.size--
	return value, true
}

func (aq *ArrayQueue[T]) DequeueBatch(maxElements int) (values []T) {
	if aq.size == 0 {
		return values
	}
	if maxElements > aq.size {
		maxElements = aq.size
	}
	values = make([]T, maxElements)
	if aq.head+maxElements <= len(aq.buf) {
		copy(values, aq.buf[aq.head:aq.head+maxElements])
		// clear references
		clear(aq.buf[aq.head : aq.head+maxElements])
	} else {
		// wrapped around
		part1Len := len(aq.buf) - aq.head
		copy(values, aq.buf[aq.head:])
		copy(values[part1Len:], aq.buf[:maxElements-part1Len])
		// clear references
		clear(aq.buf[aq.head:])
		clear(aq.buf[:maxElements-part1Len])
	}
	aq.head = (aq.head + maxElements) & aq.mask
	aq.size -= maxElements
	return values
}

func (aq *ArrayQueue[T]) Peek() (value T, ok bool) {
	if aq.size == 0 {
		return value, false
	}
	return aq.buf[aq.head], true
}

func (aq *ArrayQueue[T]) Size() int {
	return aq.size
}

func (aq *ArrayQueue[T]) IsEmpty() bool {
	return aq.size == 0
}

func (aq *ArrayQueue[T]) Clear() {
	clear(aq.buf)
	aq.head = 0
	aq.size = 0
}

func (aq *ArrayQueue[T]) ResizeToFit() {
	aq.resize(0, true)
}
