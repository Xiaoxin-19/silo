package seqs

import "iter"

func First[T any](seq iter.Seq[T]) (T, bool) {
	for v := range seq {
		return v, true
	}
	var zero T
	return zero, false
}

func Last[T any](seq iter.Seq[T]) (T, bool) {
	var last T
	found := false
	for v := range seq {
		last = v
		found = true
	}
	return last, found
}

func Any[T any](seq iter.Seq[T], predicate func(T) bool) bool {
	for v := range seq {
		if predicate(v) {
			return true
		}
	}
	return false
}

func All[T any](seq iter.Seq[T], predicate func(T) bool) bool {
	for v := range seq {
		if !predicate(v) {
			return false
		}
	}
	return true
}

func Count[T any](seq iter.Seq[T]) int {
	count := 0
	for range seq {
		count++
	}
	return count
}
