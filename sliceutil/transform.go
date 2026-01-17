package sliceutil

import (
	"runtime"
)

const minParallelThreshold = 256

// shouldRunParallel decides whether to execute logic concurrently.
// Design Rationale:
//  1. Threshold: Small collections don't justify the overhead of goroutine scheduling.
//  2. GOMAXPROCS: In containerized environments (K8s), NumCPU() reflects the host,
//     while GOMAXPROCS reflects the effective quota. We must use GOMAXPROCS.
//     Per Go docs, GOMAXPROCS is dynamic, so we query it every time.
func shouldRunParallel(n int) bool {
	return n >= minParallelThreshold && runtime.GOMAXPROCS(0) > 1
}

// ==========================================
//  Pure Functions (Happy Path)
// ==========================================

// filter
func Filter[T any](collection []T, predicate func(T) bool) []T {
	if len(collection) == 0 {
		return []T{}
	}
	// BCE hint: avoid bounds check in loop
	_ = collection[len(collection)-1]

	// Heuristic pre-allocation of capacity
	res := make([]T, 0, len(collection)/2)
	for _, v := range collection {
		if predicate(v) {
			res = append(res, v)
		}
	}
	return res
}

// FilterInPlace in-place filters the slice with zero memory allocation.
// Note: It modifies the underlying array of the original slice.
func FilterInPlace[T any](collection []T, predicate func(T) bool) []T {
	if len(collection) == 0 {
		return collection
	}
	_ = collection[len(collection)-1]

	idx := 0
	for i, v := range collection {
		if predicate(v) {
			if i != idx {
				collection[idx] = v
			}
			idx++
		}
	}

	// allow GC to reclaim memory
	clear(collection[idx:])

	return collection[:idx]
}

// Map transforms a slice of type T to a slice of type R.
func Map[T any, R any](collection []T, transform func(T) R) []R {
	if len(collection) == 0 {
		return []R{}
	}
	// BCE hint: avoid bounds check in loop
	_ = collection[len(collection)-1]

	// Pre-allocate result slice
	res := make([]R, len(collection))
	for i, v := range collection {
		res[i] = transform(v)
	}
	return res
}

// Reduce reduces a slice of type T to a single value of type R.
func Reduce[T any, R any](collection []T, accumulator func(R, T) R, initial R) R {
	if len(collection) == 0 {
		return initial
	}

	// BCE hint: avoid bounds check in loop
	_ = collection[len(collection)-1]

	result := initial
	for _, item := range collection {
		result = accumulator(result, item)
	}
	return result
}

// ==========================================
// Try Functions (Error Handling)
// Suitable for scenarios where errors may occur (Fail Fast)
// ==========================================

// TryFilter similar to Filter, but predicate may return an error.
// Returns immediately upon encountering an error.
func TryFilter[T any](collection []T, predicate func(T) (bool, error)) ([]T, error) {
	if len(collection) == 0 {
		return []T{}, nil
	}
	_ = collection[len(collection)-1]

	res := make([]T, 0, len(collection)/2)
	for _, v := range collection {
		ok, err := predicate(v)
		if err != nil {
			return nil, err
		}
		if ok {
			res = append(res, v)
		}
	}
	return res, nil
}

// TryFilterInPlace similar to FilterInPlace, but supports error handling.
func TryFilterInPlace[T any](collection []T, predicate func(T) (bool, error)) ([]T, error) {
	if len(collection) == 0 {
		return collection, nil
	}
	_ = collection[len(collection)-1]

	idx := 0
	for i, v := range collection {
		ok, err := predicate(v)
		if err != nil {
			// Early return on error
			return nil, err
		}
		if ok {
			if i != idx {
				collection[idx] = v
			}
			idx++
		}
	}

	clear(collection[idx:])
	return collection[:idx], nil
}

// TryMap similar to Map, but transform may return an error.
func TryMap[T any, R any](collection []T, transform func(T) (R, error)) ([]R, error) {
	if len(collection) == 0 {
		return []R{}, nil
	}
	// BCE hint: avoid bounds check in loop
	_ = collection[len(collection)-1]

	res := make([]R, len(collection))
	for i, v := range collection {
		var err error
		res[i], err = transform(v)
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}

// TryReduce similar to Reduce, but accumulator may return an error.
func TryReduce[T any, R any](collection []T, accumulator func(R, T) (R, error), initial R) (R, error) {
	if len(collection) == 0 {
		return initial, nil
	}
	// BCE hint: avoid bounds check in loop
	_ = collection[len(collection)-1]

	result := initial
	for _, item := range collection {
		var err error
		result, err = accumulator(result, item)
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

// GroupBy groups elements of the collection based on the keySelector function.
// the result is a map where keys are the results of keySelector and values are slices of elements that correspond to each key.
// the slice order is preserved from the original collection.
func GroupBy[T any, K comparable](collection []T, keySelector func(T) K) map[K][]T {
	if keySelector == nil {
		panic("sliceutil.GroupBy: keySelector is nil")
	}

	if len(collection) == 0 {
		return map[K][]T{}
	}

	// use len(collection)/2 as a heuristic for initial map size
	result := make(map[K][]T, len(collection)/2)

	// BCE hint: avoid bounds check in loop
	_ = collection[len(collection)-1]
	for _, item := range collection {
		key := keySelector(item)
		result[key] = append(result[key], item)
	}
	return result
}

// Partition splits the collection into two slices based on the predicate.
// The first slice contains elements that satisfy the predicate,
// while the second slice contains elements that do not.
// The order of elements in both slices is preserved from the original collection.
// The return slices are newly allocated with exact capacity.
//
// Performance Note: This function uses a two-pass approach to minimize allocations.
// Therefore, the predicate MUST be a pure function (no side effects), as it will be executed twice for each element.
func Partition[T any](collection []T, predicate func(T) bool) (matched []T, unmatched []T) {
	if len(collection) == 0 {
		return []T{}, []T{}
	}
	if predicate == nil {
		panic("sliceutil.Partition: predicate is nil")
	}
	_ = collection[len(collection)-1]
	matchCnt := 0
	for _, item := range collection {
		if predicate(item) {
			matchCnt++
		}
	}
	matched = make([]T, matchCnt)
	unmatched = make([]T, len(collection)-matchCnt)
	matchedIdx := 0
	unmatchedIdx := 0
	for _, item := range collection {
		if predicate(item) {
			matched[matchedIdx] = item
			matchedIdx++
		} else {
			unmatched[unmatchedIdx] = item
			unmatchedIdx++
		}
	}
	return matched, unmatched
}

// PartitionInPlace splits the collection into two slices based on the predicate,
// The first slice contains elements that satisfy the predicate,
// while the second slice contains elements that do not.
//
// Note:
//   - The order of elements in both slices is not guaranteed to be preserved.
//   - It modifies the underlying array of the original slice.
func PartitionInPlace[T any](collection []T, predicate func(T) bool) (matched []T, unmatched []T) {
	if len(collection) == 0 {
		return collection, collection[:0]
	}
	if predicate == nil {
		panic("sliceutil.PartitionInPlace: predicate is nil")
	}
	colLen := len(collection)
	_ = collection[colLen-1]
	matchedIdx := 0
	unmatchedIdx := colLen - 1

	for matchedIdx <= unmatchedIdx {
		if predicate(collection[matchedIdx]) {
			matchedIdx++
		} else {
			collection[matchedIdx], collection[unmatchedIdx] = collection[unmatchedIdx], collection[matchedIdx]
			unmatchedIdx--
		}
	}
	return collection[:matchedIdx], collection[matchedIdx:]
}
