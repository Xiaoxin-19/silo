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

func TestGroupBy(t *testing.T) {
	type User struct {
		ID   int
		Age  int
		Name string
	}

	t.Run("Happy Path (Multiple Groups)", func(t *testing.T) {
		input := []User{
			{ID: 1, Age: 20, Name: "Alice"},
			{ID: 2, Age: 25, Name: "Bob"},
			{ID: 3, Age: 20, Name: "Charlie"},
			{ID: 4, Age: 30, Name: "David"},
		}
		got := sliceutil.GroupBy(input, func(u User) int { return u.Age })
		want := map[int][]User{
			20: {{ID: 1, Age: 20, Name: "Alice"}, {ID: 3, Age: 20, Name: "Charlie"}},
			25: {{ID: 2, Age: 25, Name: "Bob"}},
			30: {{ID: 4, Age: 30, Name: "David"}},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("GroupBy() = %v, want %v", got, want)
		}
	})

	t.Run("Zero Value (Nil Input)", func(t *testing.T) {
		var input []User // Nil slice
		got := sliceutil.GroupBy(input, func(u User) int { return u.Age })
		if got == nil {
			t.Errorf("GroupBy() with nil input should return empty map, not nil")
		}
		if len(got) != 0 {
			t.Errorf("GroupBy() with nil input should return empty map, got %v", got)
		}
	})

	t.Run("Stability (Order Preservation)", func(t *testing.T) {
		input := []User{
			{ID: 1, Age: 20, Name: "Alice"},
			{ID: 2, Age: 20, Name: "Bob"},
			{ID: 3, Age: 25, Name: "Charlie"},
		}
		got := sliceutil.GroupBy(input, func(u User) int { return u.Age })
		want := map[int][]User{
			20: {{ID: 1, Age: 20, Name: "Alice"}, {ID: 2, Age: 20, Name: "Bob"}},
			25: {{ID: 3, Age: 25, Name: "Charlie"}},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("GroupBy() = %v, want %v", got, want)
		}
		// Verify order
		if got[20][0].ID != 1 || got[20][1].ID != 2 {
			t.Errorf("GroupBy() stability check failed: %v", got[20])
		}
	})

	t.Run("Boundary (Single Key)", func(t *testing.T) {
		input := []User{
			{ID: 1, Age: 20, Name: "Alice"},
			{ID: 2, Age: 20, Name: "Bob"},
		}
		got := sliceutil.GroupBy(input, func(u User) int { return 20 }) // All map to the same key
		if len(got) != 1 {
			t.Errorf("GroupBy() should have only one key, got %v", got)
		}
		if len(got[20]) != len(input) {
			t.Errorf("GroupBy() value length should equal input length, got %v", got[20])
		}
	})

	t.Run("Robustness (KeyFunc Stress)", func(t *testing.T) {
		count := 10000
		input := make([]User, count)
		for i := 0; i < count; i++ {
			input[i] = User{ID: i, Age: i, Name: fmt.Sprintf("User %d", i)}
		}
		got := sliceutil.GroupBy(input, func(u User) int { return u.ID }) // Each element has a unique key
		if len(got) != count {
			t.Errorf("GroupBy() should have %d keys, got %d", count, len(got))
		}
		for i := 0; i < count; i++ {
			if len(got[i]) != 1 {
				t.Errorf("GroupBy() key %d should have one element, got %v", i, got[i])
			}
		}
	})
}

