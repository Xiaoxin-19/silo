package seqs_test

import (
	"context"
	"errors"
	"iter"
	"math/rand"
	"silo/seqs"
	"slices"
	"testing"
	"time"
)

func TestParallelTryMap_Correctness(t *testing.T) {
	// 1. 准备数据：1000 个整数
	inputSize := 1_000
	seqInput := seqs.RandomIntRange(inputSize)

	original := make([]int, 0, inputSize)
	for v := range seqInput {
		original = append(original, v)
	}
	// 2. 重置 seqInput，因为上面的循环已经消费掉它
	seqInput = func(yield func(int) bool) {
		for _, v := range original {
			if !yield(v) {
				return
			}
		}
	}
	// 3. 执行并行 Map：简单的 x * 2
	resultSeq := seqs.BatchMap(seqInput, func(v int) (int, error) {
		return v * 2, nil
	}, seqs.WithWorkers(4), seqs.WithBatchSize(10), seqs.WithContext(t.Context()), seqs.WithOrderStable(true))

	// 4. 收集结果
	var output []int
	for v, err := range resultSeq {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		output = append(output, v)
	}

	// 5. 验证长度
	if len(output) != inputSize {
		t.Errorf("expected length %d, got %d", inputSize, len(output))
	}

	// 6. 验证顺序和值
	for i, v := range output {
		expected := original[i] * 2
		if v != expected {
			t.Errorf("at index %d: expected %d, got %d", i, expected, v)
		}
	}
}

func TestParallelTryMap_Unordered_Correctness(t *testing.T) {
	// 1. 准备数据
	inputSize := 1_000
	seqInput := seqs.RandomIntRange(inputSize)

	original := make([]int, 0, inputSize)
	for v := range seqInput {
		original = append(original, v)
	}
	seqInput = slices.Values(original)

	// 2. 执行并行 Map (显式无序)
	// 引入随机延迟以打破顺序
	resultSeq := seqs.BatchMap(seqInput, func(v int) (int, error) {
		if v%2 == 0 {
			time.Sleep(100 * time.Microsecond)
		}
		return v * 2, nil
	}, seqs.WithWorkers(4), seqs.WithBatchSize(10), seqs.WithContext(t.Context()), seqs.WithOrderStable(false))

	// 3. 收集结果
	var output []int
	for v, err := range resultSeq {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		output = append(output, v)
	}

	// 4. 验证结果 (排序后对比)
	slices.Sort(output)
	// 构造期望值并排序 (original 已经是 0..N 随机，所以需要重新计算期望值并排序)
	expected := make([]int, len(original))
	for i, v := range original {
		expected[i] = v * 2
	}
	slices.Sort(expected)

	if !slices.Equal(output, expected) {
		t.Errorf("Result mismatch (sorted comparison). Got len %d, want len %d", len(output), len(expected))
	}
}

func TestParallelTryMap_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	// 无限序列
	infiniteSeq := func(yield func(int) bool) {
		for i := 0; ; i++ {
			if !yield(i) {
				return
			}
		}
	}

	processedCount := 0

	// 处理函数：处理到第 100 个时取消 Context
	transform := func(v int) (int, error) {
		if v == 100 {
			cancel() // <--- 触发取消
		}
		// 模拟一点耗时，让调度器有机会响应
		time.Sleep(1 * time.Millisecond)
		return v, nil
	}

	// 运行
	resultSeq := seqs.BatchMap(infiniteSeq, transform, seqs.WithContext(ctx), seqs.WithWorkers(4))

	for _, err := range resultSeq {
		if err != nil {
			t.Logf("err: %s", err)
		}
		processedCount++
	}

	// 验证：我们取消后，它不应该无限跑下去
	// 由于并发 batch 的存在，可能会多处理一些（buffer size），但肯定会停。
	if processedCount > 200 {
		t.Errorf("Cancellation failed: processed too many items (%d) after cancel at 100", processedCount)
	}
}

func TestParallelTryMap_Concurrency_Speedup(t *testing.T) {
	workerCount := 10
	taskCount := 20
	taskDuration := 100 * time.Millisecond

	start := time.Now()

	// 构造 Seq
	input := func(yield func(int) bool) {
		for i := 0; i < taskCount; i++ {
			if !yield(i) {
				return
			}
		}
	}

	// 执行耗时任务
	seqs.BatchMap(input, func(v int) (int, error) {
		time.Sleep(taskDuration) // 模拟耗时
		return v, nil
	}, seqs.WithWorkers(workerCount), seqs.WithBatchSize(2), seqs.WithContext(t.Context()))(func(int, error) bool {
		return true // 消费所有
	})

	elapsed := time.Since(start)

	// 理论最快时间计算：
	// 总任务 20 个，BatchSize 为 2 -> 共 10 个 Batch。
	// Worker 为 10 -> 10 个 Worker 正好并行处理 10 个 Batch。
	// 每个 Batch 内部串行处理 2 个任务 -> 2 * 100ms = 200ms。
	// 所以理论上最优时间约为 200ms。放宽点，取 210ms。
	expected := 210 * time.Millisecond

	if elapsed > expected {
		t.Errorf("Concurrency check failed: took %v, expected %v", elapsed, expected)
	}

}

