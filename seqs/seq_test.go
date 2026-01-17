package seqs_test

import (
	"errors"
	"silo/seqs"
	"slices"
	"testing"
)

func TestTryMap(t *testing.T) {
	input := []int{1, 2, 3, 4}
	expectedErr := errors.New("fail")

	t.Run("Success", func(t *testing.T) {
		seq := seqs.TryMap(slices.Values(input), func(x int) (int, error) {
			return x * 2, nil
		})

		var result []int
		for v, err := range seq {
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			result = append(result, v)
		}
		if !slices.Equal(result, []int{2, 4, 6, 8}) {
			t.Errorf("TryMap success mismatch: got %v", result)
		}
	})

	t.Run("Error", func(t *testing.T) {
		seqErr := seqs.TryMap(slices.Values(input), func(x int) (int, error) {
			if x == 3 {
				return 0, expectedErr
			}
			return x * 2, nil
		})

		var result []int
		var gotErr error
		for v, err := range seqErr {
			if err != nil {
				gotErr = err
				break
			}
			result = append(result, v)
		}

		if gotErr != expectedErr {
			t.Errorf("Expected error %v, got %v", expectedErr, gotErr)
		}
		// Should stop at 3, so we get results for 1 and 2
		if !slices.Equal(result, []int{2, 4}) {
			t.Errorf("TryMap error partial result mismatch: got %v", result)
		}
	})
}
