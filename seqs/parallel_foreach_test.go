package seqs_test

import (
	"context"
	"errors"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"silo/seqs"
)

// TestParallelForeach_Correctness 验证基本的功能正确性：所有元素都被处理。
func TestParallelForeach_Correctness(t *testing.T) {
	count := 1000
	input := make([]int, count)
	for i := range input {
		input[i] = i
	}

	var processed atomic.Int32
	handler := func(ctx context.Context, batch []int) error {
		processed.Add(int32(len(batch)))
		return nil
	}

	seqs.BatchForeach(context.Background(), slices.Values(input), handler, seqs.WithBatcherSize[int](10))

	if processed.Load() != int32(count) {
		t.Errorf("Expected %d items, got %d", count, processed.Load())
	}
}

// TestParallelForeach_Concurrency 验证并发是否生效（耗时应显著小于串行）。
func TestParallelForeach_Concurrency(t *testing.T) {
	itemCount := 20
	batchSize := 2
	workers := 10
	sleepTime := 50 * time.Millisecond

	// 构造输入
	input := make([]int, itemCount)

	handler := func(ctx context.Context, batch []int) error {
		time.Sleep(sleepTime) // 模拟耗时操作
		return nil
	}

	start := time.Now()
	seqs.BatchForeach(context.Background(), slices.Values(input), handler,
		seqs.WithBatcherSize[int](batchSize),
		seqs.WithConcurrency[int](workers),
	)
	elapsed := time.Since(start)

	// 理论串行时间: 20 items / 2 batchSize * 50ms = 500ms
	// 理论并行时间: (10 batches / 10 workers) * 50ms = 50ms (理想情况)
	// 考虑到调度开销，设置阈值为 250ms
	maxDuration := 250 * time.Millisecond
	if elapsed > maxDuration {
		t.Errorf("Parallel execution took too long: %v (expected < %v)", elapsed, maxDuration)
	}
}

// TestParallelForeach_ErrorHandling 验证错误处理回调是否被正确调用。
func TestParallelForeach_ErrorHandling(t *testing.T) {
	input := []int{1, 2, 3, 4, 5}
	targetErr := errors.New("oops")

	var errCount atomic.Int32

	// 模拟 Handler 报错
	handler := func(ctx context.Context, batch []int) error {
		return targetErr
	}

	// 错误处理器
	errHandler := func(ctx context.Context, err error, batch []int) {
		if errors.Is(err, targetErr) {
			errCount.Add(1)
		}
	}

	// BatchSize=1 确保调用 5 次 handler
	seqs.BatchForeach(context.Background(), slices.Values(input), handler,
		seqs.WithBatcherSize[int](1),
		seqs.WithErrorHandler(errHandler),
	)

	if errCount.Load() != 5 {
		t.Errorf("Expected 5 errors, got %d", errCount.Load())
	}
}

// TestParallelForeach_Cancellation 验证 Context 取消能否终止执行。
func TestParallelForeach_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// 无限序列
	infiniteSeq := func(yield func(int) bool) {
		for i := 0; ; i++ {
			if !yield(i) {
				return
			}
		}
	}

	processedCount := atomic.Int32{}
	handler := func(ctx context.Context, batch []int) error {
		processedCount.Add(int32(len(batch)))
		if processedCount.Load() > 100 {
			cancel() // 触发取消
		}
		time.Sleep(10 * time.Millisecond)
		return nil
	}

	done := make(chan struct{})
	go func() {
		seqs.BatchForeach(ctx, infiniteSeq, handler, seqs.WithBatcherSize[int](10), seqs.WithConcurrency[int](4))
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("ParallelForeach did not return after cancellation")
	}
}

