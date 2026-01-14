package sliceutil_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"silo/sliceutil"
)

func TestFilter(t *testing.T) {
	input := []int{1, 2, 3, 4, 5, 6}
	want := []int{2, 4, 6}
	got := sliceutil.Filter(input, func(x int) bool {
		return x%2 == 0
	})
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Filter() = %v, want %v", got, want)
	}
}

func TestFilterInPlace(t *testing.T) {
	input := []int{1, 2, 3, 4, 5, 6}
	want := []int{2, 4, 6}

	got := sliceutil.FilterInPlace(input, func(x int) bool {
		return x%2 == 0
	})

	if !reflect.DeepEqual(got, want) {
		t.Errorf("FilterInPlace() = %v, want %v", got, want)
	}

	// Verify that the underlying array has been modified
	if input[0] != 2 || input[1] != 4 || input[2] != 6 {
		t.Errorf("Underlying array not modified correctly: %v", input)
	}
}

func TestMap(t *testing.T) {
	input := []int{1, 2, 3}
	want := []string{"1", "2", "3"}
	got := sliceutil.Map(input, func(x int) string {
		return fmt.Sprintf("%d", x)
	})
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Map() = %v, want %v", got, want)
	}
}

func TestReduce(t *testing.T) {
	input := []int{1, 2, 3, 4}
	want := 10
	got := sliceutil.Reduce(input, func(acc, item int) int {
		return acc + item
	}, 0)
	if got != want {
		t.Errorf("Reduce() = %v, want %v", got, want)
	}
}

func TestTryFilter(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		input := []int{1, 2, 3}
		got, err := sliceutil.TryFilter(input, func(x int) (bool, error) {
			return x > 1, nil
		})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got, []int{2, 3}) {
			t.Errorf("TryFilter() = %v, want %v", got, []int{2, 3})
		}
	})

	t.Run("Error", func(t *testing.T) {
		input := []int{1, 2, 3}
		expectedErr := errors.New("fail")
		_, err := sliceutil.TryFilter(input, func(x int) (bool, error) {
			if x == 2 {
				return false, expectedErr
			}
			return true, nil
		})
		if err != expectedErr {
			t.Errorf("TryFilter() error = %v, want %v", err, expectedErr)
		}
	})
}

func TestTryMap(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		input := []int{1, 2}
		got, err := sliceutil.TryMap(input, func(x int) (int, error) {
			return x * 2, nil
		})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got, []int{2, 4}) {
			t.Errorf("TryMap() = %v, want %v", got, []int{2, 4})
		}
	})

	t.Run("Error", func(t *testing.T) {
		input := []int{1, 2}
		expectedErr := errors.New("fail")
		_, err := sliceutil.TryMap(input, func(x int) (int, error) {
			return 0, expectedErr
		})
		if err != expectedErr {
			t.Errorf("TryMap() error = %v, want %v", err, expectedErr)
		}
	})
}

func TestTryParallelMap(t *testing.T) {
	t.Run("SmallDataset", func(t *testing.T) {
		input := []int{1, 2, 3}
		got, err := sliceutil.TryParallelMap(input, func(x int) (int, error) {
			return x * 2, nil
		})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got, []int{2, 4, 6}) {
			t.Errorf("SmallDataset: got %v", got)
		}
	})

	t.Run("LargeDataset", func(t *testing.T) {
		count := 1000
		input := make([]int, count)
		for i := 0; i < count; i++ {
			input[i] = i
		}
		got, err := sliceutil.TryParallelMap(input, func(x int) (int, error) {
			return x * 2, nil
		})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if len(got) != count {
			t.Fatalf("Length mismatch: got %d, want %d", len(got), count)
		}
		// Simple sampling verification
		if got[0] != 0 || got[count-1] != (count-1)*2 {
			t.Errorf("Value mismatch")
		}
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		count := 1000
		input := make([]int, count)
		for i := 0; i < count; i++ {
			input[i] = i
		}
		expectedErr := errors.New("oops")

		_, err := sliceutil.TryParallelMap(input, func(x int) (int, error) {
			if x == 500 {
				return 0, expectedErr
			}
			return x * 2, nil
		})

		if err != expectedErr {
			t.Errorf("Expected error %v, got %v", expectedErr, err)
		}
	})
}

func TestTryParallelFilter(t *testing.T) {
	t.Run("OrderAndCorrectness", func(t *testing.T) {
		count := 1000
		input := make([]int, count)
		for i := 0; i < count; i++ {
			input[i] = i
		}

		// Keep even numbers
		got, err := sliceutil.TryParallelFilter(input, func(x int) (bool, error) {
			return x%2 == 0, nil
		})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if len(got) != 500 {
			t.Fatalf("Expected 500 elements, got %d", len(got))
		}

		// Verify order and values
		for i, v := range got {
			if v != i*2 {
				t.Errorf("Mismatch at index %d: got %d, want %d", i, v, i*2)
				break
			}
		}
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		input := make([]int, 1000)
		for i := range input {
			input[i] = i
		}
		expectedErr := errors.New("filter error")

		_, err := sliceutil.TryParallelFilter(input, func(x int) (bool, error) {
			if x == 500 {
				return false, expectedErr
			}
			return true, nil
		})

		if err != expectedErr {
			t.Errorf("Expected error %v, got %v", expectedErr, err)
		}
	})
}

var PredicateheavyPredicate = func(x int) (bool, error) {
	for i := 0; i < 1000; i++ {
		x = (x + i*i) % 10000
	}
	return x%2 == 0, nil
}

// BenchmarkParallelFilter_HeavyWork
func BenchmarkParallelFilter_HeavyWork(b *testing.B) {
	size := 10000
	input := make([]int, size)
	for i := 0; i < size; i++ {
		input[i] = i
	}

	b.Run("SerialFilter_Heavy", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = sliceutil.TryFilter(input, PredicateheavyPredicate)
		}
	})

	b.Run("ParallelFilter_Heavy", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = sliceutil.TryParallelFilter(input, PredicateheavyPredicate)
		}
	})
}

var heavyTransform = func(x int) (int, error) {
	for i := 0; i < 1000; i++ {
		x = (x + i*i) % 10000
	}
	return x, nil
}

// BenchmarkParallelMap_HeavyWork
func BenchmarkParallelMap_HeavyWork(b *testing.B) {
	size := 10000
	input := make([]int, size)
	for i := 0; i < size; i++ {
		input[i] = i
	}

	b.Run("SerialMap_Heavy", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = sliceutil.TryMap(input, heavyTransform)
		}
	})

	b.Run("ParallelMap_Heavy", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = sliceutil.TryParallelMap(input, heavyTransform)
		}
	})
}
