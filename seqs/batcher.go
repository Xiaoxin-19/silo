package seqs

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"
)

// 默认配置
const (
	DefaultBatchSize       = 100
	DefaultInterval        = 1 * time.Second
	DefaultWorkers         = 1 // default to single worker
	DefaultShutdownTimeout = 5 * time.Second
)

// BatchHandler defines the business logic for handling a batch of elements.
type BatchHandler[T any] func(context.Context, []T) error

// BatchErrHandler defines a callback for handling errors during batch processing.
type BatchErrHandler[T any] func(context.Context, error, []T)

// BatcherQueue defines the interface for the queue used by Batcher.
type BatcherQueue[T any] interface {
	Done() <-chan struct{}
	Ready() <-chan struct{}
	Size() int
	TryDequeueBatchInto(dst []T) int
}

type BatcherOption[T any] func(*BatcherConfig[T])

type BatcherMonitor[T any] interface {
	// reason: "timeout", "full", "shutdown", "periodic"
	OnFlush(reason string, size int)
	// OnWorkerError records an error occurring during batch processing.
	OnWorkerError(ctx context.Context, err error, batch []T)
	OnQueueDepth(size int)
}

type NoopBatcherMonitor[T any] struct{}

func (n *NoopBatcherMonitor[T]) OnFlush(reason string, size int)                         {}
func (n *NoopBatcherMonitor[T]) OnWorkerError(ctx context.Context, err error, batch []T) {}
func (n *NoopBatcherMonitor[T]) OnQueueDepth(size int)                                   {}

// teeBatcherMonitor wraps a monitor and an error handler, ensuring both are called.
type teeBatcherMonitor[T any] struct {
	monitor BatcherMonitor[T]
	handler BatchErrHandler[T]
}

func (t *teeBatcherMonitor[T]) OnFlush(reason string, size int) {
	t.monitor.OnFlush(reason, size)
}
func (t *teeBatcherMonitor[T]) OnWorkerError(ctx context.Context, err error, batch []T) {
	t.monitor.OnWorkerError(ctx, err, batch)
	t.handler(ctx, err, batch)
}
func (t *teeBatcherMonitor[T]) OnQueueDepth(size int) {
	t.monitor.OnQueueDepth(size)
}

type batcherJob[T any] struct {
	ctx   context.Context
	items []T
}

type Batcher[T any] struct {
	// --- Basic Configuration ---
	queue           BatcherQueue[T]
	batchSize       int
	interval        time.Duration
	shutdownTimeout time.Duration
	// --- Concurrency Control ---
	workerNum int                // Worker number
	jobCh     chan batcherJob[T] // Job channel
	wg        sync.WaitGroup     // Wait for Worker exit

	// --- Error Handling ---
	// processing
	handler BatchHandler[T]
	monitor BatcherMonitor[T]
}

type BatcherConfig[T any] struct {
	batchSize       int
	interval        time.Duration
	concurrency     int
	monitor         BatcherMonitor[T]
	shutdownTimeout time.Duration
	errorHandler    BatchErrHandler[T]
}

func WithBatcherSize[T any](size int) BatcherOption[T] {
	return func(cfg *BatcherConfig[T]) {
		cfg.batchSize = size
	}
}

func WithInterval[T any](interval time.Duration) BatcherOption[T] {
	return func(cfg *BatcherConfig[T]) {
		cfg.interval = interval
	}
}

func WithConcurrency[T any](workers int) BatcherOption[T] {
	return func(cfg *BatcherConfig[T]) {
		cfg.concurrency = workers
	}
}

// WithMonitor sets the monitor for the batcher.
func WithMonitor[T any](m BatcherMonitor[T]) BatcherOption[T] {
	return func(cfg *BatcherConfig[T]) {
		cfg.monitor = m
	}
}

func WithErrorHandler[T any](handler BatchErrHandler[T]) BatcherOption[T] {
	if handler == nil {
		panic("silo.WithErrorHandler: error handler cannot be nil")
	}
	return func(cfg *BatcherConfig[T]) {
		cfg.errorHandler = handler
	}
}

func WithShutdownTimeout[T any](timeout time.Duration) BatcherOption[T] {
	return func(cfg *BatcherConfig[T]) {
		cfg.shutdownTimeout = timeout
	}
}

