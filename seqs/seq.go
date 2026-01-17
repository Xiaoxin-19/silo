package seqs

import (
	"iter"
	"math/rand"
	"time"
)

// Filter applies predicate to each element of seq, yielding only those that satisfy the predicate.
func Filter[T any](seq iter.Seq[T], predicate func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range seq {
			if predicate(v) {
				if !yield(v) {
					return
				}
			}
		}
	}
}

// TryFilter returns a sequence of elements that satisfy the predicate.
// The predicate function can return an error.
//
// The resulting sequence yields pairs of (element, error).
// If the predicate returns an error:
//   - The error is yielded to the consumer along with the element 'v' that caused it.
//   - The iteration CONTINUES if the consumer returns true (yield returns true).
//   - The iteration STOPS if the consumer returns false (yield returns false).
func TryFilter[T any](seq iter.Seq[T], predicate func(T) (bool, error)) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		for v := range seq {
			keep, err := predicate(v)
			if err != nil {
				// error occurred: yield the error along with the element 'v' that caused it
				if !yield(v, err) {
					return
				}
				// The iteration CONTINUES if the consumer returns true (yield returns true).
				continue
			}

			// No error: decide whether to keep based on predicate result
			if keep {
				if !yield(v, nil) {
					return
				}
			}
		}
	}
}

// Map applies transform to each element of seq, yielding the transformed elements.
func Map[T, R any](seq iter.Seq[T], transform func(T) R) iter.Seq[R] {
	return func(yield func(R) bool) {
		for v := range seq {
			if !yield(transform(v)) {
				return
			}
		}
	}
}

// TryMap applies transform to each element of seq, yielding the transformed elements.
// The transform function can return an error.
// The resulting sequence yields pairs of (transformed element, error).
// If transform returns an error:
//   - The error is yielded to the consumer along with a zero-value of type R.
//   - The iteration CONTINUES if the consumer returns true (yield returns true).
//   - The iteration STOPS if the consumer returns false (yield returns false).
func TryMap[T, R any](seq iter.Seq[T], transform func(T) (R, error)) iter.Seq2[R, error] {
	return func(yield func(R, error) bool) {
		for v := range seq {
			res, err := transform(v)
			if !yield(res, err) {
				return
			}
		}
	}
}

// Reduce aggregates the elements of seq using the reducer function, starting from the initial value.
func Reduce[T, R any](seq iter.Seq[T], initial R, reducer func(R, T) R) R {
	acc := initial
	for v := range seq {
		acc = reducer(acc, v)
	}
	return acc
}

// TryReduce aggregates the elements of seq using the reducer function, starting from the initial value.
// If reducer returns an error, it is returned immediately.
func TryReduce[T, R any](seq iter.Seq[T], initial R, reducer func(R, T) (R, error)) (R, error) {
	acc := initial
	for v := range seq {
		acc, err := reducer(acc, v)
		if err != nil {
			return acc, err
		}
	}
	return acc, nil
}

// RandomIntRange generates a sequence of random integers of the specified size.
func RandomIntRange(size int) iter.Seq[int] {
	randSeed := time.Now().UnixNano()
	rd := rand.New(rand.NewSource(randSeed))
	return func(yield func(int) bool) {
		for range size {
			if !yield(rd.Int()) {
				return
			}
		}
	}
}
