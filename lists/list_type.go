package lists

import "iter"

// List Defines a generic list interface supporting common list operations.
// T can be any type.
type List[T any] interface {
	// -------------------------------------------------------
	// Basic Operations
	// -------------------------------------------------------

	// Add appends one or more elements to the end of the list
	// Corresponds to the append operation on slices or tail insert on linked lists
	Add(values ...T)

	// Insert inserts an element at the specified index
	// Returns an error if index < 0 or index > Size()
	Insert(index int, value T) error

	// Remove removes and returns the element at the specified index
	// Returns an error if index is out of bounds
	Remove(index int) (T, error)

	// Set modifies the element at the specified index
	// Returns an error if index is out of bounds
	Set(index int, value T) error

	// Get retrieves the element at the specified index
	// Returns an error if index is out of bounds
	Get(index int) (T, error)

	// -------------------------------------------------------
	// Query Operations
	// -------------------------------------------------------

	// Size returns the current number of elements in the list
	Size() int

	// IsEmpty checks if the list is empty
	IsEmpty() bool

	// Clear clears the list and releases memory
	Clear()

	// Contains checks if the list contains a specific element
	// Since T is any, direct comparison using == is not possible, so an equality function equal must be provided
	Contains(value T, equal func(a, b T) bool) bool

	// IndexOf finds the first occurrence index of an element, returns -1 if not found
	IndexOf(value T, equal func(a, b T) bool) int

	// -------------------------------------------------------
	// Transformation & Iteration
	// -------------------------------------------------------
	All() iter.Seq[T]
}

// FindIndex 是一个独立的函数，不是 List 的方法
// 它要求 T 必须是 comparable
func FindIndex[T comparable](l List[T], v T) int {
	return l.IndexOf(v, func(a, b T) bool {
		return a == b // 这里可以使用 ==，因为泛型约束是 comparable
	})
}
