package silces_test

import (
	slices "silo/silces"
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
			if got := slices.Contains(tt.input, tt.target); got != tt.want {
				t.Errorf("Contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFind(t *testing.T) {
	input := []int{1, 2, 3, 4, 5}

	t.Run("Found", func(t *testing.T) {
		val, found := slices.Find(input, func(x int) bool { return x > 3 })
		if !found || val != 4 {
			t.Errorf("Find() = (%v, %v), want (4, true)", val, found)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		val, found := slices.Find(input, func(x int) bool { return x > 10 })
		if found {
			t.Errorf("Find() should not find element > 10, got %v", val)
		}
	})
}
