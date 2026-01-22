package seqs

import (
	"context"
	"iter"
	"silo/queues"
)

// -------------------------------------------------------
// Channel Adapter
// -------------------------------------------------------

// ChannelQueue adapts a native Go channel to the BatcherQueue interface.
type ChannelQueue[T any] struct {
	*queues.NotifyQueue[T]
	input <-chan T
}

// NewChannelQueue creates a BatcherQueue from a read-only channel.
// The queue closes automatically when the input channel is closed.
// capacity sets the maximum number of items to buffer before blocking (backpressure).
func NewChannelQueue[T any](input <-chan T, capacity int) *ChannelQueue[T] {
	if capacity <= 0 {
		capacity = 128
	}
	q := &ChannelQueue[T]{
		NotifyQueue: queues.NewNotifyQueue[T](capacity, capacity),
		input:       input,
	}
	go q.ingest()
	return q
}

func (q *ChannelQueue[T]) ingest() {
	defer q.Close()
	for item := range q.input {
		// EnqueueOrWait blocks if the queue is full, providing backpressure.
		if err := q.EnqueueOrWait(item); err != nil {
			return // Queue closed
		}
	}
}

// -------------------------------------------------------
// Seq Adapter
// -------------------------------------------------------

// SeqQueue adapts a standard Go iterator (iter.Seq) to the BatcherQueue interface.
type SeqQueue[T any] struct {
	*queues.NotifyQueue[T]
	seq       iter.Seq[T]
	batchSize int
}

// NewSeqQueue creates a BatcherQueue from an iterator sequence.
// The queue closes automatically when the sequence is exhausted.
// capacity sets the maximum number of items to buffer before blocking (backpressure).
func NewSeqQueue[T any](seq iter.Seq[T], capacity int) *SeqQueue[T] {
	if capacity <= 0 {
		capacity = 128
	}

	// Ensure local pre-batch size doesn't exceed capacity to prevent deadlocks
	// if the underlying queue blocks on large batches.
	// We cap it at 128 for general performance.
	bs := 128
	if capacity < bs {
		bs = capacity
	}

	q := &SeqQueue[T]{
		NotifyQueue: queues.NewNotifyQueue[T](capacity, capacity),
		seq:         seq,
		batchSize:   bs,
	}
	go q.ingest()
	return q
}

func (q *SeqQueue[T]) ingest() {
	defer q.Close()
	buffer := make([]T, 0, q.batchSize)

	for item := range q.seq {
		buffer = append(buffer, item)
		if len(buffer) >= q.batchSize {
			if err := q.EnqueueBatchOrWait(buffer...); err != nil {
				return
			}
			// Allocate new buffer to prevent data race if the queue retains the slice
			buffer = make([]T, 0, q.batchSize)
		}
	}
	// Flush remaining
	if len(buffer) > 0 {
		q.EnqueueBatchOrWait(buffer...)
	}
}

// -------------------------------------------------------
// Fetcher Adapter
// -------------------------------------------------------

// FetcherFunc is a function that pulls a single item (blocking).
// It should return an error to stop the queue (e.g. io.EOF or context cancellation).
type FetcherFunc[T any] func(context.Context) (T, error)

// FetcherQueue adapts a blocking fetch function to the BatcherQueue interface.
type FetcherQueue[T any] struct {
	*queues.NotifyQueue[T]
	fetcher FetcherFunc[T]
	ctx     context.Context
}

// NewFetcherQueue creates a BatcherQueue from a fetcher function.
// The queue stops when the fetcher returns an error or the context is cancelled.
// capacity sets the maximum number of items to buffer before blocking (backpressure).
func NewFetcherQueue[T any](ctx context.Context, fetcher FetcherFunc[T], capacity int) *FetcherQueue[T] {
	if capacity <= 0 {
		capacity = 128
	}
	q := &FetcherQueue[T]{
		NotifyQueue: queues.NewNotifyQueue[T](capacity, capacity),
		fetcher:     fetcher,
		ctx:         ctx,
	}
	go q.loop()
	return q
}

func (q *FetcherQueue[T]) loop() {
	defer q.Close()
	for {
		if q.ctx.Err() != nil {
			return
		}
		val, err := q.fetcher(q.ctx)
		if err != nil {
			return
		}
		if err := q.EnqueueOrWait(val); err != nil {
			return
		}
	}
}