func TestParallelTryMap_ErrorHandling(t *testing.T) {
	input := []int{1, 2, 3, 4, 5}
	targetErr := errors.New("boom")

	seqInput := slices.Values(input)

	// 只有 3 会报错
	resultSeq := seqs.BatchMap(seqInput, func(v int) (int, error) {
		if v == 3 {
			return 0, targetErr
		}
		return v * 2, nil
	}, seqs.WithContext(t.Context()))

	foundErr := false
	idx := 0
	for res, err := range resultSeq {
		expectedVal := input[idx] * 2

		if input[idx] == 3 {
			if err != targetErr {
				t.Errorf("Expected error at index %d, got nil", idx)
			}
			foundErr = true
		} else {
			if err != nil {
				t.Errorf("Unexpected error at index %d: %v", idx, err)
			}
			if res != expectedVal {
				t.Errorf("Value mismatch at index %d", idx)
			}
		}
		idx++
	}

	if !foundErr {
		t.Error("Did not verify the error for input 3")
	}
}

func FuzzParallelTryMap(f *testing.F) {
	// 1. Seed Corpus
	f.Add([]byte{1, 2, 3, 4, 5}, byte(100)) // No error
	f.Add([]byte{1, 2, 3, 4, 5}, byte(3))   // Error at 3

	largeData := make([]byte, 1000)
	for i := range largeData {
		largeData[i] = byte(i % 255)
	}
	f.Add(largeData, byte(250))

	// Add random seeds
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 5; i++ {
		data := make([]byte, rng.Intn(100))
		rng.Read(data)
		f.Add(data, byte(rng.Intn(256)))
	}

	f.Fuzz(func(t *testing.T, input []byte, failVal byte) {
		// Transform: double the value, error if equals failVal
		transform := func(b byte) (byte, error) {
			if b == failVal {
				return 0, errors.New("mock error")
			}
			return b * 2, nil
		}

		// Helper to collect seq results (stop on first error)
		collect := func(seq iter.Seq2[byte, error]) ([]byte, error) {
			var res []byte
			for v, err := range seq {
				if err != nil {
					return res, err
				}
				res = append(res, v)
			}
			return res, nil
		}

		// 1. Serial Baseline
		want, wantErr := collect(seqs.TryMap(slices.Values(input), transform))

		// 2. Parallel Target
		// Use small batch size to trigger batching logic even with small inputs
		got, gotErr := collect(seqs.BatchMap(slices.Values(input), transform, seqs.WithContext(t.Context()), seqs.WithBatchSize(4), seqs.WithOrderStable(true)))

		// 3. Compare
		if (wantErr == nil) != (gotErr == nil) {
			t.Fatalf("Error mismatch: want %v, got %v", wantErr, gotErr)
		}
		if !slices.Equal(got, want) {
			t.Fatalf("Result mismatch.\nInput len: %d\nFailVal: %d\nGot:  %v\nWant: %v", len(input), failVal, got, want)
		}
	})
}

func FuzzParallelTryMap_Unordered(f *testing.F) {
	f.Add([]byte{1, 2, 3, 4, 5}, byte(100))
	f.Add([]byte{1, 2, 3, 4, 5}, byte(3))

	f.Fuzz(func(t *testing.T, input []byte, failVal byte) {
		transform := func(b byte) (byte, error) {
			if b == failVal {
				return 0, errors.New("mock error")
			}
			return b * 2, nil
		}

		collect := func(seq iter.Seq2[byte, error]) ([]byte, error) {
			var res []byte
			for v, err := range seq {
				if err != nil {
					return res, err
				}
				res = append(res, v)
			}
			return res, nil
		}

		// 1. Serial Baseline (Ordered)
		want, wantErr := collect(seqs.TryMap(slices.Values(input), transform))

		// 2. Parallel Target (Unordered)
		got, gotErr := collect(seqs.BatchMap(slices.Values(input), transform, seqs.WithContext(t.Context()), seqs.WithBatchSize(4), seqs.WithOrderStable(false)))

		// 3. Compare Errors
		if (wantErr == nil) != (gotErr == nil) {
			t.Fatalf("Error mismatch: want %v, got %v", wantErr, gotErr)
		}

		// 4. Compare Values (Sort first)
		slices.Sort(got)
		slices.Sort(want)

		if !slices.Equal(got, want) {
			t.Fatalf("Result mismatch.\nInput len: %d\nFailVal: %d\nGot:  %v\nWant: %v", len(input), failVal, got, want)
		}
	})
}

