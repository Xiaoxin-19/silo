package seqs_test

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"silo/seqs"
)

// TestChannelQueue_Integration verifies that ChannelQueue works correctly with Batcher.
// It reuses the logic from TestBatcher_Basic but feeds data via a channel.
func TestChannelQueue_Integration(t *testing.T) {
	// 1. Setup Input Channel
	inputCh := make(chan int, 10)

	// 2. Create Adapter
	q := seqs.NewChannelQueue(inputCh, 10)

	// 3. Setup Batcher (Reusing logic from TestBatcher_Basic)
	var processedCount atomic.Int32
	var batchCount atomic.Int32

	handler := func(ctx context.Context, batch []int) error {
		batchCount.Add(1)
		processedCount.Add(int32(len(batch)))
		return nil
	}

	// BatchSize 10, Interval 100ms
	b := seqs.NewBatcher[int](q, handler, seqs.WithBatcherSize[int](10), seqs.WithInterval[int](100*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		b.Run(ctx)
	}()

	// 4. Feed data via channel (Producer)
	itemCount := 25
	go func() {
		for i := 0; i < itemCount; i++ {
			inputCh <- i
		}
		// Closing the input channel should trigger q.Close() inside the adapter,
		// which in turn signals the Batcher to drain and exit.
		close(inputCh)
	}()

	// 5. Wait for batcher to finish
	wg.Wait()
	cancel()

	// 6. Verify results
	if processedCount.Load() != int32(itemCount) {
		t.Errorf("Expected %d processed items, got %d", itemCount, processedCount.Load())
	}
	// Expected batches: 10, 10, 5 -> 3 batches
	if batchCount.Load() != 3 {
		t.Errorf("Expected 3 batches, got %d", batchCount.Load())
	}
}

// TestFetcherQueue_Integration verifies that FetcherQueue works correctly with Batcher.
// It simulates a pull-based data source.
func TestFetcherQueue_Integration(t *testing.T) {
	// 1. Setup Mock Fetcher
	itemCount := 25
	currentIndex := 0
	var mu sync.Mutex

	fetcher := func(ctx context.Context) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		if currentIndex >= itemCount {
			return 0, io.EOF // Signal end of data
		}
		val := currentIndex
		currentIndex++
		return val, nil
	}

	// 2. Create Adapter
	q := seqs.NewFetcherQueue(context.Background(), fetcher, 10)

	// 3. Setup Batcher
	var processedCount atomic.Int32
	var batchCount atomic.Int32

	handler := func(ctx context.Context, batch []int) error {
		batchCount.Add(1)
		processedCount.Add(int32(len(batch)))
		return nil
	}

	b := seqs.NewBatcher[int](q, handler, seqs.WithBatcherSize[int](10), seqs.WithInterval[int](100*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		b.Run(ctx)
	}()

	// 4. Wait for batcher to finish (triggered by io.EOF from fetcher)
	wg.Wait()
	cancel()

	// 5. Verify results
	if processedCount.Load() != int32(itemCount) {
		t.Errorf("Expected %d processed items, got %d", itemCount, processedCount.Load())
	}
	if batchCount.Load() != 3 {
		t.Errorf("Expected 3 batches, got %d", batchCount.Load())
	}
}

// TestFetcherQueue_ContextCancel verifies that cancelling the context stops the fetcher loop.
func TestFetcherQueue_ContextCancel(t *testing.T) {
	// Infinite fetcher that respects context
	fetcher := func(ctx context.Context) (int, error) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(10 * time.Millisecond):
			return 1, nil
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	q := seqs.NewFetcherQueue(ctx, fetcher, 10)

	// Let it run for a bit
	time.Sleep(50 * time.Millisecond)

	// Cancel context
	cancel()

	// Wait for queue to close
	select {
	case <-q.Done():
		// Success: Queue closed
	case <-time.After(1 * time.Second):
		t.Fatal("FetcherQueue did not close after context cancellation")
	}
}

// TestFetcherQueue_Error verifies that an error from the fetcher closes the queue.
func TestFetcherQueue_Error(t *testing.T) {
	expectedErr := errors.New("fetch error")
	fetcher := func(ctx context.Context) (int, error) {
		return 0, expectedErr
	}

	q := seqs.NewFetcherQueue(context.Background(), fetcher, 10)

	select {
	case <-q.Done():
		// Success: Queue closed
	case <-time.After(1 * time.Second):
		t.Fatal("FetcherQueue did not close on fetcher error")
	}
}
