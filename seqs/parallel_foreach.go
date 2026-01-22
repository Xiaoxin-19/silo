package seqs

import (
	"context"
	"iter"
)

// ParallelForeach processes elements from seq in parallel batches using the provided BatchHandler.
// It utilizes a Batcher internally to manage batching and concurrency.
// Options can be provided to customize the Batcher's behavior.
func ParallelForeach[T any](ctx context.Context, seq iter.Seq[T], h BatchHandler[T], options ...BatcherOption[T]) {
	q := NewSeqQueue(seq, 4096)
	batcher := NewBatcher(q, h, options...)
	batcher.Run(ctx)
}