func BenchmarkParallelTryMap_Order_Comparison(b *testing.B) {
	count := 1_000_000
	input := make([]int, count)
	for i := range input {
		input[i] = i
	}

	// 模拟一定的 CPU 负载，使并行有意义
	work := func(v int) (int, error) {
		for i := 0; i < 1000; i++ {
			v = (v + i*i) % 10000
		}
		return v, nil
	}

	b.Run("Ordered", func(b *testing.B) {
		b.SetBytes(int64(count * 8)) // 8 bytes per int (64-bit)
		for i := 0; i < b.N; i++ {
			for range seqs.BatchMap(slices.Values(input), work, seqs.WithOrderStable(true)) {
			}
		}
	})

	b.Run("Unordered", func(b *testing.B) {
		b.SetBytes(int64(count * 8))
		for i := 0; i < b.N; i++ {
			for range seqs.BatchMap(slices.Values(input), work, seqs.WithOrderStable(false)) {
			}
		}
	})
}

// -------------------------------------------------------
// ParallelMap (Non-Batch) Tests
// -------------------------------------------------------

func TestParallelMap_Function_Correctness(t *testing.T) {
	input := []int{1, 2, 3, 4, 5}
	// ParallelMap is inherently unordered due to concurrency
	seq := seqs.ParallelMap(context.Background(), slices.Values(input), func(v int) (int, error) {
		return v * 2, nil
	}, 2)

	var got []int
	for v, err := range seq {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got = append(got, v)
	}
	slices.Sort(got)
	expected := []int{2, 4, 6, 8, 10}
	if !slices.Equal(got, expected) {
		t.Errorf("got %v, want %v", got, expected)
	}
}

func TestParallelMap_Function_Panic(t *testing.T) {
	seq := seqs.ParallelMap(context.Background(), slices.Values([]int{1}), func(v int) (int, error) {
		panic("test panic")
	}, 1)

	for _, err := range seq {
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "panic: test panic" {
			t.Errorf("expected panic error, got %v", err)
		}
	}
}

func TestParallelMap_Function_Break(t *testing.T) {
	// Infinite sequence
	seq := func(yield func(int) bool) {
		for i := 0; ; i++ {
			if !yield(i) {
				return
			}
		}
	}

	ctx := context.Background()
	resultSeq := seqs.ParallelMap(ctx, seq, func(v int) (int, error) {
		return v, nil
	}, 4)

	count := 0
	for range resultSeq {
		count++
		if count >= 10 {
			break
		}
	}
	// Test passes if it doesn't hang
}

func FuzzParallelMap_Function(f *testing.F) {
	f.Add([]byte{1, 2, 3, 4, 5}, byte(100))
	f.Add([]byte{1, 2, 3, 4, 5}, byte(3))

	f.Fuzz(func(t *testing.T, input []byte, failVal byte) {
		transform := func(b byte) (byte, error) {
			if b == failVal {
				return 0, errors.New("mock error")
			}
			return b * 2, nil
		}

		// ParallelMap (Unordered)
		gotSeq := seqs.ParallelMap(context.Background(), slices.Values(input), transform, 4)

		var got []byte
		for v, _ := range gotSeq {
			// ParallelMap yields results even if other items failed, we just collect values here
			got = append(got, v)
		}
		// Since ParallelMap output includes zero values on error/panic, strict comparison is complex in Fuzz.
		// We mainly ensure it doesn't crash.
	})
}

/*
go test -v -count=1 -fullpath=true -fuzz=FuzzParallelTryMap -fuzztime 30s -run "^(TestParallelTryMap_Correctness|TestParallelTryMap_Cancellation|TestParallelTryMap_Concurrency_Speedup|TestParallelTryMap_ErrorHandling|FuzzParallelTryMap)$" silo/seqs
go test -v -count=1 -fullpath=true -fuzz=FuzzParallelMap_Function -fuzztime 30s -run "^(TestParallelMap_Function_Correctness|TestParallelMap_Function_Panic|TestParallelMap_Function_Break|FuzzParallelMap_Function)$" silo/seqs
*/
