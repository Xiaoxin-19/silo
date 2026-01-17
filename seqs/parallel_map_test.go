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
	resultSeq := seqs.ParallelTryMap(seqInput, func(v int) (int, error) {
		return v * 2, nil
	}, seqs.WithWorkers(4), seqs.WithBatchSize(10), seqs.WithContext(t.Context()))

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
	resultSeq := seqs.ParallelTryMap(infiniteSeq, transform, seqs.WithContext(ctx), seqs.WithWorkers(4))

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
	seqs.ParallelTryMap(input, func(v int) (int, error) {
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
	resultSeq := seqs.ParallelTryMap(seqInput, func(v int) (int, error) {
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
		got, gotErr := collect(seqs.ParallelTryMap(slices.Values(input), transform, seqs.WithContext(t.Context()), seqs.WithBatchSize(4)))

		// 3. Compare
		if (wantErr == nil) != (gotErr == nil) {
			t.Fatalf("Error mismatch: want %v, got %v", wantErr, gotErr)
		}
		if !slices.Equal(got, want) {
			t.Fatalf("Result mismatch.\nInput len: %d\nFailVal: %d\nGot:  %v\nWant: %v", len(input), failVal, got, want)
		}
	})
}

/*
go test -v -count=1 -fullpath=true -fuzz=FuzzParallelTryMap -fuzztime 30s -run "^(TestParallelTryMap_Correctness|TestParallelTryMap_Cancellation|TestParallelTryMap_Concurrency_Speedup|TestParallelTryMap_ErrorHandling|FuzzParallelTryMap)$" silo/seqs
*/
