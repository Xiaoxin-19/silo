package lists

import "iter"

// List defines a generic list interface.
// It combines the characteristics of RandomAccess lists and Linked lists.
type List[T any] interface {
	// --- Basic Operations ---
	// Add appends values to the end of the list.
	Add(values ...T)
	// Insert inserts value at the specified index.
	Insert(index int, value T) error
	// InsertAll inserts multiple values at the specified index.
	InsertAll(index int, values ...T) error
	// Get retrieves the element at the specified index.
	Get(index int) (T, error)
	// Set replaces the element at the specified index.
	Set(index int, value T) error
	// Remove removes the element at the specified index.
	Remove(index int) (T, error)
	// RemoveRange removes elements from start (inclusive) to end (exclusive).
	RemoveRange(start, end int) error
	// Swap swaps the elements at the specified indices.
	Swap(i, j int)

	// --- Deque Operations (Optimized for LinkedList) ---
	// AddFirst prepends a value to the list.
	AddFirst(value T)
	// RemoveFirst removes and returns the first element.
	RemoveFirst() (T, error)
	// RemoveLast removes and returns the last element.
	RemoveLast() (T, error)
	// First returns the first element without removing it.
	First() (T, error)
	// Last returns the last element without removing it.
	Last() (T, error)

	// --- Bulk & Functional Operations ---
	// RemoveIf removes all elements satisfying the predicate.
	// This allows O(N) filtering for both ArrayList and LinkedList.
	RemoveIf(predicate func(T) bool) int
	// Sort sorts the list in-place.
	// implementation chooses the best algorithm (QuickSort for Array, MergeSort for Linked).
	Sort(compare func(a, b T) int)

	// --- Query Operations ---
	Size() int
	IsEmpty() bool
	Clear()
	// ContainsFunc returns true if any element satisfies the predicate.
	ContainsFunc(predicate func(T) bool) bool
	// IndexFunc returns the index of the first element satisfying the predicate, or -1 if not found.
	IndexFunc(predicate func(T) bool) int

	// --- Iteration (Go 1.23+) ---
	Values() iter.Seq[T]
	All() iter.Seq2[int, T]
	Backward() iter.Seq2[int, T]
}

// Helper functions for comparable types
func Index[T comparable](l List[T], v T) int {
	return l.IndexFunc(func(item T) bool {
		return item == v
	})
}

func Contains[T comparable](l List[T], v T) bool {
	return l.ContainsFunc(func(item T) bool {
		return item == v
	})
}
