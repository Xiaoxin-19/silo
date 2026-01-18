package seqs_test

import (
	"silo/seqs"
	"silo/sliceutil"
	"slices"
	"testing"
)

// heavyCalc simulates a CPU intensive operation
func heavyCalc(x int) int {
	for i := 0; i < 1000; i++ {
		x = (x + i*i) % 10000
	}
	return x
}

// BenchmarkUnified_Map compares Map operations across different implementations and workloads.
func BenchmarkUnified_Map(b *testing.B) {
	size := 1_000_000
	input := make([]int, size)
	for i := 0; i < size; i++ {
		input[i] = i
	}

	workloads := []struct {
		name         string
		transform    func(int) int
		transformErr func(int) (int, error)
	}{
		{
			name:         "Light",
			transform:    func(x int) int { return x * 2 },
			transformErr: func(x int) (int, error) { return x * 2, nil },
		},
		{
			name:         "Heavy",
			transform:    heavyCalc,
			transformErr: func(x int) (int, error) { return heavyCalc(x), nil },
		},
	}

	for _, wl := range workloads {
		b.Run(wl.name, func(b *testing.B) {
			b.Run("Slice_Serial", func(b *testing.B) {
				for b.Loop() {
					_ = sliceutil.Map(input, wl.transform)
				}
			})

			b.Run("Seq_Serial", func(b *testing.B) {
				for b.Loop() {
					for range seqs.Map(slices.Values(input), wl.transform) {
					}
				}
			})

			b.Run("Slice_Parallel", func(b *testing.B) {
				for b.Loop() {
					_, _ = sliceutil.TryParallelMap(input, wl.transformErr)
				}
			})

			b.Run("Seq_Parallel_Batch", func(b *testing.B) {
				for b.Loop() {
					if wl.name == "Heavy" {
						for range seqs.ParallelTryMap(slices.Values(input), wl.transformErr, seqs.WithContext(b.Context())) {
						}
					}

					if wl.name == "Light" {
						for range seqs.ParallelTryMap(slices.Values(input), wl.transformErr, seqs.WithBatchSize(2048), seqs.WithContext(b.Context())) {
						}
					}

				}
			})
		})
	}
}

// BenchmarkUnified_Filter compares Filter operations across different implementations and workloads.
func BenchmarkUnified_Filter(b *testing.B) {
	size := 1_000_000
	input := make([]int, size)
	for i := 0; i < size; i++ {
		input[i] = i
	}

	workloads := []struct {
		name         string
		predicate    func(int) bool
		predicateErr func(int) (bool, error)
	}{
		{
			name:         "Light",
			predicate:    func(x int) bool { return x%2 == 0 },
			predicateErr: func(x int) (bool, error) { return x%2 == 0, nil },
		},
		{
			name:         "Heavy",
			predicate:    func(x int) bool { return heavyCalc(x)%2 == 0 },
			predicateErr: func(x int) (bool, error) { return heavyCalc(x)%2 == 0, nil },
		},
	}

	for _, wl := range workloads {
		b.Run(wl.name, func(b *testing.B) {
			b.Run("Slice_Serial", func(b *testing.B) {
				for b.Loop() {
					_ = sliceutil.Filter(input, wl.predicate)
				}
			})

			b.Run("Seq_Serial", func(b *testing.B) {
				for b.Loop() {
					for range seqs.Filter(slices.Values(input), wl.predicate) {
					}
				}
			})

			b.Run("Slice_InPlace", func(b *testing.B) {
				scratch := make([]int, len(input))
				copy(scratch, input)
				data := make([]int, len(input))

				for b.Loop() {
					b.StopTimer()
					copy(data, scratch)
					b.StartTimer()
					_ = sliceutil.FilterInPlace(data, wl.predicate)
				}
			})

			b.Run("Slice_Parallel", func(b *testing.B) {
				for b.Loop() {
					_, _ = sliceutil.TryParallelFilter(input, wl.predicateErr)
				}
			})

			b.Run("Seq_Parallel_Batch", func(b *testing.B) {
				for b.Loop() {
					if wl.name == "Heavy" {
						for range seqs.ParallelTryFilter(slices.Values(input), wl.predicateErr, seqs.WithContext(b.Context())) {
						}
						if wl.name == "Light" {
							count := 0
							for v, err := range seqs.ParallelTryFilter(slices.Values(input), wl.predicateErr, seqs.WithContext(b.Context()), seqs.WithBatchSize(1024)) {
								count += v
								if err != nil {
									b.Fatal(err)
								}
							}
						}
					}
				}
			})
		})
	}
}
