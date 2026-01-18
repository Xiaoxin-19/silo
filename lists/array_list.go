package lists

import (
	"fmt"
	"iter"
	"slices"
)

type ArrayList[T any] struct {
	data []T
}

var (
	ErrIndexOutOfBounds = fmt.Errorf("index out of bounds")
)

func NewArrayList[T any](initialCapacity int) *ArrayList[T] {
	if initialCapacity < 0 {
		initialCapacity = 0
	}
	return &ArrayList[T]{
		data: make([]T, 0, initialCapacity),
	}
}

func (al *ArrayList[T]) Add(values ...T) {
	al.data = append(al.data, values...)
}

func (al *ArrayList[T]) Insert(index int, value T) error {
	if index < 0 || index > len(al.data) {
		return ErrIndexOutOfBounds
	}

	var zero T
	al.data = append(al.data, zero)
	copy(al.data[index+1:], al.data[index:])
	al.data[index] = value
	return nil
}

// InsertAll inserts multiple elements at the specified index.
// Optimization: Performs exactly ONE allocation (if needed) and ONE memory shift.
func (l *ArrayList[T]) InsertAll(index int, values ...T) error {
	if index < 0 || index > len(l.data) {
		return ErrIndexOutOfBounds
	}

	n := len(values)
	if n == 0 {
		return nil
	}

	// calculate new length
	oldLen := len(l.data)
	newLen := oldLen + n

	//  grow underlying array if needed
	if newLen > cap(l.data) {
		newCap := max(newLen, 2*oldLen)

		// allocate new array
		newItems := make([]T, newLen, newCap)

		// convey data
		// [0...index] -> [0...index]
		copy(newItems, l.data[:index])
		// [index...] -> [index+n...] (leave a gap in the middle)
		copy(newItems[index+n:], l.data[index:])

		// fill in new values
		copy(newItems[index:], values)
		l.data = newItems
	} else {
		// enough capacity, in-place shift
		// resize to new length, to allow copy to work
		l.data = l.data[:newLen]

		// convey old data to make room
		// copy treats overlapping regions correctly
		copy(l.data[index+n:], l.data[index:])
		// fill in new values
		copy(l.data[index:], values)
	}

	return nil
}

func (al *ArrayList[T]) Get(index int) (T, error) {
	if index < 0 || index >= len(al.data) {
		var zero T
		return zero, ErrIndexOutOfBounds
	}
	return al.data[index], nil
}

func (al *ArrayList[T]) Set(index int, value T) error {
	if index < 0 || index >= len(al.data) {
		return ErrIndexOutOfBounds
	}
	al.data[index] = value
	return nil
}

func (al *ArrayList[T]) Remove(index int) (T, error) {
	if index < 0 || index >= len(al.data) {
		var zero T
		return zero, ErrIndexOutOfBounds
	}
	removed := al.data[index]
	copy(al.data[index:], al.data[index+1:])
	// clear the last element, let it be GCed
	clear(al.data[len(al.data)-1:])
	al.data = al.data[:len(al.data)-1]
	return removed, nil
}

// RemoveRange removes elements from index 'start' (inclusive) to 'end' (exclusive).
func (al *ArrayList[T]) RemoveRange(start, end int) error {
	if start < 0 || end > len(al.data) || start > end {
		return ErrIndexOutOfBounds
	}
	if start == end {
		return nil
	}

	// Shift data to the left
	copy(al.data[start:], al.data[end:])

	// Clear the tail to prevent memory leaks
	newLen := len(al.data) - (end - start)
	clear(al.data[newLen:])
	al.data = al.data[:newLen]
	return nil
}

func (al *ArrayList[T]) Swap(i, j int) {
	if i < 0 || i >= len(al.data) || j < 0 || j >= len(al.data) {
		return
	}
	al.data[i], al.data[j] = al.data[j], al.data[i]
}

func (al *ArrayList[T]) AddFirst(value T) {
	al.Insert(0, value)
}

func (al *ArrayList[T]) RemoveFirst() (T, error) {
	return al.Remove(0)
}

func (al *ArrayList[T]) RemoveLast() (T, error) {
	return al.Remove(len(al.data) - 1)
}

func (al *ArrayList[T]) First() (T, error) {
	return al.Get(0)
}

func (al *ArrayList[T]) Last() (T, error) {
	return al.Get(len(al.data) - 1)
}

func (al *ArrayList[T]) RemoveIf(predicate func(T) bool) int {
	al.data = slices.DeleteFunc(al.data, predicate)
	return len(al.data)
}

func (al *ArrayList[T]) Sort(compare func(a, b T) int) {
	slices.SortFunc(al.data, compare)
}

func (al *ArrayList[T]) Size() int {
	return len(al.data)
}

func (al *ArrayList[T]) IsEmpty() bool {
	return len(al.data) == 0
}

func (al *ArrayList[T]) Clear() {
	// clear the underlying array to let elements be GCed
	clear(al.data)
	al.data = al.data[:0]
}

// ResizeToFit reduces the capacity of the underlying array to match the current size.
// Use this to release memory when the list will no longer grow.
func (al *ArrayList[T]) ResizeToFit() {
	al.data = slices.Clip(al.data)
}

// Clone returns a shallow copy of the list.
// It allocates a new underlying slice and copies the elements.
// Note: If T is a pointer or reference type, the referenced data is shared.
func (l *ArrayList[T]) Clone() List[T] {
	newItems := make([]T, len(l.data))

	copy(newItems, l.data)
	return &ArrayList[T]{
		data: newItems,
	}
}

// String implements fmt.Stringer for easier debugging.
func (al *ArrayList[T]) String() string {
	return fmt.Sprintf("%v", al.data)
}

func (al *ArrayList[T]) ContainsFunc(predicate func(T) bool) bool {
	return slices.ContainsFunc(al.data, predicate)
}

func (al *ArrayList[T]) IndexFunc(predicate func(T) bool) int {
	return slices.IndexFunc(al.data, predicate)
}

func (al *ArrayList[T]) Values() iter.Seq[T] {
	return slices.Values(al.data)
}

func (al *ArrayList[T]) All() iter.Seq2[int, T] {
	return slices.All(al.data)
}

func (al *ArrayList[T]) Backward() iter.Seq2[int, T] {
	return slices.Backward(al.data)
}
