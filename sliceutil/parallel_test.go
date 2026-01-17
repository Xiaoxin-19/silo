package sliceutil_test

import (
	"errors"
	"reflect"
	"silo/sliceutil"
	"testing"
)

func FuzzTryParallelFilter(f *testing.F) {
	// 1. Seed Corpus
	f.Add([]byte{1, 2, 3, 4, 5, 6}, byte(100))
	f.Add([]byte{1, 2, 3}, byte(2))

	largeData := make([]byte, 1000)
	for i := range largeData {
		largeData[i] = byte(i % 255)
	}
	f.Add(largeData, byte(250))

	f.Fuzz(func(t *testing.T, input []byte, failVal byte) {
		// Predicate: keep even numbers, error on failVal
		predicate := func(b byte) (bool, error) {
			if b == failVal {
				return false, errors.New("mock error")
			}
			return b%2 == 0, nil
		}

		// 2. Execute parallel version (target under test)
		got, err := sliceutil.TryParallelFilter(input, predicate)

		// 3. Execute serial version (baseline truth)
		want, expectedErr := sliceutil.TryFilter(input, predicate)

		// 4. validate results
		if expectedErr != nil {
			if err == nil {
				t.Errorf("Expected error but got nil. Input len: %d, FailVal: %d", len(input), failVal)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error: %v. Input len: %d", err, len(input))
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Result mismatch.\nInput len: %d\nGot:  %v\nWant: %v", len(input), got, want)
			}
		}
	})
}
func FuzzTryParallelMap(f *testing.F) {
	// 1. Seed Corpus
	// Case 1: Small slice, no error
	f.Add([]byte{1, 2, 3}, byte(100))
	// Case 2: Small slice, contains element that triggers error
	f.Add([]byte{1, 2, 3}, byte(2))

	// Case 3: Large slice, triggers parallel logic (Threshold = 256)
	largeData := make([]byte, 1000)
	for i := range largeData {
		largeData[i] = byte(i % 255)
	}
	f.Add(largeData, byte(250))

	f.Fuzz(func(t *testing.T, input []byte, failVal byte) {
		// Define transform function: simulate computation and error on specific value
		transform := func(b byte) (byte, error) {
			if b == failVal {
				return 0, errors.New("mock error")
			}
			// Simple computation
			return b * 2, nil
		}

		// 2. Execute parallel version (target under test)
		got, err := sliceutil.TryParallelMap(input, transform)

		// 3. Execute serial version (baseline truth)
		want, expectedErr := sliceutil.TryMap(input, transform)

		// 4. validate results
		if expectedErr != nil {
			if err == nil {
				t.Errorf("Expected error but got nil. Input len: %d, FailVal: %d", len(input), failVal)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error: %v. Input len: %d", err, len(input))
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Result mismatch.\nInput len: %d\nGot:  %v\nWant: %v", len(input), got, want)
			}
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
