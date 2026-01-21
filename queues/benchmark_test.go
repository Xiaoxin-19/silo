package queues_test

import (
	"fmt"
	"silo/queues"
	"sync"
	"testing"
)

// ==========================================
// 1. Data Payloads (Variable A: Payload Size)
// ==========================================

// Tiny: 8 Bytes (int64)
type PayloadTiny int64

// Medium: 128 Bytes
type PayloadMedium struct {
	Data [128]byte
}

// Large: 1KB (1024 Bytes)
type PayloadLarge struct {
	Data [1024]byte
}

// ==========================================
// 2. Queue Modes (Variable B: Transport Mode)
// ==========================================

// BenchmarkInterface defines the interaction
type BenchDriver[T any] interface {
	Setup(capacity int)
	Produce(items []T)
	Consume(count int)
}

// --- Mode 1: Item Queue (One by One) ---
// Uses NotifyQueue[T]. Calls EnqueueOrWait for each item.
type ModeItem[T any] struct {
	q *queues.NotifyQueue[T]
}

func (m *ModeItem[T]) Setup(capacity int) {
	m.q = queues.NewNotifyQueue[T](capacity, capacity)
}
func (m *ModeItem[T]) Produce(items []T) {
	// Simulate loop of single items (High lock contention)
	for _, item := range items {
		m.q.EnqueueOrWait(item)
	}
}
func (m *ModeItem[T]) Consume(count int) {
	for i := 0; i < count; i++ {
		m.q.DequeueOrWait()
	}
}

// --- Mode 2: Batch Copy Queue (Copy Data) ---
// Uses NotifyQueue[T]. Calls EnqueueBatchOrWait.
// Data is copied from input slice into internal queue array.
type ModeBatchCopy[T any] struct {
	q *queues.NotifyQueue[T]
}

func (m *ModeBatchCopy[T]) Setup(capacity int) {
	m.q = queues.NewNotifyQueue[T](capacity, capacity)
}
func (m *ModeBatchCopy[T]) Produce(items []T) {
	// One lock acquisition, but O(N) memory copy
	m.q.EnqueueBatchOrWait(items...)
}
func (m *ModeBatchCopy[T]) Consume(count int) {
	// We consume in batches to match the production logic for throughput
	// Assuming consumer also wants batch efficiency
	// Note: 'count' here is total items.
	// We need to consume until we get 'count' items.
	remaining := count
	buf := make([]T, 128) // Use a fixed buffer for consumption
	for remaining > 0 {
		toRead := len(buf)
		if remaining < toRead {
			toRead = remaining
		}
		n := m.q.DequeueBatchOrWait(buf[:toRead])
		remaining -= n
	}
}

// --- Mode 3: Slice Pointer Queue (Zero Copy) ---
// Uses NotifyQueue[[]T]. The "Item" in the queue is the slice header itself.
type ModeSlicePointer[T any] struct {
	q *queues.NotifyQueue[[]T]
}

func (m *ModeSlicePointer[T]) Setup(capacity int) {
	// Capacity here is "number of slices", not "number of T items"
	// To be fair, if we treat capacity as "buffer size", this queue can hold 'capacity' batches.
	m.q = queues.NewNotifyQueue[[]T](capacity, capacity)
}
func (m *ModeSlicePointer[T]) Produce(items []T) {
	// Enqueue the slice header as a single unit (O(1) copy)
	// IMPORTANT: In real code, 'items' ownership is transferred!
	m.q.EnqueueOrWait(items)
}
func (m *ModeSlicePointer[T]) Consume(count int) {
	// Here 'count' is total items.
	// But in this mode, one Dequeue = one batch of items.
	// We need to know the batch size to know how many dequeues to do?
	// Or we just consume 'ops' times.

	// For benchmarking, Produce is called with a 'batch'.
	// So 1 Produce call = 1 Dequeue call here.
	// But the interface says 'Consume(count int)'.
	// To make it work, we assume Consume is called with number of *Batches* for this mode,
	// or we adjust the driver.

	// Let's adjust the driver logic to be simpler:
	// The benchmark loop N is "Number of Produce calls".
	m.q.DequeueOrWait()
}

