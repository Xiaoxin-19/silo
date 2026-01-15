package sliceutil

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
)

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

// ==========================================
//  Concurrency (Advanced)
// ==========================================

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
	if len(collection) < 256 {
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
	numWorkers := runtime.NumCPU()
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
	if len(collection) < 256 {
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
	numWorkers := runtime.NumCPU()
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

// Chunk splits a slice into multiple chunks of specified size.
// the original slices are used for each chunk.
// The last chunk may be smaller if there are not enough elements.
func Chunk[T any](collection []T, size int) [][]T {
	if size <= 0 {
		panic("sliceutil.Chunk: size must be greater than 0")
	}
	if len(collection) == 0 {
		return [][]T{}
	}
	batchSize := (len(collection) + size - 1) / size
	_ = collection[len(collection)-1]
	res := make([][]T, 0, batchSize)
	for i := 0; i < len(collection); i += size {
		end := min(i+size, len(collection))
		res = append(res, collection[i:end])
	}
	return res
}

// ChunkCopy splits a slice into multiple chunks of specified size,
// creating new slices for each chunk.
// The last chunk may be smaller if there are not enough elements.
func ChunkCopy[T any](collection []T, size int) [][]T {
	if size <= 0 {
		panic("sliceutil.ChunkCopy: size must be greater than 0")
	}
	if len(collection) == 0 {
		return [][]T{}
	}
	batchSize := (len(collection) + size - 1) / size
	_ = collection[len(collection)-1]
	res := make([][]T, 0, batchSize)
	for i := 0; i < len(collection); i += size {
		end := min(i+size, len(collection))
		newChunk := make([]T, end-i)
		copy(newChunk, collection[i:end])
		res = append(res, newChunk)
	}
	return res
}
