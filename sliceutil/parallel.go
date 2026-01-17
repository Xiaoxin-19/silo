package sliceutil

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
)

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
