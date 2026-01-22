package seqs_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"silo/queues"
	"silo/seqs"
)

// 1. 添加全局变量，防止编译器优化
var resultSink int

// heavyCalc simulates a CPU intensive operation.
// Copied from benchmark_test.go to ensure independence if packages differ.
func heavyCalc(x int) int {
	for i := 0; i < 1000; i++ {
		x = (x + i*i) % 10000
	}
	return x
}

// --- Payloads ---

type PayloadTiny int64

type PayloadMedium struct {
	Data [128]byte
}

// --- Benchmark 1: Overhead Matrix ---
// Tests the framework overhead across different dimensions (Payload Size, Batch Size, Concurrency).
// Handler is No-Op.
func BenchmarkBatcher_Overhead_Matrix(b *testing.B) {
	batchSizes := []int{10, 100, 1000}
	workers := []int{1, 4, 8, 16, 32}

	// 1. Tiny Payload (8 bytes) - CPU/Scheduling Bound
	b.Run("Payload=Tiny", func(b *testing.B) {
		for _, bs := range batchSizes {
			for _, w := range workers {
				runOverheadComparison[PayloadTiny](b, bs, w, func(i int) PayloadTiny { return PayloadTiny(i) })
			}
		}
	})

	// 2. Medium Payload (128 bytes) - Memory Copy Bound
	b.Run("Payload=Medium", func(b *testing.B) {
		for _, bs := range batchSizes {
			for _, w := range workers {
				runOverheadComparison[PayloadMedium](b, bs, w, func(i int) PayloadMedium { return PayloadMedium{} })
			}
		}
	})
}

func runOverheadComparison[T any](b *testing.B, batchSize int, workers int, factory func(int) T) {
	name := fmt.Sprintf("BatchSize=%d/Workers=%d", batchSize, workers)
	b.Run(name, func(b *testing.B) {
		// Baseline: Raw Channel (Manual Batching)
		// Producer sends T, Consumer aggregates []T.
		// High channel overhead (1 send per item).
		b.Run("RawChan_Batch", func(b *testing.B) {
			ch := make(chan T, 1024)
			var wg sync.WaitGroup
			wg.Add(workers)

			for i := 0; i < workers; i++ {
				go func() {
					defer wg.Done()
					buffer := make([]T, 0, batchSize)
					for item := range ch {
						buffer = append(buffer, item)
						if len(buffer) >= batchSize {
							buffer = buffer[:0]
						}
					}
				}()
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ch <- factory(i)
			}
			close(ch)
			wg.Wait()
		})

		// Baseline: Raw Channel (Pre-batched)
		// Producer aggregates []T, sends []T.
		// Low channel overhead (1 send per batch).
		// This represents the theoretical "Speed of Light" for channel-based passing.
		b.Run("RawChan_PreBatched", func(b *testing.B) {
			ch := make(chan []T, 1024)
			var wg sync.WaitGroup
			wg.Add(workers)

			for i := 0; i < workers; i++ {
				go func() {
					defer wg.Done()
					for range ch {
						// Consumer just receives the batch
						// No-op processing
					}
				}()
			}

			b.ResetTimer()
			// Producer loop needs to simulate batching locally
			buffer := make([]T, 0, batchSize)
			for i := 0; i < b.N; i++ {
				buffer = append(buffer, factory(i))
				if len(buffer) >= batchSize {
					// Send a copy or the slice itself.
					// To be fair with Batcher which allocates new buffers, we send the slice.
					// In a real tight loop, we might reuse, but here we simulate "producing a batch".
					ch <- buffer
					buffer = make([]T, 0, batchSize)
				}
			}
			close(ch)
			wg.Wait()
		})

		// Target: Silo Batcher
		b.Run("Silo_Batcher", func(b *testing.B) {
			q := queues.NewNotifyQueue[T](1024, 1024)
			handler := func(ctx context.Context, batch []T) error {
				return nil
			}

			batcher := seqs.NewBatcher(q, handler,
				seqs.WithBatcherSize[T](batchSize),
				seqs.WithConcurrency[T](workers),
			)

			ctx, cancel := context.WithCancel(context.Background())
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				batcher.Run(ctx)
			}()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				q.EnqueueOrWait(factory(i))
			}
			q.Close()
			wg.Wait()
			cancel()
		})

		// Target: Silo Batcher (Pre-batched Producer)
		// Producer aggregates []T locally and calls EnqueueBatchOrWait.
		// Reduces lock contention on the queue significantly.
		b.Run("Silo_Batcher_PreBatched", func(b *testing.B) {
			q := queues.NewNotifyQueue[T](1024, 1024)
			handler := func(ctx context.Context, batch []T) error {
				return nil
			}

			batcher := seqs.NewBatcher(q, handler,
				seqs.WithBatcherSize[T](batchSize),
				seqs.WithConcurrency[T](workers),
			)

			ctx, cancel := context.WithCancel(context.Background())
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				batcher.Run(ctx)
			}()

			b.ResetTimer()

			// Producer loop simulating batching locally
			buffer := make([]T, 0, batchSize)
			for i := 0; i < b.N; i++ {
				buffer = append(buffer, factory(i))
				if len(buffer) >= batchSize {
					q.EnqueueBatchOrWait(buffer...)
					buffer = buffer[:0] // Reuse buffer memory since queue copies data
				}
			}
			if len(buffer) > 0 {
				q.EnqueueBatchOrWait(buffer...)
			}
			q.Close()
			wg.Wait()
			cancel()
		})
	})
}

// --- Benchmark 2: CPU Scaling ---
// Tests how well the Batcher scales with different computational workloads.
func BenchmarkBatcher_Workload_Scaling(b *testing.B) {
	const batchSize = 100
	workersList := []int{1, 4, 8, 16}

	workloads := []struct {
		name string
		work func(int)
	}{
		{
			name: "Light", // Simple arithmetic
			work: func(v int) { _ = v * 2 },
		},
		{
			name: "Medium", // Moderate loop (100 iterations)
			work: func(v int) {
				for i := 0; i < 100; i++ {
					v = (v + i*i) % 10000
				}
			},
		},
		{
			name: "Heavy", // Heavy loop (1000 iterations)
			work: func(v int) {
				// Batcher 适合处理 >100µs 的任务，2.6µs 对它来说还是太轻了，全是调度开销。
				// 我们循环 50 次，把单次任务撑到 ~130µs
				for i := 0; i < 50; i++ {
					// 将结果赋值给全局变量，防止编译器优化 (Dead Code Elimination)
					resultSink = heavyCalc(v)
				}
			},
		},
	}

	for _, wl := range workloads {
		b.Run(wl.name, func(b *testing.B) {
			for _, workers := range workersList {
				b.Run(fmt.Sprintf("Workers=%d", workers), func(b *testing.B) {
					q := queues.NewNotifyQueue[int](1024, 1024)
					handler := func(ctx context.Context, batch []int) error {
						for _, v := range batch {
							wl.work(v)
						}
						return nil
					}

					batcher := seqs.NewBatcher(q, handler,
						seqs.WithBatcherSize[int](batchSize),
						seqs.WithConcurrency[int](workers),
					)

					ctx, cancel := context.WithCancel(context.Background())
					var wg sync.WaitGroup
					wg.Add(1)
					go func() {
						defer wg.Done()
						batcher.Run(ctx)
					}()

					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						q.EnqueueOrWait(i)
					}
					q.Close()
					wg.Wait()
					cancel()
				})
			}
		})
	}
}
