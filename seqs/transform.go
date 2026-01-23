package seqs

import "iter"

func FlatMap[S any, T any](source iter.Seq[S], f func(S) iter.Seq[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for s := range source {
			for t := range f(s) {
				if !yield(t) {
					return
				}
			}
		}
	}
}

func TryFlatMap[S any, T any](source iter.Seq[S], f func(S) iter.Seq2[T, error]) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		for s := range source {
			for t, err := range f(s) {
				if !yield(t, err) {
					return
				}
			}
		}
	}
}

func Concat[T any](seqs ...iter.Seq[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, seq := range seqs {
			for v := range seq {
				if !yield(v) {
					return
				}
			}
		}
	}
}

type Pair[T1, T2 any] struct {
	V1 T1
	V2 T2
}

func Zip[T1, T2 any](seq1 iter.Seq[T1], seq2 iter.Seq[T2]) iter.Seq[Pair[T1, T2]] {
	return func(yield func(Pair[T1, T2]) bool) {
		next2, stop2 := iter.Pull(seq2)
		defer stop2()

		for v1 := range seq1 {
			v2, ok := next2()
			if !ok {
				return
			}
			if !yield(Pair[T1, T2]{v1, v2}) {
				return
			}
		}
	}
}

// ZipLongest zips two sequences together.
// When one sequence is exhausted, it continues with the fill values.
// use fill1 and fill2 to fill in the missing values from seq1 and seq2 respectively.
func ZipLongest[T1, T2 any](
	seq1 iter.Seq[T1],
	seq2 iter.Seq[T2],
	fill1 T1,
	fill2 T2,
) iter.Seq[Pair[T1, T2]] {
	return func(yield func(Pair[T1, T2]) bool) {
		next1, stop1 := iter.Pull(seq1)
		defer stop1()
		next2, stop2 := iter.Pull(seq2)
		defer stop2()

		for {
			v1, ok1 := next1()
			v2, ok2 := next2()

			// all done
			if !ok1 && !ok2 {
				return
			}

			// fill missing values
			if !ok1 {
				v1 = fill1
			}
			if !ok2 {
				v2 = fill2
			}
			// 3. 输出
			if !yield(Pair[T1, T2]{V1: v1, V2: v2}) {
				return
			}
		}
	}
}

func Enumerate[T any](seq iter.Seq[T]) iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		index := 0
		for v := range seq {
			if !yield(index, v) {
				return
			}
			index++
		}
	}
}

// Chunk splits the input sequence into chunks of the specified size.
// The last chunk may be smaller if there are not enough elements.
func Chunk[T any](seq iter.Seq[T], size int) iter.Seq[[]T] {
	return func(yield func([]T) bool) {
		if size <= 0 {
			return
		}

		batch := make([]T, 0, size)

		for v := range seq {
			batch = append(batch, v)
			if len(batch) == size {
				if !yield(batch) {
					return
				}
				batch = make([]T, 0, size)
			}
		}
		if len(batch) > 0 {
			yield(batch)
		}
	}
}

// Window creates a sliding window over the input sequence.
// size: window size.
// step: step size for each slide.
//
// Scenario 1 (step < size): overlapping windows. For example, [1,2,3], [2,3,4] (size=3, step=1)
// Scenario 2 (step == size): equivalent to Chunk.
// Scenario 3 (step > size): gapped windows (some data is skipped in between).
func Window[T any](seq iter.Seq[T], size, step int) iter.Seq[[]T] {
	return func(yield func([]T) bool) {
		if size <= 0 || step <= 0 {
			return
		}

		// Pre-allocate buffer with a slightly larger capacity to avoid append reallocations
		buffer := make([]T, 0, size)

		// when step > size, we need to skip some elements after yielding
		skipCount := 0

		for v := range seq {
			// 1. in skip mode
			if skipCount > 0 {
				skipCount--
				continue
			}

			// 2. collect data
			buffer = append(buffer, v)

			// 3. window not full, continue collecting
			if len(buffer) < size {
				continue
			}

			// 4. window full, yield it
			output := make([]T, size)
			copy(output, buffer)

			if !yield(output) {
				return
			}

			// 5. slide window
			if step < size {
				// overlapping mode: keep the latter part
				// Go's copy handles overlapping memory very safely and efficiently (memmove)
				// e.g. [1,2,3,4,5], step=2 => copy([1...], [3,4,5]) => [3,4,5,4,5]
				copy(buffer, buffer[step:])
				// Re-slice => [3,4,5]
				buffer = buffer[:size-step]
			} else {
				// gap mode: clear and set skip count
				buffer = buffer[:0]
				skipCount = step - size
			}
		}
	}
}

// Distinct returns a sequence that yields only unique elements.
// It maintains a map of seen elements, so memory usage is proportional to the number of unique elements.
func Distinct[T comparable](seq iter.Seq[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		seen := make(map[T]struct{})
		for v := range seq {
			if _, ok := seen[v]; !ok {
				seen[v] = struct{}{}
				if !yield(v) {
					return
				}
			}
		}
	}
}

// Peek performs the provided action on each element of the sequence without modifying it.
// It is useful for debugging (e.g., logging) or side effects.
func Peek[T any](seq iter.Seq[T], action func(T)) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range seq {
			action(v)
			if !yield(v) {
				return
			}
		}
	}
}

// Scan is similar to Reduce, but it yields the accumulated result at each step.
func Scan[T, R any](seq iter.Seq[T], initial R, reducer func(R, T) R) iter.Seq[R] {
	return func(yield func(R) bool) {
		acc := initial
		for v := range seq {
			acc = reducer(acc, v)
			if !yield(acc) {
				return
			}
		}
	}
}