// ==========================================
// 3. Benchmark Runner
// ==========================================

func BenchmarkNotifyQueue(b *testing.B) {
	// Matrix: Payload Types
	// Tiny: 8, Medium: 128, Large: 1024

	b.Run("Tiny(8B)", func(b *testing.B) { runModes[PayloadTiny](b, 8) })
	b.Run("Medium(128B)", func(b *testing.B) { runModes[PayloadMedium](b, 128) })
	b.Run("Large(1KB)", func(b *testing.B) { runModes[PayloadLarge](b, 1024) })
}

func runModes[T any](b *testing.B, itemSize int64) {
	concurrencies := []struct{ P, C int }{{1, 1}, {10, 10}, {100, 100}}
	batchSizes := []int{10, 100, 1000}

	for _, c := range concurrencies {
		for _, batch := range batchSizes {
			// Calculate total bytes per Op (one batch)
			bytesPerOp := itemSize * int64(batch)

			group := fmt.Sprintf("%dP%dC/BatchSize-%d", c.P, c.C, batch)

			b.Run(group, func(b *testing.B) {
				// 1. Item Mode (Loop)
				// Only test for small batches to avoid timeout
				if batch <= 100 {
					b.Run("Mode=Item", func(b *testing.B) {
						b.SetBytes(bytesPerOp)
						driver := &ModeItem[T]{}
						runBench[T](b, driver, c.P, c.C, batch, false)
					})
				}

				// 2. Batch Copy Mode
				b.Run("Mode=BatchCopy", func(b *testing.B) {
					b.SetBytes(bytesPerOp)
					driver := &ModeBatchCopy[T]{}
					runBench[T](b, driver, c.P, c.C, batch, false)
				})

				// 3. Slice Pointer Mode
				b.Run("Mode=SlicePtr", func(b *testing.B) {
					b.SetBytes(bytesPerOp)
					driver := &ModeSlicePointer[T]{}
					runBench[T](b, driver, c.P, c.C, batch, true)
				})
			})
		}
	}
}

func runBench[T any](b *testing.B, driver BenchDriver[T], P, C int, batchSize int, isSliceMode bool) {
	// Setup Queue with enough buffer to avoid blocking bias.
	// We want to measure the "Queue Operation Cost", not the "Wait for Consumer Cost".
	// Base capacity in terms of "Batches"
	const baseBatchCapacity = 1024

	if isSliceMode {
		driver.Setup(baseBatchCapacity)
	} else {
		// For Item queues, capacity must be items * batchSize
		driver.Setup(baseBatchCapacity * batchSize)
	}

	var wg sync.WaitGroup
	wg.Add(P + C)

	// b.N is total number of Batches
	opsPerProd := b.N / P
	if opsPerProd == 0 {
		opsPerProd = 1
	}

	totalBatches := opsPerProd * P
	batchesPerCons := totalBatches / C
	remainder := totalBatches % C

	payload := make([]T, batchSize)

	b.ResetTimer()

	// Consumers
	for i := 0; i < C; i++ {
		n := batchesPerCons
		if i < remainder {
			n++
		}

		go func(batchCount int) {
			defer wg.Done()
			if isSliceMode {
				for j := 0; j < batchCount; j++ {
					driver.Consume(1)
				}
			} else {
				// For Item/BatchCopy mode, consume exact item count
				driver.Consume(batchCount * batchSize)
			}
		}(n)
	}

	// Producers
	for i := 0; i < P; i++ {
		go func(count int) {
			defer wg.Done()
			for j := 0; j < count; j++ {
				driver.Produce(payload)
			}
		}(opsPerProd)
	}

	wg.Wait()
	b.StopTimer()
}
