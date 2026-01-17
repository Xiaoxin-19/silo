package sliceutil_test

import (
	"testing"

	"silo/sliceutil"
)

const benchSize = 1_000_000

func getBenchData() []int {
	data := make([]int, benchSize)
	for i := 0; i < benchSize; i++ {
		data[i] = i
	}
	return data
}

var isEven = func(x int) bool {
	return x%2 == 0
}

// BenchmarkPartition_TwoPass benchmarks the optimized two-pass Partition implementation.
// Expectation: Very few allocations (2 allocs) due to exact pre-allocation.
func BenchmarkPartition_TwoPass(b *testing.B) {
	data := getBenchData()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = sliceutil.Partition(data, isEven)
	}
}

// BenchmarkPartition_NaiveAppend benchmarks a naive implementation using append.
// Expectation: High allocations and copying overhead due to dynamic slice growth.
func BenchmarkPartition_NaiveAppend(b *testing.B) {
	data := getBenchData()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var matched []int
		var unmatched []int
		for _, v := range data {
			if isEven(v) {
				matched = append(matched, v)
			} else {
				unmatched = append(unmatched, v)
			}
		}
	}
}

// BenchmarkPartition_InPlace benchmarks the in-place partition.
// Expectation: Fastest speed and zero allocations.
func BenchmarkPartition_InPlace(b *testing.B) {
	data := getBenchData()
	// Create a scratch buffer to reset data, ensuring we measure the swap cost
	// rather than scanning an already partitioned array.
	scratch := make([]int, len(data))
	copy(scratch, data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		copy(data, scratch)
		b.StartTimer()

		_, _ = sliceutil.PartitionInPlace(data, isEven)
	}
}

// BenchmarkPartition_GroupBy benchmarks using GroupBy for binary classification.
// Expectation: Slowest due to map overhead, hashing, and slice growth.
func BenchmarkPartition_GroupBy(b *testing.B) {
	data := getBenchData()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sliceutil.GroupBy(data, func(x int) bool {
			return x%2 == 0
		})
	}
}
