package lists

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

	// ToSlice converts the list to a native slice
	// This is an "escape hatch" method for users to fall back to standard library operations
	ToSlice() []T

	// Iterator returns an iterator for efficient traversal (mainly for LinkedList)
	Iterator() Iterator[T]
}

// Iterator defines the behavior of an iterator
// Allows users to traverse using for it.Next() without worrying about whether the underlying structure is an array or a linked list
type Iterator[T any] interface {
	// HasNext checks if there is a next element
	HasNext() bool

	// Next returns the current element and advances the cursor to the next position
	// If there are no elements, the behavior is undefined (usually panic or return zero value, recommended to be decided by the implementation)
	Next() T

	// Index returns the index of the current element (optional but useful in some scenarios)
	Index() int
}

// FindIndex 是一个独立的函数，不是 List 的方法
// 它要求 T 必须是 comparable
func FindIndex[T comparable](l List[T], v T) int {
	return l.IndexOf(v, func(a, b T) bool {
		return a == b // 这里可以使用 ==，因为泛型约束是 comparable
	})
}