func NewBatcher[T any](queue BatcherQueue[T], handler BatchHandler[T], opts ...BatcherOption[T]) *Batcher[T] {
	if handler == nil {
		panic("silo.NewBatcher: handler cannot be nil")
	}
	// default config
	cfg := &BatcherConfig[T]{
		batchSize:       DefaultBatchSize,
		interval:        DefaultInterval,
		concurrency:     DefaultWorkers,
		shutdownTimeout: DefaultShutdownTimeout,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	// Validate config to prevent runtime panic
	if cfg.interval <= 0 {
		cfg.interval = DefaultInterval
	}

	// Setup monitor
	if cfg.monitor == nil {
		// Default to silent (Noop) to avoid polluting stdout in library code
		cfg.monitor = &NoopBatcherMonitor[T]{}
	}

	// If ErrorHandler is provided, wrap the monitor to ensure it's called
	if cfg.errorHandler != nil {
		cfg.monitor = &teeBatcherMonitor[T]{
			monitor: cfg.monitor,
			handler: cfg.errorHandler,
		}
	}

	// Prevent deadlock if concurrency is set to <= 0
	if cfg.concurrency < 1 {
		cfg.concurrency = 1
	}

	batcher := &Batcher[T]{
		batchSize:       cfg.batchSize,
		interval:        cfg.interval,
		workerNum:       cfg.concurrency,
		handler:         handler,
		monitor:         cfg.monitor,
		queue:           queue,
		jobCh:           make(chan batcherJob[T], cfg.concurrency*2),
		shutdownTimeout: cfg.shutdownTimeout,
	}
	return batcher
}

func (b *Batcher[T]) Run(ctx context.Context) {
	for i := 0; i < b.workerNum; i++ {
		b.wg.Add(1) // Fix: Add before starting goroutine to avoid race condition
		go b.workerLoop()
	}

	// start dispatcher
	b.dispatcherLoop(ctx)

	// all done, close jobCh to signal workers to exit
	close(b.jobCh)

	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(b.shutdownTimeout):
		b.monitor.OnWorkerError(context.Background(), fmt.Errorf("silo.Batcher: shutdown timeout waiting for workers"), nil)
	}
}

// Securely execute handler, catch panic and call back error handler
func (b *Batcher[T]) executeSafe(ctx context.Context, batch []T) {
	defer func() {
		if r := recover(); r != nil {
			err, ok := r.(error)
			if !ok {
				err = fmt.Errorf("silo.Batcher: worker panic: %v\nStack: %s", r, debug.Stack())
			}
			b.monitor.OnWorkerError(ctx, err, batch)
		}
	}()
	if err := b.handler(ctx, batch); err != nil {
		select {
		case <-ctx.Done():
			return
		default:
		}
		b.monitor.OnWorkerError(ctx, err, batch)
	}
}

func (b *Batcher[T]) sendJobs(jobCtx context.Context, items []T, reason string) bool {
	if len(items) == 0 {
		return true
	}
	b.monitor.OnFlush(reason, len(items))

	select {
	case b.jobCh <- batcherJob[T]{ctx: jobCtx, items: items}:
		return true
	case <-jobCtx.Done():
		// If the Job's Context is done (timeout or cancel) before sending, discard and log
		b.monitor.OnWorkerError(jobCtx, fmt.Errorf("silo.Batcher: context done before sending batch (reason: %s): %w", reason, jobCtx.Err()), items)
		return false
	}
}

func (b *Batcher[T]) dispatcherLoop(ctx context.Context) {
	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()

	buffer := make([]T, 0, b.batchSize)

	// Internal helper: Flush current buffer
	flush := func(ctx context.Context, reason string) {
		if len(buffer) > 0 {
			b.sendJobs(ctx, buffer, reason)
			buffer = make([]T, 0, b.batchSize)
		}
	}

	for {
		select {
		case <-ctx.Done():
			// 1. Stop receiving new data (no longer pulling from queue).
			// 2. Try to send a buffer already in memory (Best Effort).
			// 3. If ctx has timed out/cancelled, sendJobs will fail, we accept this lost,
			//    Because the upstream (like MQ) should be responsible for retrying, or the client will retry.

			// Fix: Use a separate timeout context for shutdown flush to avoid immediate failure due to cancelled ctx
			shutdownCtx, cancel := context.WithTimeout(context.Background(), b.shutdownTimeout)
			defer cancel()
			flush(shutdownCtx, "shutdown")
			return

		case <-b.queue.Done():

			// Queue Close means that the upstream clearly has no more data, and we need to deal with all the backlog
			flush(ctx, "queue_closed")
			for {
				// Fill the current buffer
				n := b.queue.TryDequeueBatchInto(buffer[len(buffer):cap(buffer)])
				if n == 0 {
					break
				}
				buffer = buffer[:len(buffer)+n]
				// flush will send the jobs and allocate a NEW buffer for the next iteration
				flush(ctx, "drain")
			}
			return

		case <-ticker.C:
			// Record queue depth periodically
			b.monitor.OnQueueDepth(b.queue.Size())
			// Time's up, force send (even if not full)
			flush(ctx, "periodic")

		case <-b.queue.Ready():
			// Direct copy into the free space of the buffer
			n := b.queue.TryDequeueBatchInto(buffer[len(buffer):cap(buffer)])
			buffer = buffer[:len(buffer)+n]

			// If full, send and reset
			if len(buffer) >= b.batchSize {
				flush(ctx, "full")
				// Reset ticker, avoid too frequent ticks after batch sent
				ticker.Reset(b.interval)
			}
		}
	}
}

func (b *Batcher[T]) workerLoop() {
	defer b.wg.Done()
	for job := range b.jobCh {
		b.executeSafe(job.ctx, job.items)
	}
}
