package seqs

import (
	"context"
	"fmt"
	"iter"
	"runtime"
	"sync"
	"sync/atomic"
)

const (
	defaultBatchSize   = 128
	defaultOrderStable = false // default to unordered for performance
)

type parallelConfig struct {
	ctx         context.Context
	batchSize   int
	workers     int
	orderStable bool
}

type ParallelOption func(*parallelConfig)

func WithContext(ctx context.Context) func(*parallelConfig) {
	return func(o *parallelConfig) {
		o.ctx = ctx
	}
}

func WithBatchSize(size int) func(*parallelConfig) {
	return func(o *parallelConfig) {
		if size < 1 {
			size = 1
		}
		o.batchSize = size
	}
}

func WithWorkers(count int) func(*parallelConfig) {
	return func(o *parallelConfig) {
		if count < 1 {
			count = 1
		}
		o.workers = count
	}
}

func WithOrderStable(stable bool) func(*parallelConfig) {
	return func(o *parallelConfig) {
		o.orderStable = stable
	}
}

// resultTuple wraps the dual return values of the transform function
// to allow storing them in a single slice for batching.
type resultTuple[R any] struct {
	res R
	err error
}

type batchJob[T any] struct {
	idx   int
	chunk *[]T
}

type batchResult[R any] struct {
	idx   int
	chunk *[]resultTuple[R]
}

// parallelMapExecutor is a helper struct to manage state for parallel execution.
type parallelMapExecutor[T, R any] struct {
	// Configuration
	ctx       context.Context
	transform func(T) (R, error)
	batchSize int
	workers   int

	// Communication
	jobs    chan batchJob[T]
	results chan batchResult[R]

	// Resources
	chunkPool *sync.Pool
	resPool   *sync.Pool
	// ensure all workers have completed
	wg sync.WaitGroup
	// ensure feeder has completed
	feederWg sync.WaitGroup
	// cancellation flag, set when consumer stops early
	canceled atomic.Bool
	// semaphore to limit the number of in-flight batches (backpressure for ordering)
	inflightSem chan struct{}
}

// -------------------------------------------------------
// Executor Implementation
// -------------------------------------------------------

func newMapExecutor[T, R any](cfg parallelConfig, transform func(T) (R, error)) *parallelMapExecutor[T, R] {
	chanSize := cfg.workers * 2
	// Limit in-flight batches to prevent unbounded memory usage in ordered mode.
	// E.g., allow buffering up to 4x workers count.
	maxInflight := cfg.workers * 4
	return &parallelMapExecutor[T, R]{
		transform:   transform,
		batchSize:   cfg.batchSize,
		workers:     cfg.workers,
		jobs:        make(chan batchJob[T], chanSize),
		results:     make(chan batchResult[R], chanSize),
		inflightSem: make(chan struct{}, maxInflight),
		chunkPool: &sync.Pool{New: func() any {
			s := make([]T, 0, cfg.batchSize)
			return &s
		}},
		resPool: &sync.Pool{New: func() any {
			s := make([]resultTuple[R], 0, cfg.batchSize)
			return &s
		}},
	}
}

// Feeder: 负责读取 Seq 并打包
func (e *parallelMapExecutor[T, R]) startFeeder(seq iter.Seq[T]) {
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

		// Flush remaining items
		if len(*chunkPtr) > 0 {
			select {
			case e.inflightSem <- struct{}{}:
			case <-e.ctx.Done():
				return
			}
			if e.sendJob(idx, chunkPtr) {
				chunkPtr = nil // transfer ownership, prevent double free
			}
		}
	}()
}

// Workers: responsible for consuming tasks
func (e *parallelMapExecutor[T, R]) startWorkers() {
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

// Closer: responsible for closing the results channel
func (e *parallelMapExecutor[T, R]) startCloser() {
	go func() {
		e.wg.Wait()
		close(e.results)
	}()
}

// Collector: collect result and yield in order
func (e *parallelMapExecutor[T, R]) collect(yield func(R, error) bool) {
	pending := make(map[int]*[]resultTuple[R])
	nextIdx := 0

	// 退出时清理暂存区
	defer func() {
		for _, chunk := range pending {
			e.putResult(chunk)
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
				if chunk, ok := pending[nextIdx]; ok {
					delete(pending, nextIdx)
					if !e.yieldChunk(chunk, yield) {
						return
					}
					<-e.inflightSem
					nextIdx++
				} else {
					break
				}
			}
		} else {
			// 乱序到达，暂存
			pending[res.idx] = res.chunk
		}
	}
}

func (e *parallelMapExecutor[T, R]) collectUnordered(yield func(R, error) bool) {
	for res := range e.results {
		if !e.yieldChunk(res.chunk, yield) {
			return
		}
		// In unordered mode, we also release token here
		<-e.inflightSem
	}
}

// -------------------------------------------------------
// Helpers
// -------------------------------------------------------