// FuzzParallelForeach 使用随机数据进行模糊测试。
func FuzzParallelForeach(f *testing.F) {
	f.Add(10, 5)
	f.Add(100, 10)
	f.Add(1000, 1)

	f.Fuzz(func(t *testing.T, count int, batchSize int) {
		if count <= 0 || count > 10000 {
			return
		}
		if batchSize <= 0 {
			batchSize = 1
		}

		input := make([]int, count)
		for i := range input {
			input[i] = i
		}

		var processed atomic.Int32
		handler := func(ctx context.Context, batch []int) error {
			processed.Add(int32(len(batch)))
			return nil
		}

		seqs.BatchForeach(context.Background(), slices.Values(input), handler, seqs.WithBatcherSize[int](batchSize))

		if processed.Load() != int32(count) {
			t.Errorf("Count mismatch: want %d, got %d", count, processed.Load())
		}
	})
}

// BenchmarkParallelForeach 性能基准测试。
func BenchmarkParallelForeach(b *testing.B) {
	count := 100_000
	input := make([]int, count)
	for i := range input {
		input[i] = i
	}

	// 模拟 CPU 密集型操作
	heavyWork := func(v int) int {
		for i := 0; i < 5000; i++ {
			v = (v + i*i) % 10000
		}
		return v
	}

	handler := func(ctx context.Context, batch []int) error {
		for _, v := range batch {
			heavyWork(v)
		}
		return nil
	}

	b.Run("Serial", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, v := range input {
				heavyWork(v)
			}
		}
	})

	b.Run("Parallel_Batch100_Workers4", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			seqs.BatchForeach(context.Background(), slices.Values(input), handler,
				seqs.WithBatcherSize[int](100),
				seqs.WithConcurrency[int](4),
			)
		}
	})

	b.Run("Parallel_Batch1000_Workers8", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			seqs.BatchForeach(context.Background(), slices.Values(input), handler,
				seqs.WithBatcherSize[int](1000),
				seqs.WithConcurrency[int](8),
			)
		}
	})
}

// -------------------------------------------------------
// ParallelForeach (Non-Batch) Tests
// -------------------------------------------------------

func TestParallelForeach_Function_Correctness(t *testing.T) {
	input := []int{1, 2, 3, 4, 5}
	var processed atomic.Int32

	seq := seqs.ParallelForeach(context.Background(), slices.Values(input), func(v int) error {
		processed.Add(1)
		return nil
	}, 2)

	count := 0
	for _, err := range seq {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		count++
	}

	if count != 5 {
		t.Errorf("expected 5 items yielded, got %d", count)
	}
	if processed.Load() != 5 {
		t.Errorf("expected 5 items processed, got %d", processed.Load())
	}
}

func TestParallelForeach_Function_Panic(t *testing.T) {
	seq := seqs.ParallelForeach(context.Background(), slices.Values([]int{1}), func(v int) error {
		panic("foreach panic")
	}, 1)

	for _, err := range seq {
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "panic: foreach panic" {
			t.Errorf("expected panic error, got %v", err)
		}
	}
}

func TestParallelForeach_Function_Break(t *testing.T) {
	seq := func(yield func(int) bool) {
		for i := 0; ; i++ {
			if !yield(i) {
				return
			}
		}
	}

	resultSeq := seqs.ParallelForeach(context.Background(), seq, func(v int) error {
		return nil
	}, 4)

	count := 0
	for range resultSeq {
		count++
		if count >= 10 {
			break
		}
	}
}

func FuzzParallelForeach_Function(f *testing.F) {
	f.Add([]byte{1, 2, 3, 4, 5}, byte(100))
	f.Fuzz(func(t *testing.T, input []byte, failVal byte) {
		handler := func(b byte) error {
			if b == failVal {
				return errors.New("mock error")
			}
			return nil
		}

		seq := seqs.ParallelForeach(context.Background(), slices.Values(input), handler, 4)

		var got []byte
		for v, _ := range seq {
			got = append(got, v)
		}
		slices.Sort(got)

		// Input is also expected output (keys)
		want := slices.Clone(input)
		slices.Sort(want)

		if !slices.Equal(got, want) {
			t.Errorf("Result mismatch. Got %v, Want %v", got, want)
		}
	})
}
