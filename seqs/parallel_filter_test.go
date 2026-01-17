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

func TestParallelTryFilter_Correctness(t *testing.T) {
	// 1. 准备数据
	inputSize := 1_000
	seqInput := seqs.RandomIntRange(inputSize)

	// 捕获原始数据用于验证
	original := make([]int, 0, inputSize)
	for v := range seqInput {
		original = append(original, v)
	}
	seqInput = slices.Values(original)

	// 2. 执行并行 Filter：保留偶数
	resultSeq := seqs.ParallelTryFilter(seqInput, func(v int) (bool, error) {
		return v%2 == 0, nil
	}, seqs.WithWorkers(4), seqs.WithBatchSize(10), seqs.WithContext(t.Context()))

	// 3. 收集结果
	var output []int
	for v, err := range resultSeq {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		output = append(output, v)
	}

	// 4. 验证结果（与串行逻辑对比）
	var expected []int
	for _, v := range original {
		if v%2 == 0 {
			expected = append(expected, v)
		}
	}

	if !slices.Equal(output, expected) {
		t.Errorf("Result mismatch. Got len %d, want len %d", len(output), len(expected))
	}
}

func TestParallelTryFilter_Cancellation(t *testing.T) {
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

	// Predicate：处理到第 100 个时取消 Context
	predicate := func(v int) (bool, error) {
		if v == 100 {
			cancel()
		}
		// 模拟耗时，给调度器反应时间
		time.Sleep(1 * time.Millisecond)
		return true, nil
	}

	// 运行
	for range seqs.ParallelTryFilter(infiniteSeq, predicate, seqs.WithContext(ctx), seqs.WithWorkers(4)) {
		processedCount++
	}

	// 验证：取消后应该停止，考虑到并发缓冲，可能会多处理一些，但不应无限执行
	if processedCount > 200 {
		t.Errorf("Cancellation failed: processed too many items (%d) after cancel at 100", processedCount)
	}
}

func TestParallelTryFilter_ErrorHandling(t *testing.T) {
	input := []int{1, 2, 3, 4, 5}
	targetErr := errors.New("boom")

	seqInput := slices.Values(input)

	// 只有 3 会报错
	resultSeq := seqs.ParallelTryFilter(seqInput, func(v int) (bool, error) {
		if v == 3 {
			return false, targetErr
		}
		return true, nil
	}, seqs.WithContext(t.Context()))

	foundErr := false
	var output []int

	for v, err := range resultSeq {
		if v == 3 {
			if err != targetErr {
				t.Errorf("Expected error at value 3, got %v", err)
			}
			foundErr = true
		} else {
			if err != nil {
				t.Errorf("Unexpected error at value %d: %v", v, err)
			}
			output = append(output, v)
		}
	}

	if !foundErr {
		t.Error("Did not receive expected error for value 3")
	}

	// 验证 3 之后的元素是否继续被处理（ParallelTryFilter 应该允许继续）
	expected := []int{1, 2, 4, 5} // 3 报错被捕获，未加入 output
	if !slices.Equal(output, expected) {
		t.Errorf("Output mismatch: got %v, want %v", output, expected)
	}
}

func TestParallelTryFilter_Concurrency_Speedup(t *testing.T) {
	workerCount := 10
	taskCount := 20
	taskDuration := 50 * time.Millisecond

	start := time.Now()

	input := func(yield func(int) bool) {
		for i := 0; i < taskCount; i++ {
			if !yield(i) {
				return
			}
		}
	}

	// 执行耗时任务
	seqs.ParallelTryFilter(input, func(v int) (bool, error) {
		time.Sleep(taskDuration)
		return true, nil
	}, seqs.WithWorkers(workerCount), seqs.WithBatchSize(2), seqs.WithContext(t.Context()))(func(int, error) bool {
		return true // 消费所有
	})

	elapsed := time.Since(start)

	// 理论分析：
	// 总任务 20 个，BatchSize=2 -> 10 个 Batch。
	// Workers=10 -> 10 个 Worker 并行处理 10 个 Batch。
	// 每个 Batch 内部串行 2 个任务 -> 2 * 50ms = 100ms。
	// 加上调度开销，预期应该远小于串行时间 (20 * 50ms = 1000ms)。
	expectedMax := 300 * time.Millisecond

	if elapsed > expectedMax {
		t.Errorf("Concurrency check failed: took %v, expected < %v", elapsed, expectedMax)
	}
}

func FuzzParallelTryFilter(f *testing.F) {
	f.Add([]byte{1, 2, 3, 4, 5}, byte(100)) // 无错误
	f.Add([]byte{1, 2, 3, 4, 5}, byte(3))   // 在 3 处报错

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
		// Predicate: 偶数保留，遇到 failVal 报错
		predicate := func(b byte) (bool, error) {
			if b == failVal {
				return false, errors.New("mock error")
			}
			return b%2 == 0, nil
		}

		// 辅助函数：收集结果直到遇到错误
		collect := func(t *testing.T, seq iter.Seq2[byte, error]) ([]byte, error) {
			var res []byte
			for v, err := range seq {
				if err != nil {
					return res, err // 遇到错误立即返回
				}
				res = append(res, v)
			}
			return res, nil
		}

		// 1. 基准测试：串行 TryFilter
		want, wantErr := collect(t, seqs.TryFilter(slices.Values(input), predicate))

		// 2. 目标测试：并行 ParallelTryFilter
		// 使用较小的 BatchSize 以在小数据量下也能触发并行逻辑
		got, gotErr := collect(t, seqs.ParallelTryFilter(slices.Values(input), predicate, seqs.WithContext(t.Context()), seqs.WithBatchSize(4)))

		// 3. 对比
		if (wantErr == nil) != (gotErr == nil) {
			t.Errorf("Error mismatch: want %v, got %v", wantErr, gotErr) // use t.Errorf
			return                                                       // 快速失败，防止更多错误
		}
		if !slices.Equal(got, want) {
			t.Fatalf("Result mismatch.\nInput len: %d\nFailVal: %d\nGot:  %v\nWant: %v", len(input), failVal, got, want)
		}
	})
}

/*
 go test -race -fullpath=true -v -count=1 -fuzz=FuzzParallelTryFilter -fuzztime 30s  -run "^(TestParallelTryFilter_Correctness|TestParallelTryFilter_Cancellation|TestParallelTryFilter_ErrorHandling|TestParallelTryFilter_Concurrency_Speedup|FuzzParallelTryFilter)$" silo/seqs
*/
