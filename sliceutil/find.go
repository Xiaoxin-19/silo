package sliceutil

// Contains checks if the target element exists in the collection.
// Works for comparable types.
func Contains[T comparable](collection []T, target T) bool {
	if len(collection) == 0 {
		return false
	}
	_ = collection[len(collection)-1] // BCE hint
	for _, v := range collection {
		if v == target {
			return true
		}
	}
	return false
}

// ContainsFunc checks if any element satisfies the predicate.
// Useful for non-comparable types or custom matching logic.
func ContainsFunc[T any](collection []T, predicate func(T) bool) bool {
	if len(collection) == 0 {
		return false
	}
	_ = collection[len(collection)-1]

	for _, item := range collection {
		if predicate(item) {
			return true
		}
	}
	return false
}

// Find searches for the first element that satisfies the predicate.
// Returns the element and true if found, otherwise returns the zero value and false.
func Find[T any](collection []T, predicate func(T) bool) (T, bool) {
	var target T
	if len(collection) == 0 {
		return target, false
	}
	_ = collection[len(collection)-1] // BCE hint
	for _, v := range collection {
		if predicate(v) {
			return v, true
		}
	}
	return target, false
}

// FindIndex searches for the index of the first element that satisfies the predicate.
// Returns the index if found, otherwise returns -1.
func FindIndex[T any](collection []T, predicate func(T) bool) int {
	if len(collection) == 0 {
		return -1
	}
	_ = collection[len(collection)-1]

	for i, item := range collection {
		if predicate(item) {
			return i
		}
	}
	return -1
}
