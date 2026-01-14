package slices

// Filter returns a new slice containing all elements of the collection that satisfy the predicate.
func Filter[T any](collection []T, predicate func(T) bool) []T {
	res := make([]T, 0, len(collection)/2)
	for _, v := range collection {
		if predicate(v) {
			res = append(res, v)
		}
	}
	return res
}

// Map applies a transformation function to each element of the collection and returns a new slice of the results.
func Map[T any, R any](collection []T, transform func(T) R) []R {
	res := make([]R, len(collection))
	for i, v := range collection {
		res[i] = transform(v)
	}
	return res
}

// Reduce reduces the elements of the slice to a single value.
func Reduce[T any, R any](collection []T, accumulator func(R, T) R, initial R) R {
	result := initial
	for _, item := range collection {
		result = accumulator(result, item)
	}
	return result
}
