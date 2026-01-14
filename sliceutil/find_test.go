package sliceutil_test

import (
	"silo/sliceutil"
	"testing"
)

func TestContains(t *testing.T) {
	tests := []struct {
		name   string
		input  []int
		target int
		want   bool
	}{
		{"Found", []int{1, 2, 3}, 2, true},
		{"NotFound", []int{1, 2, 3}, 4, false},
		{"Empty", []int{}, 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sliceutil.Contains(tt.input, tt.target); got != tt.want {
				t.Errorf("Contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFind(t *testing.T) {
	input := []int{1, 2, 3, 4, 5}

	t.Run("Found", func(t *testing.T) {
		val, found := sliceutil.Find(input, func(x int) bool { return x > 3 })
		if !found || val != 4 {
			t.Errorf("Find() = (%v, %v), want (4, true)", val, found)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		val, found := sliceutil.Find(input, func(x int) bool { return x > 10 })
		if found {
			t.Errorf("Find() should not find element > 10, got %v", val)
		}
	})

	t.Run("Empty", func(t *testing.T) {
		val, found := sliceutil.Find([]int{}, func(x int) bool { return true })
		if found {
			t.Errorf("Find() on empty slice should return false, got %v", val)
		}
	})

	t.Run("SingleElementFound", func(t *testing.T) {
		val, found := sliceutil.Find([]int{42}, func(x int) bool { return x == 42 })
		if !found || val != 42 {
			t.Errorf("Find() = (%v, %v), want (42, true)", val, found)
		}
	})

	t.Run("SingleElementNotFound", func(t *testing.T) {
		val, found := sliceutil.Find([]int{42}, func(x int) bool { return x == 0 })
		if found {
			t.Errorf("Find() should not find, got %v", val)
		}
	})
}

func TestContainsFunc(t *testing.T) {
	tests := []struct {
		name       string
		collection []int
		predicate  func(int) bool
		want       bool
	}{
		{"FoundFirst", []int{1, 2, 3}, func(x int) bool { return x == 1 }, true},
		{"FoundMiddle", []int{1, 2, 3}, func(x int) bool { return x == 2 }, true},
		{"FoundLast", []int{1, 2, 3}, func(x int) bool { return x == 3 }, true},
		{"NotFound", []int{1, 2, 3}, func(x int) bool { return x == 4 }, false},
		{"Empty", []int{}, func(x int) bool { return true }, false},
		{"SingleElementFound", []int{42}, func(x int) bool { return x == 42 }, true},
		{"SingleElementNotFound", []int{42}, func(x int) bool { return x == 0 }, false},
		{"AllMatch", []int{2, 4, 6}, func(x int) bool { return x%2 == 0 }, true},
		{"NoneMatch", []int{1, 3, 5}, func(x int) bool { return x%2 == 0 }, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sliceutil.ContainsFunc(tt.collection, tt.predicate); got != tt.want {
				t.Errorf("ContainsFunc() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindIndex(t *testing.T) {
	tests := []struct {
		name       string
		collection []int
		predicate  func(int) bool
		want       int
	}{
		{"FoundFirst", []int{1, 2, 3}, func(x int) bool { return x == 1 }, 0},
		{"FoundMiddle", []int{1, 2, 3}, func(x int) bool { return x == 2 }, 1},
		{"FoundLast", []int{1, 2, 3}, func(x int) bool { return x == 3 }, 2},
		{"NotFound", []int{1, 2, 3}, func(x int) bool { return x == 4 }, -1},
		{"Empty", []int{}, func(x int) bool { return true }, -1},
		{"SingleElementFound", []int{42}, func(x int) bool { return x == 42 }, 0},
		{"SingleElementNotFound", []int{42}, func(x int) bool { return x == 0 }, -1},
		{"Duplicates", []int{1, 2, 2, 3}, func(x int) bool { return x == 2 }, 1}, // Should return first match
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sliceutil.FindIndex(tt.collection, tt.predicate); got != tt.want {
				t.Errorf("FindIndex() = %v, want %v", got, tt.want)
			}
		})
	}
}