func (e *parallelMapExecutor[T, R]) processJob(job batchJob[T]) {
	// If cancelled, quickly clean up and return
	if e.ctx.Err() != nil {
		e.canceled.Store(true)
		e.putChunk(job.chunk)
		return
	}

	// Get and reset length of result slice
	count := len(*job.chunk)
	resPtr := e.getResult()
	*resPtr = (*resPtr)[:count]

	for i, v := range *job.chunk {
		if e.canceled.Load() {
			e.putResult(resPtr)
			e.putChunk(job.chunk)
			return
		}

		var r R
		var err error
		func() {
			defer func() {
				if p := recover(); p != nil {
					err = fmt.Errorf("panic in transform: %v", p)
				}
			}()
			r, err = e.transform(v)
		}()

		(*resPtr)[i] = resultTuple[R]{r, err}
	}

	e.putChunk(job.chunk)

	// 发送结果
	select {
	case e.results <- batchResult[R]{idx: job.idx, chunk: resPtr}:
	case <-e.ctx.Done():
		e.putResult(resPtr)
	}
}

func (e *parallelMapExecutor[T, R]) sendJob(idx int, chunk *[]T) bool {
	select {
	case e.jobs <- batchJob[T]{idx: idx, chunk: chunk}:
		return true
	case <-e.ctx.Done():
		return false
	}
}

func (e *parallelMapExecutor[T, R]) yieldChunk(chunk *[]resultTuple[R], yield func(R, error) bool) bool {
	defer e.putResult(chunk) // Ensure return to pool
	for _, item := range *chunk {
		if !yield(item.res, item.err) {
			e.canceled.Store(true)
			return false
		}
	}
	return true
}

// Pool Helpers Encapsulating cumbersome Cast and Reset logic
func (e *parallelMapExecutor[T, R]) getChunk() *[]T {
	ptr := e.chunkPool.Get().(*[]T)
	*ptr = (*ptr)[:0]
	return ptr
}

func (e *parallelMapExecutor[T, R]) putChunk(ptr *[]T) {
	clear(*ptr)
	e.chunkPool.Put(ptr)
}

func (e *parallelMapExecutor[T, R]) getResult() *[]resultTuple[R] {
	// Note: The length of the result slice is set by the caller based on the input length
	return e.resPool.Get().(*[]resultTuple[R])
}

func (e *parallelMapExecutor[T, R]) putResult(ptr *[]resultTuple[R]) {
	clear(*ptr)
	e.resPool.Put(ptr)
}

func runSerial[T, R any](seq iter.Seq[T], transform func(T) (R, error), ctx context.Context, yield func(R, error) bool) {
	for v := range seq {
		if ctx.Err() != nil {
			return
		}
		res, err := transform(v)
		if !yield(res, err) {
			return
		}
	}
}

// BatchMap applies transform to each element of seq concurrently using batching.
// Options can be provided to customize context, batch size, whether to preserve order, and number of workers.
// batchSize determines how many elements are bundled into a single task to reduce channel overhead.
// Recommended batchSize is between 64 and 1024 depending on the workload.
// if WithOrderStable(true) is set, the output order will match the input order, otherwise results may arrive out of order for better performance.
func BatchMap[T, R any](seq iter.Seq[T], transform func(T) (R, error), opts ...ParallelOption) iter.Seq2[R, error] {
	// 1. Config Setup
	cfg := parallelConfig{
		ctx:         context.Background(),
		batchSize:   defaultBatchSize,
		workers:     runtime.GOMAXPROCS(0),
		orderStable: defaultOrderStable,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	return func(yield func(R, error) bool) {
		// 2. Serial Fallback (Fast Path)
		if cfg.workers < 2 || cfg.batchSize < 1 {
			runSerial(seq, transform, cfg.ctx, yield)
			return
		}

		// 3. Parallel Execution (Slow Path)
		exec := newMapExecutor(cfg, transform)

		// bind context with cancel
		ctx, cancel := context.WithCancel(cfg.ctx)
		exec.ctx = ctx

		// Propagate context cancellation to atomic flag for fast-path checks
		go func() {
			<-ctx.Done()
			exec.canceled.Store(true)
		}()

		// Ensure all goroutines are cleaned up on exit
		defer func() {
			cancel()
			exec.feederWg.Wait()
		}()

		// start all components
		exec.startWorkers()
		exec.startFeeder(seq)
		// async close results channel, to close collector when all workers done
		exec.startCloser()

		switch cfg.orderStable {
		case true:
			exec.collect(yield)
		case false:
			exec.collectUnordered(yield)
		}
	}
}

// ParallelMap applies transform to each element of seq in parallel using the specified number of workers.
// It returns a Seq2 that yields the transformed elements along with any error encountered during processing.
// If workers <= 0, it defaults to 1.
func ParallelMap[T, R any](ctx context.Context, seq iter.Seq[T], transform func(T) (R, error), workers int) iter.Seq2[R, error] {
	if workers <= 0 {
		workers = 1
	}
	type resultType struct {
		result R
		err    error
	}

	return func(yield func(R, error) bool) {
		stopCh := make(chan struct{})
		resultCh := make(chan resultType, workers*3)
		sem := make(chan struct{}, workers)
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
							var zero R
							select {
							case resultCh <- resultType{zero, fmt.Errorf("panic: %v", r)}:
							case <-ctx.Done():
							case <-stopCh:
							}
						}
					}()
					res, err := transform(val)

					select {
					case resultCh <- resultType{res, err}:
					case <-ctx.Done():
					case <-stopCh:
					}
				}(v)
			}
		}()

		for v := range resultCh {
			if !yield(v.result, v.err) {
				stopOnce.Do(func() {
					close(stopCh)
				})
				return
			}
		}
	}
}
