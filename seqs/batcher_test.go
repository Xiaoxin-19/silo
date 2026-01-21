package seqs_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"silo/queues"
	"silo/seqs"
)

// TestBatcher_Basic verifies simple batching behavior and draining on queue close.
func TestBatcher_Basic(t *testing.T) {
	q := queues.NewNotifyQueue[int](100, 100)
	var processedCount atomic.Int32
	var batchCount atomic.Int32

	handler := func(ctx context.Context, batch []int) error {
		batchCount.Add(1)
		processedCount.Add(int32(len(batch)))
		return nil
	}

	// BatchSize 10
	b := seqs.NewBatcher[int](q, handler, seqs.WithBatcherSize[int](10), seqs.WithInterval[int](100*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		b.Run(ctx)
	}()

	// Enqueue 25 items
	for i := 0; i < 25; i++ {
		q.EnqueueOrWait(i)
	}

	// Close queue to trigger drain
	q.Close()

	// Wait for batcher to finish
	wg.Wait()
	cancel()

	if processedCount.Load() != 25 {
		t.Errorf("Expected 25 processed items, got %d", processedCount.Load())
	}
	// Expected batches: 10, 10, 5 -> 3 batches
	if batchCount.Load() != 3 {
		t.Errorf("Expected 3 batches, got %d", batchCount.Load())
	}
}

// TestBatcher_PeriodicFlush verifies that items are flushed after interval even if batch is not full.
func TestBatcher_PeriodicFlush(t *testing.T) {
	q := queues.NewNotifyQueue[int](100, 100)
	received := make(chan []int, 1)

	handler := func(ctx context.Context, batch []int) error {
		received <- batch
		return nil
	}

	// Interval 50ms, BatchSize 10
	b := seqs.NewBatcher[int](q, handler, seqs.WithBatcherSize[int](10), seqs.WithInterval[int](50*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go b.Run(ctx)

	// Enqueue 1 item
	q.EnqueueOrWait(1)

	select {
	case batch := <-received:
		if len(batch) != 1 {
			t.Errorf("Expected batch size 1, got %d", len(batch))
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Timeout waiting for periodic flush")
	}
}

// TestBatcher_ContextCancel_FlushBuffer verifies that cancelling context flushes the internal buffer.
func TestBatcher_ContextCancel_FlushBuffer(t *testing.T) {
	q := queues.NewNotifyQueue[int](100, 100)
	var processedCount atomic.Int32

	handler := func(ctx context.Context, batch []int) error {
		processedCount.Add(int32(len(batch)))
		return nil
	}

	// Large interval to prevent periodic flush
	b := seqs.NewBatcher[int](q, handler, seqs.WithBatcherSize[int](10), seqs.WithInterval[int](time.Hour))

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		b.Run(ctx)
		close(done)
	}()

	// Enqueue 5 items (less than batch size 10)
	for i := 0; i < 5; i++ {
		q.EnqueueOrWait(i)
	}

	// Give dispatcher time to pick up items into buffer
	time.Sleep(50 * time.Millisecond)

	// Cancel context
	cancel()
	<-done

	if processedCount.Load() != 5 {
		t.Errorf("Expected 5 items flushed on shutdown, got %d", processedCount.Load())
	}
}

// TestBatcher_WorkerPanic verifies that panics in workers are recovered and reported.
func TestBatcher_WorkerPanic(t *testing.T) {
	q := queues.NewNotifyQueue[string](10, 10)

	errCh := make(chan error, 1)
	monitor := &testMonitor[string]{
		onWorkerError: func(ctx context.Context, err error, batch []string) {
			errCh <- err
		},
	}

	handler := func(ctx context.Context, batch []string) error {
		for _, s := range batch {
			if s == "panic" {
				panic("boom")
			}
		}
		return nil
	}

	b := seqs.NewBatcher[string](q, handler,
		seqs.WithMonitor[string](monitor),
		seqs.WithBatcherSize[string](1), // Ensure immediate processing
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.Run(ctx)

	q.EnqueueOrWait("panic")

	select {
	case err := <-errCh:
		if err.Error() != "silo.Batcher: worker panic: boom" {
			t.Errorf("Unexpected error message: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for panic report")
	}
}

// TestBatcher_ShutdownTimeout verifies that Run waits for workers but times out if they are too slow.
func TestBatcher_ShutdownTimeout(t *testing.T) {
	q := queues.NewNotifyQueue[int](10, 10)

	// Worker sleeps 200ms
	handler := func(ctx context.Context, batch []int) error {
		time.Sleep(200 * time.Millisecond)
		return nil
	}

	errCh := make(chan error, 1)
	monitor := &testMonitor[int]{
		onWorkerError: func(ctx context.Context, err error, batch []int) {
			errCh <- err
		},
	}

	// Shutdown timeout 50ms (shorter than worker sleep)
	b := seqs.NewBatcher[int](q, handler,
		seqs.WithShutdownTimeout[int](50*time.Millisecond),
		seqs.WithMonitor[int](monitor),
		seqs.WithBatcherSize[int](1), // Process immediately
	)

	ctx, cancel := context.WithCancel(context.Background())
	go b.Run(ctx)

	q.EnqueueOrWait(1)
	time.Sleep(10 * time.Millisecond) // Ensure worker started
	cancel()                          // Trigger shutdown

	select {
	case err := <-errCh:
		if err.Error() != "silo.Batcher: shutdown timeout waiting for workers" {
			t.Errorf("Unexpected error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for shutdown timeout error")
	}
}

// TestBatcher_Concurrency_Race verifies thread safety under load.
func TestBatcher_Concurrency_Race(t *testing.T) {
	q := queues.NewNotifyQueue[int](1000, 1000)
	var processedCount atomic.Int32
	target := 10000

	handler := func(ctx context.Context, batch []int) error {
		processedCount.Add(int32(len(batch)))
		return nil
	}

	b := seqs.NewBatcher[int](q, handler,
		seqs.WithConcurrency[int](4),
		seqs.WithBatcherSize[int](50),
	)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		b.Run(ctx)
	}()

	// Multiple producers
	var prodWg sync.WaitGroup
	prodWg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			defer prodWg.Done()
			for j := 0; j < target/10; j++ {
				q.EnqueueOrWait(j)
			}
		}()
	}
	prodWg.Wait()
	q.Close() // Drain

	wg.Wait()
	cancel()

	if processedCount.Load() != int32(target) {
		t.Errorf("Expected %d processed, got %d", target, processedCount.Load())
	}
}

// FuzzBatcher fuzzes the batcher with random inputs.
func FuzzBatcher(f *testing.F) {
	f.Add(10, 100)
	f.Add(1, 1)
	f.Add(50, 10)

	f.Fuzz(func(t *testing.T, batchSize int, inputCount int) {
		if batchSize <= 0 {
			batchSize = 1
		}
		if inputCount <= 0 || inputCount > 1000 {
			return // Skip invalid or too large inputs for fuzzing speed
		}

		q := queues.NewNotifyQueue[int](inputCount, inputCount)
		var processedCount atomic.Int32

		handler := func(ctx context.Context, batch []int) error {
			processedCount.Add(int32(len(batch)))
			return nil
		}

		b := seqs.NewBatcher[int](q, handler, seqs.WithBatcherSize[int](batchSize))

		ctx, cancel := context.WithCancel(context.Background())
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.Run(ctx)
		}()

		for i := 0; i < inputCount; i++ {
			q.EnqueueOrWait(i)
		}
		q.Close()
		wg.Wait()
		cancel()

		if processedCount.Load() != int32(inputCount) {
			t.Errorf("BatchSize: %d, Input: %d. Expected %d, got %d", batchSize, inputCount, inputCount, processedCount.Load())
		}
	})
}

// --- Helpers ---

type testMonitor[T any] struct {
	onFlush       func(reason string, size int)
	onWorkerError func(ctx context.Context, err error, batch []T)
	onQueueDepth  func(size int)
}

func (m *testMonitor[T]) OnFlush(reason string, size int) {
	if m.onFlush != nil {
		m.onFlush(reason, size)
	}
}
func (m *testMonitor[T]) OnWorkerError(ctx context.Context, err error, batch []T) {
	if m.onWorkerError != nil {
		m.onWorkerError(ctx, err, batch)
	}
}
func (m *testMonitor[T]) OnQueueDepth(size int) {
	if m.onQueueDepth != nil {
		m.onQueueDepth(size)
	}
}

/*
go test -v -count=1 -fullpath=true -race -fuzz=FuzzBatcher -fuzztime=30s  -run "^(TestBatcher_Basic|TestBatcher_PeriodicFlush|TestBatcher_ContextCancel_FlushBuffer|TestBatcher_WorkerPanic|TestBatcher_ShutdownTimeout|TestBatcher_Concurrency_Race|FuzzBatcher)$" silo/seqs
*/
