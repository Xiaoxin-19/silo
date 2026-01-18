package sliceutil

import (
	"context"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
)

// ==========================================
//  Pure Functions (Happy Path)
// ==========================================

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

// Filter returns a new slice containing elements that satisfy the predicate.
func Filter[T any](collection []T, predicate func(T) bool) []T {
	if len(collection) == 0 {
		return []T{}
	}
	copySlice := make([]T, len(collection))
	copy(copySlice, collection)
	return FilterInPlace(copySlice, predicate)
}

// FilterInPlace in-place filters the slice with zero memory allocation.
// Note: It modifies the underlying array of the original slice.
func FilterInPlace[T any](collection []T, predicate func(T) bool) []T {
	// Use standard library implementation for best performance.
	// Invert predicate because DeleteFunc deletes when true, but Filter keeps when true.
	return slices.DeleteFunc(collection, func(t T) bool {
		return !predicate(t)
	})
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

// TryParallelMap is a convenience wrapper for TryParallelMapWithContext, using context.Background().
func TryParallelMap[T any, R any](collection []T, transform func(T) (R, error)) ([]R, error) {
	return TryParallelMapWithContext(context.Background(), collection, transform)
}

// TryParallelMapWithContext uses concurrency to perform mapping with error handling and context support.
// Suitable for CPU-intensive or IO-intensive tasks with large data sets.
func TryParallelMapWithContext[T any, R any](ctx context.Context, collection []T, transform func(T) (R, error)) ([]R, error) {
	// Fast path for already canceled context
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if len(collection) == 0 {
		return []R{}, nil
	}

	// Performance threshold: if the data set is too small, fall back to serial execution to avoid goroutine scheduling overhead.
	// This number depends on the specific task; here 256 is used as an empirical value.
	if !shouldRunParallel(len(collection)) {
		// Serial execution with context support
		res := make([]R, len(collection))
		for i, v := range collection {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			var err error
			res[i], err = transform(v)
			if err != nil {
				return nil, err
			}
		}
		return res, nil
	}

	res := make([]R, len(collection))

	// Determine number of workers based on CPU cores
	numWorkers := runtime.GOMAXPROCS(0)
	chunkSize := (len(collection) + numWorkers - 1) / numWorkers

	var wg sync.WaitGroup
	var firstErr error // to capture the first error
	var canceled int32 // atomic flag: 0 = running, 1 = canceled

	for i := 0; i < numWorkers; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if start >= len(collection) {
			break
		}
		if end > len(collection) {
			end = len(collection)
		}

		wg.Add(1)
		go func(s, e int) {
			defer wg.Done()
			for k := s; k < e; k++ {
				// Fast fail check
				if atomic.LoadInt32(&canceled) == 1 {
					return
				}

				select {
				case <-ctx.Done():
					if atomic.CompareAndSwapInt32(&canceled, 0, 1) {
						firstErr = ctx.Err()
					}
					return
				default:
				}

				val, err := transform(collection[k])

				if err != nil {
					// Set canceled flag
					if atomic.CompareAndSwapInt32(&canceled, 0, 1) {
						firstErr = err
					}
					return
				}
				res[k] = val
			}
		}(start, end)
	}

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}
	return res, nil
}

// TryParallelFilter is a convenience wrapper for TryParallelFilterWithContext, using context.Background().
func TryParallelFilter[T any](collection []T, predicate func(T) (bool, error)) ([]T, error) {
	return TryParallelFilterWithContext(context.Background(), collection, predicate)
}

// TryParallelFilterWithContext uses concurrency to filter with error handling and context support.
// It maintains the relative order of elements.
// Suitable for CPU-intensive or IO-intensive tasks with large data sets.
func TryParallelFilterWithContext[T any](ctx context.Context, collection []T, predicate func(T) (bool, error)) ([]T, error) {
	// Fast path for already canceled context
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if len(collection) == 0 {
		return []T{}, nil
	}

	// Performance threshold: if the data set is too small, fall back to serial execution to avoid goroutine scheduling overhead.
	if !shouldRunParallel(len(collection)) {
		// Serial execution with context support
		// Use len(collection) as capacity to avoid reallocation for small sets
		res := make([]T, 0, len(collection))
		for _, v := range collection {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			keep, err := predicate(v)
			if err != nil {
				return nil, err
			}
			if keep {
				res = append(res, v)
			}
		}
		return res, nil
	}

	// Determine number of workers based on CPU cores
	numWorkers := runtime.GOMAXPROCS(0)
	chunkSize := (len(collection) + numWorkers - 1) / numWorkers

	var wg sync.WaitGroup
	var firstErr error // to capture the first error
	var canceled int32 // atomic flag: 0 = running, 1 = canceled
	// Use a slice of slices to collect results from each worker independently
	localSlices := make([][]T, numWorkers)

	for i := 0; i < numWorkers; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if start >= len(collection) {
			break
		}
		if end > len(collection) {
			end = len(collection)
		}

		wg.Add(1)
		go func(workerID, s, e int) {
			defer wg.Done()
			// Pre-allocate assuming 50% pass rate to reduce allocations
			localRes := make([]T, 0, (e-s)/2)

			for k := s; k < e; k++ {
				if atomic.LoadInt32(&canceled) == 1 {
					return
				}
				select {
				case <-ctx.Done():
					if atomic.CompareAndSwapInt32(&canceled, 0, 1) {
						firstErr = ctx.Err()
					}
					return
				default:
				}

				keep, err := predicate(collection[k])
				if err != nil {
					if atomic.CompareAndSwapInt32(&canceled, 0, 1) {
						firstErr = err
					}
					return
				}
				if keep {
					localRes = append(localRes, collection[k])
				}
			}
			localSlices[workerID] = localRes
		}(i, start, end)
	}

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	// Merge results
	totalLen := 0
	for _, s := range localSlices {
		totalLen += len(s)
	}
	res := make([]T, 0, totalLen)
	for _, s := range localSlices {
		res = append(res, s...)
	}

	return res, nil
}
