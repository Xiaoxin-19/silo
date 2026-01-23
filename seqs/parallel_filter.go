package seqs

import (
	"context"
	"fmt"
	"iter"
	"runtime"
	"sync"
	"sync/atomic"
)

// parallelFilterExecutor is a helper struct to manage state for parallel execution.
type parallelFilterExecutor[T any] struct {
	// Configuration
	ctx       context.Context
	predicate func(T) (bool, error)
	batchSize int
	workers   int

	// Communication
	jobs    chan batchJob[T]
	results chan batchResult[T]

	// Resources
	chunkPool *sync.Pool
	resPool   *sync.Pool
	// ensure all workers and closer have exited
	wg sync.WaitGroup
	// ensure feeder has exited
	feederWg sync.WaitGroup
	// cancellation flag, set when consumer stops early
	canceled atomic.Bool
	// semaphore to limit the number of in-flight batches (backpressure for ordering)
	inflightSem chan struct{}
}

// -------------------------------------------------------
// Executor Implementation
// -------------------------------------------------------

func newFilterExecutor[T any](cfg parallelConfig, predicate func(T) (bool, error)) *parallelFilterExecutor[T] {
	chanSize := cfg.workers * 2
	// Limit in-flight batches to prevent unbounded memory usage.
	// E.g., allow buffering up to 4x workers count.
	maxInflight := cfg.workers * 4
	return &parallelFilterExecutor[T]{
		predicate:   predicate,
		batchSize:   cfg.batchSize,
		workers:     cfg.workers,
		jobs:        make(chan batchJob[T], chanSize),
		results:     make(chan batchResult[T], chanSize),
		inflightSem: make(chan struct{}, maxInflight),
		chunkPool: &sync.Pool{New: func() any {
			s := make([]T, 0, cfg.batchSize)
			return &s
		}},
		resPool: &sync.Pool{New: func() any {
			s := make([]resultTuple[T], 0, cfg.batchSize)
			return &s
		}},
	}
}

func (e *parallelFilterExecutor[T]) startFeeder(seq iter.Seq[T]) {
	e.feederWg.Add(1)
	go func() {
		defer e.feederWg.Done()
		defer close(e.jobs)

		idx := 0
		chunkPtr := e.getChunk()

		defer func() {
			if chunkPtr != nil {
				e.putChunk(chunkPtr)
			}
		}()

		for v := range seq {
			*chunkPtr = append(*chunkPtr, v)
			if len(*chunkPtr) == e.batchSize {
				// Acquire token before sending job to limit in-flight batches
				select {
				case e.inflightSem <- struct{}{}:
				case <-e.ctx.Done():
					return
				}

				if !e.sendJob(idx, chunkPtr) {
					return
				}
				idx++
				chunkPtr = e.getChunk()
			}
			if e.canceled.Load() {
				return
			}
		}
		if len(*chunkPtr) > 0 {
			select {
			case e.inflightSem <- struct{}{}:
			case <-e.ctx.Done():
				return
			}
			if e.sendJob(idx, chunkPtr) {
				chunkPtr = nil // 所有权移交
			}
		}
	}()
}

func (e *parallelFilterExecutor[T]) startWorkers() {
	// Start workers
	for i := 0; i < e.workers; i++ {
		e.wg.Add(1)
		go func() {
			defer e.wg.Done()
			for job := range e.jobs {
				e.processJob(job)
			}
		}()
	}
}

func (e *parallelFilterExecutor[T]) startCloser() {
	// Start closer
	go func() {
		e.wg.Wait()
		close(e.results)
	}()
}

func (e *parallelFilterExecutor[T]) collect(yield func(T, error) bool) {
	pending := make(map[int]*[]resultTuple[T])
	nextIdx := 0

	// 退出时清理暂存区
	defer func() {
		for _, chunkPtr := range pending {
			e.putResult(chunkPtr)
		}
	}()

	for res := range e.results {
		if res.idx == nextIdx {
			// 1. 处理当前包
			if !e.yieldChunk(res.chunk, yield) {
				return
			}
			// Release token after the chunk is yielded (consumed)
			<-e.inflightSem
			nextIdx++

			// 2. 检查缓冲区的后续包
			for {
				if chunkPtr, ok := pending[nextIdx]; ok {
					delete(pending, nextIdx)
					if !e.yieldChunk(chunkPtr, yield) {
						return
					}
					<-e.inflightSem
					nextIdx++
				} else {
					break
				}
			}
		} else {
			pending[res.idx] = res.chunk
		}
	}
}

// -------------------------------------------------------
// Helpers
// -------------------------------------------------------

func (e *parallelFilterExecutor[T]) processJob(job batchJob[T]) {
	if e.ctx.Err() != nil {
		e.canceled.Store(true)
		e.putChunk(job.chunk)
		return
	}
	resPtr := e.getResult()
	for _, v := range *job.chunk {
		if e.canceled.Load() {
			e.putResult(resPtr)
			e.putChunk(job.chunk)
			return
		}
		keep, err := e.predicate(v)
		if err != nil {
			// On error, yield the element with error
			*resPtr = append(*resPtr, resultTuple[T]{res: v, err: err})
		} else if keep {
			*resPtr = append(*resPtr, resultTuple[T]{res: v, err: nil})
		}
	}

	// Recycle input chunk
	e.putChunk(job.chunk)

	select {
	case e.results <- batchResult[T]{idx: job.idx, chunk: resPtr}:
	case <-e.ctx.Done():
		e.putResult(resPtr)
		return
	}
}

