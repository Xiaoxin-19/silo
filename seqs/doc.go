/*
Package seqs provides a comprehensive library for working with Go 1.23+ iterators (iter.Seq).

It is designed for high-performance stream processing and includes:

  - **Functional Transformations**: [Map], [Filter], [Reduce], [FlatMap], [Zip], etc.
  - **Concurrency Patterns**: [ParallelMap] and [ParallelFilter] for distributing work across
    goroutines with optional order preservation.
  - **Batch Processing**: [Batcher] for grouping elements by size or time interval, ideal for
    bulk database inserts or API calls.
  - **Flow Control**: [Window], [Chunk], [Take], [Skip].

# Concurrency

The package offers robust concurrency primitives. For example, [Batcher] handles backpressure,
graceful shutdown, and panic recovery automatically.

	// Example of parallel processing
	seqs.ParallelMap(ctx, input, func(item T) (R, error) {
		return process(item)
	}, workers)

# Error Handling

Many functions come in "Try" variants (e.g., [TryMap], [TryFilter]) to handle errors gracefully
within the stream. If a predicate or transformer returns an error, it is propagated to the consumer.

# Performance

The library is optimized to minimize allocations. Concurrency helpers use object pooling and
efficient synchronization to ensure low overhead even for high-throughput streams.
*/
package seqs