func TestPartition(t *testing.T) {
	t.Run("Happy Path (Exact Length & Order)", func(t *testing.T) {
		input := []int{1, 2, 3, 4, 5}
		// Keep evens
		matched, unmatched := sliceutil.Partition(input, func(x int) bool { return x%2 == 0 })

		wantMatched := []int{2, 4}
		wantUnmatched := []int{1, 3, 5}

		if !reflect.DeepEqual(matched, wantMatched) {
			t.Errorf("Matched = %v, want %v", matched, wantMatched)
		}
		if !reflect.DeepEqual(unmatched, wantUnmatched) {
			t.Errorf("Unmatched = %v, want %v", unmatched, wantUnmatched)
		}
	})

	t.Run("Memory Semantics (Copy Isolation)", func(t *testing.T) {
		input := []int{1, 2, 3, 4}
		matched, _ := sliceutil.Partition(input, func(x int) bool { return x%2 == 0 })

		if len(matched) > 0 {
			matched[0] = 999
		}

		// Verify original slice is untouched
		if input[1] == 999 { // input[1] was 2, which became matched[0]
			t.Error("Partition() should return a copy, but original slice was modified")
		}
	})

	t.Run("Edge Case (Empty/Nil)", func(t *testing.T) {
		var input []int
		m, u := sliceutil.Partition(input, func(x int) bool { return x > 0 })
		if m == nil || u == nil {
			t.Error("Partition() should return non-nil empty slices for nil input")
		}
	})
}

func TestPartitionInPlace(t *testing.T) {
	t.Run("Correctness", func(t *testing.T) {
		input := []int{1, 2, 3, 4, 5, 6}
		matched, unmatched := sliceutil.PartitionInPlace(input, func(x int) bool { return x%2 == 0 })

		// Verify all matched are even
		for _, v := range matched {
			if v%2 != 0 {
				t.Errorf("Matched slice contains odd number: %d", v)
			}
		}
		// Verify all unmatched are odd
		for _, v := range unmatched {
			if v%2 == 0 {
				t.Errorf("Unmatched slice contains even number: %d", v)
			}
		}
		// Verify total count
		if len(matched)+len(unmatched) != 6 {
			t.Errorf("Total elements lost during partition")
		}
	})

	t.Run("Instability (Order Change)", func(t *testing.T) {
		type Item struct {
			ID  int
			Val int
		}
		// Construct a case where the swap logic changes relative order.
		// Input: [Odd1, Even1, Even2, Odd2] -> Keep Evens
		// Trace:
		// 1. Odd1 swaps with Odd2 -> [Odd2, Even1, Even2, Odd1]
		// 2. Odd2 swaps with Even2 -> [Even2, Even1, Odd2, Odd1]
		// Result Matched: [Even2, Even1] (Reversed relative to original [Even1, Even2])
		input := []Item{{1, 1}, {2, 2}, {3, 2}, {4, 1}}
		matched, _ := sliceutil.PartitionInPlace(input, func(i Item) bool { return i.Val == 2 })

		if len(matched) != 2 {
			t.Fatalf("Expected 2 matched items, got %d", len(matched))
		}
		// If order was preserved, it would be ID:2 then ID:3.
		// Due to instability, we expect ID:3 then ID:2.
		if matched[0].ID == 2 && matched[1].ID == 3 {
			t.Log("Order happened to be preserved (unexpected for this algorithm but possible in some cases)")
		} else if matched[0].ID == 3 && matched[1].ID == 2 {
			// This confirms instability
		} else {
			t.Errorf("Unexpected order in matched slice: %v", matched)
		}
	})

	t.Run("Memory Semantics (In-Place Modification)", func(t *testing.T) {
		input := []int{1, 2, 3, 4} // Evens: 2, 4. Odds: 1, 3
		origPtr := &input[0]       // Pointer to first element
		matched, _ := sliceutil.PartitionInPlace(input, func(x int) bool { return x%2 == 0 })

		// Verify pointer stability (matched slice should point to the start of original array)
		if len(matched) > 0 && &matched[0] != origPtr {
			t.Error("PartitionInPlace should reuse the underlying array")
		}

		// Verify original slice content is modified (it should now be partitioned)
		// Likely result: [4, 2, 3, 1] or similar, but definitely not [1, 2, 3, 4]
		isModified := false
		for i, v := range input {
			if v != []int{1, 2, 3, 4}[i] {
				isModified = true
				break
			}
		}
		if !isModified {
			t.Error("Original slice was not modified in place")
		}
	})
}
