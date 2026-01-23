package seqs_test

import (
	"context"
	"fmt"
	"silo/seqs"
	"slices"
)

func ExampleMap() {
	input := slices.Values([]int{1, 2, 3})

	// Apply a transformation
	result := seqs.Map(input, func(v int) int {
		return v * 10
	})

	for v := range result {
		fmt.Println(v)
	}

	// Output:
	// 10
	// 20
	// 30
}

func ExampleWindow() {
	input := slices.Values([]int{1, 2, 3, 4, 5})

	// Create sliding windows of size 3 with step 1
	windows := seqs.Window(input, 3, 1)

	for w := range windows {
		fmt.Println(w)
	}

	// Output:
	// [1 2 3]
	// [2 3 4]
	// [3 4 5]
}

func ExampleBatcher() {
	// Simulate a queue source
	input := []int{1, 2, 3, 4, 5}
	q := seqs.NewSeqQueue(slices.Values(input), 10)

	// Define a handler that processes a batch
	handler := func(ctx context.Context, batch []int) error {
		fmt.Printf("Processed batch: %v\n", batch)
		return nil
	}

	// Configure batcher to flush every 2 items.
	// Note: In a real app, you would typically run this in a goroutine.
	b := seqs.NewBatcher(q, handler, seqs.WithBatcherSize[int](2))
	b.Run(context.Background())

	// Output:
	// Processed batch: [1 2]
	// Processed batch: [3 4]
	// Processed batch: [5]
}
