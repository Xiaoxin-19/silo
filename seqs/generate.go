package seqs

import (
	"iter"
	"math/rand/v2"
)

// RandomInts generates a sequence of random integers of the specified size.
func RandomInts(size int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := 0; i < size; i++ {
			if !yield(rand.Int()) {
				return
			}
		}
	}
}

func Range(start, end, step int) iter.Seq[int] {
	return func(yield func(int) bool) {
		if step == 0 {
			return
		}
		for i := start; step > 0 && i < end || step < 0 && i > end; i += step {
			if !yield(i) {
				return
			}
		}
	}
}

func Repeat[T any](value T, count int) iter.Seq[T] {
	return func(yield func(T) bool) {
		for i := 0; i < count; i++ {
			if !yield(value) {
				return
			}
		}
	}
}
