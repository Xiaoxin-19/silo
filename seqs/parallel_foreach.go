package seqs

import (
	"context"
	"fmt"
	"iter"
	"sync"
)

// BatchForeach processes elements from seq in parallel batches using the provided BatchHandler.
// It utilizes a Batcher internally to manage batching and concurrency.
// Options can be provided to customize the Batcher's behavior.
func BatchForeach[T any](ctx context.Context, seq iter.Seq[T], h BatchHandler[T], options ...BatcherOption[T]) {
	q := NewSeqQueue(seq, 4096)
	batcher := NewBatcher(q, h, options...)
	batcher.Run(ctx)
}

// ParallelForeach processes each element of seq in parallel using the provided handler function.
// The number of concurrent workers can be specified. If workers <= 0, it defaults to 1.
// It returns a Seq2 that yields the original elements along with any error encountered during processing.
func ParallelForeach[T any](ctx context.Context, seq iter.Seq[T], handler func(T) error, workers int) iter.Seq2[T, error] {
	if workers <= 0 {
		workers = 1
	}

	type resultType struct {
		result T
		err    error
	}

	return func(yield func(T, error) bool) {

		// stopCh is closed when we need to stop processing early
		stopCh := make(chan struct{})
		var stopOnce sync.Once
		// resultCh is used to send processing results back to the consumer
		resultCh := make(chan resultType, workers*3)

		go func() {
			var wg sync.WaitGroup
			defer func() {
				wg.Wait()
				close(resultCh)
			}()

			// sem is a semaphore to control concurrency
			sem := make(chan struct{}, workers)

			for v := range seq {
				select {
				case <-ctx.Done():
					return // context cancelled, stop dispatching
				case <-stopCh:
					return // consumer requested stop, stop dispatching
				case sem <- struct{}{}: // acquire token
					// continue execution
				}

				wg.Add(1)
				go func(val T) {
					defer func() {
						wg.Done()
						<-sem
						if r := recover(); r != nil {

							select {
							case resultCh <- resultType{val, fmt.Errorf("panic: %v", r)}:
							case <-ctx.Done():
							case <-stopCh:
							}
						}
					}()

					err := handler(val)

					select {
					case resultCh <- resultType{val, err}:
					case <-ctx.Done():
					case <-stopCh:
					}
				}(v)
			}
		}()

		for item := range resultCh {
			if !yield(item.result, item.err) {
				stopOnce.Do(func() {
					close(stopCh)
				})
				return
			}
		}
	}
}
