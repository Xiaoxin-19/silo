package queues_test

import (
	"fmt"
	"silo/queues"
	"sync"
	"testing"
)

// QueueInterface defines the common behavior for benchmarking
type QueueInterface interface {
	Produce(val int)
	Consume() int
	ProduceBatch(vals []int)
	ConsumeBatch(maxItems int) []int
}

// ChannelWrapper wraps a native channel to match QueueInterface
type ChannelWrapper struct {
	ch chan int
}

func NewChannelWrapper(cap int) *ChannelWrapper {
	return &ChannelWrapper{ch: make(chan int, cap)}
}

func (cw *ChannelWrapper) Produce(val int) {
	cw.ch <- val
}

func (cw *ChannelWrapper) Consume() int {
	return <-cw.ch
}

func (cw *ChannelWrapper) ProduceBatch(vals []int) {
	for _, v := range vals {
		cw.ch <- v
	}
}

func (cw *ChannelWrapper) ConsumeBatch(maxItems int) []int {
	res := make([]int, 0, maxItems)
	for i := 0; i < maxItems; i++ {
		res = append(res, <-cw.ch)
	}
	return res
}

// BlockingQueueWrapper wraps our custom BlockingQueue
type BlockingQueueWrapper struct {
	bq *queues.BlockingQueue[int]
}

func NewBlockingQueueWrapper(cap int) *BlockingQueueWrapper {
	// Limit equals capacity to mimic channel behavior (blocking when full)
	return &BlockingQueueWrapper{bq: queues.NewBlockingQueue[int](cap, cap)}
}

func (bqw *BlockingQueueWrapper) Produce(val int) {
	bqw.bq.EnqueueOrWait(val)
}

func (bqw *BlockingQueueWrapper) Consume() int {
	val, _ := bqw.bq.DequeueOrWait()
	return val
}

func (bqw *BlockingQueueWrapper) ProduceBatch(vals []int) {
	bqw.bq.EnqueueBatchOrWait(vals)
}

func (bqw *BlockingQueueWrapper) ConsumeBatch(maxItems int) []int {
	return bqw.bq.DequeueBatchOrWait(maxItems)
}

func BenchmarkQueue(b *testing.B) {
	// Define different concurrency levels (Producers x Consumers)
	concurrencyLevels := []struct {
		producers int
		consumers int
	}{
		{1, 1},
		{4, 4},
		{10, 10},
		{100, 100},
	}

	// Define batch sizes to test (1 means single item)
	batchSizes := []int{1, 10, 100}

	const bufferSize = 1024

	for _, c := range concurrencyLevels {
		for _, batchSize := range batchSizes {
			groupName := fmt.Sprintf("%dP%dC/Batch-%d", c.producers, c.consumers, batchSize)

			b.Run(groupName, func(b *testing.B) {
				// 1. Benchmark Native Channel
				b.Run("Channel", func(b *testing.B) {
					q := NewChannelWrapper(bufferSize)
					if batchSize == 1 {
						runBenchmark(b, q, c.producers, c.consumers)
					} else {
						runBatchBenchmark(b, q, c.producers, c.consumers, batchSize)
					}
				})

				// 2. Benchmark BlockingQueue
				b.Run("BlockingQueue", func(b *testing.B) {
					q := NewBlockingQueueWrapper(bufferSize)
					if batchSize == 1 {
						runBenchmark(b, q, c.producers, c.consumers)
					} else {
						runBatchBenchmark(b, q, c.producers, c.consumers, batchSize)
					}
				})
			})
		}
	}
}

// runBenchmark is the driver for single item operations
func runBenchmark(b *testing.B, q QueueInterface, producers, consumers int) {
	var wg sync.WaitGroup
	wg.Add(producers + consumers)

	// totalItems is roughly b.N
	opsPerProd := b.N / producers
	if opsPerProd == 0 {
		opsPerProd = 1
	}
	
	// Recalculate exact total items to match production and consumption
	totalItems := opsPerProd * producers
	itemsPerConsumer := totalItems / consumers

	// Start Consumers
	for i := 0; i < consumers; i++ {
		count := itemsPerConsumer
		// Handle remainder for last consumer if any (though in this bench P==C mostly)
		if i == consumers-1 {
			count = totalItems - itemsPerConsumer*(consumers-1)
		}
		
		go func(c int) {
			defer wg.Done()
			for j := 0; j < c; j++ {
				q.Consume()
			}
		}(count)
	}

	// Start Producers
	b.ResetTimer()
	for i := 0; i < producers; i++ {
		go func(c int) {
			defer wg.Done()
			for j := 0; j < c; j++ {
				q.Produce(j)
			}
		}(opsPerProd)
	}

	wg.Wait()
	b.StopTimer()
}

// runBatchBenchmark is the driver for batch operations
func runBatchBenchmark(b *testing.B, q QueueInterface, producers, consumers, batchSize int) {
	var wg sync.WaitGroup
	wg.Add(producers + consumers)

	// b.N represents number of "batches" or "ops" ? 
	// Usually b.N is "number of operations". 
	// If we want to compare throughput of ITEMS, we should be careful.
	// But usually benchmark compares "time per op".
	// Here an "op" is a "batch produce" or "batch consume".
	
	opsPerProd := b.N / producers
	if opsPerProd == 0 {
		opsPerProd = 1
	}
	
	// totalBatches
	totalBatches := opsPerProd * producers
	batchesPerConsumer := totalBatches / consumers

	// Pre-allocate a batch to send
	sendBatch := make([]int, batchSize)
	for k := 0; k < batchSize; k++ {
		sendBatch[k] = k
	}

	// Start Consumers
	for i := 0; i < consumers; i++ {
		count := batchesPerConsumer
		if i == consumers-1 {
			count = totalBatches - batchesPerConsumer*(consumers-1)
		}

		go func(c int) {
			defer wg.Done()
			for j := 0; j < c; j++ {
				q.ConsumeBatch(batchSize)
			}
		}(count)
	}

	// Start Producers
	b.ResetTimer()
	for i := 0; i < producers; i++ {
		go func(c int) {
			defer wg.Done()
			for j := 0; j < c; j++ {
				q.ProduceBatch(sendBatch)
			}
		}(opsPerProd)
	}

	wg.Wait()
	b.StopTimer()
}