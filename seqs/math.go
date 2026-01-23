package seqs

import "iter"

type Number interface {
	int | int8 | int16 | int32 | int64 |
		uint | uint8 | uint16 | uint32 | uint64 |
		float32 | float64
}

func Sum[T Number](seq iter.Seq[T]) T {
	var total T
	for v := range seq {
		total += v
	}
	return total
}

func Min[T Number](seq iter.Seq[T]) (T, bool) {
	var min T
	first := true
	for v := range seq {
		if first {
			min = v
			first = false
			continue
		}
		if v < min {
			min = v
		}
	}
	if first {
		var zero T
		return zero, false
	}
	return min, true
}

func Max[T Number](seq iter.Seq[T]) (T, bool) {
	var max T
	first := true
	for v := range seq {
		if first {
			max = v
			first = false
			continue
		}
		if v > max {
			max = v
		}
	}
	if first {
		var zero T
		return zero, false
	}
	return max, true
}