func (e *parallelFilterExecutor[T]) sendJob(idx int, chunk *[]T) bool {
	select {
	case e.jobs <- batchJob[T]{idx: idx, chunk: chunk}:
		return true
	case <-e.ctx.Done():
		return false
	}
}

func (e *parallelFilterExecutor[T]) yieldChunk(chunk *[]resultTuple[T], yield func(T, error) bool) bool {
	defer e.putResult(chunk) // 确保归还
	for _, item := range *chunk {
		if !yield(item.res, item.err) {
			e.canceled.Store(true)
			return false
		}
	}
	return true
}

// Pool Helpers (Encapsulating cumbersome Cast and Reset logic)
func (e *parallelFilterExecutor[T]) getChunk() *[]T {
	ptr := e.chunkPool.Get().(*[]T)
	*ptr = (*ptr)[:0]
	return ptr
}

func (e *parallelFilterExecutor[T]) putChunk(ptr *[]T) {
	clear(*ptr)
	e.chunkPool.Put(ptr)
}

func (e *parallelFilterExecutor[T]) getResult() *[]resultTuple[T] {
	ptr := e.resPool.Get().(*[]resultTuple[T])
	*ptr = (*ptr)[:0]
	return ptr
}

func (e *parallelFilterExecutor[T]) putResult(ptr *[]resultTuple[T]) {
	clear(*ptr)
	e.resPool.Put(ptr)
}

func runFilterSerial[T any](seq iter.Seq[T], predicate func(T) (bool, error), ctx context.Context, yield func(T, error) bool) {
	for v := range seq {
		if ctx.Err() != nil {
			return
		}
		keep, err := predicate(v)
		if err != nil {
			// error occurred: yield the error along with the element 'v' that caused it
			if !yield(v, err) {
				return
			}
			continue
		}
		if keep {
			if !yield(v, nil) {
				return
			}
		}
	}
}

// BatchFilter filters elements of seq concurrently using batching.
// batchSize determines how many elements are bundled into a single task to reduce channel overhead.
// Recommended batchSize is between 64 and 1024 depending on the workload.
func BatchFilter[T any](seq iter.Seq[T], predicate func(T) (bool, error), opts ...ParallelOption) iter.Seq2[T, error] {
	cfg := parallelConfig{
		ctx:       context.Background(),
		batchSize: defaultBatchSize,
		workers:   runtime.GOMAXPROCS(0),
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return func(yield func(T, error) bool) {
		numWorkers := cfg.workers
		// Fallback to serial if single core or invalid batch size
		if numWorkers < 2 || cfg.batchSize < 1 {
			runFilterSerial(seq, predicate, cfg.ctx, yield)
			return
		}

		// Set up executor
		ctx, cancel := context.WithCancel(cfg.ctx)
		executor := newFilterExecutor(cfg, predicate)
		executor.ctx = ctx

		// Propagate context cancellation to atomic flag for fast-path checks
		go func() {
			<-ctx.Done()
			executor.canceled.Store(true)
		}()

		defer func() {
			cancel()
			executor.feederWg.Wait()
		}()

		// Start pipeline
		executor.startFeeder(seq)
		executor.startWorkers()
		executor.startCloser()

		// Collect and reorder
		executor.collect(yield)
	}
}

// ParallelFilter filters elements of seq concurrently.
// It returns a Seq2 that yields the original elements along with any error encountered during processing.
// If workers <= 0, it defaults to 1.
func ParallelFilter[T any](ctx context.Context, seq iter.Seq[T], predicate func(T) (bool, error), workers int) iter.Seq2[T, error] {
	if workers <= 0 {
		workers = 1
	}
	type resultType struct {
		result T
		err    error
	}

	return func(yield func(T, error) bool) {
		sem := make(chan struct{}, workers)
		resultCh := make(chan resultType, workers*2)
		stopCh := make(chan struct{})
		var stopOnce sync.Once
		var wg sync.WaitGroup

		go func() {
			defer func() {
				wg.Wait()
				close(resultCh)
				close(sem)
			}()

			for v := range seq {
				select {
				case <-ctx.Done():
					return
				case <-stopCh:
					return
				case sem <- struct{}{}:
				}
				wg.Add(1)
				go func(val T) {
					defer func() {
						wg.Done()
						<-sem
						if r := recover(); r != nil {
							var zero T
							select {
							case resultCh <- resultType{zero, fmt.Errorf("panic: %v", r)}:
							case <-ctx.Done():
							case <-stopCh:
							}
						}
					}()
					keep, err := predicate(val)
					if err != nil {
						select {
						case resultCh <- resultType{val, err}:
						case <-ctx.Done():
						case <-stopCh:
						}
					} else if keep {
						select {
						case resultCh <- resultType{val, nil}:
						case <-ctx.Done():
						case <-stopCh:
						}
					}
				}(v)
			}
		}()

		for v := range resultCh {
			if !yield(v.result, v.err) {
				stopOnce.Do(func() {
					close(stopCh)
				})
				break
			}
		}
	}
}
