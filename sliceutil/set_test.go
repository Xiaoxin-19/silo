package sliceutil_test

import (
	"reflect"
	"silo/sliceutil"
	"testing"
)

func TestIntersection(t *testing.T) {
	tests := []struct {
		name string
		a, b []int
		want []int
	}{
		{"Normal", []int{1, 2, 3}, []int{2, 3, 4}, []int{2, 3}},
		{"NoOverlap", []int{1, 2}, []int{3, 4}, []int{}},
		{"EmptyA", []int{}, []int{1, 2}, []int{}},
		{"EmptyB", []int{1, 2}, []int{}, []int{}},
		{"DuplicatesInA", []int{1, 2, 2, 3}, []int{2, 3}, []int{2, 3}}, // Should dedup result
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sliceutil.Intersection(tt.a, tt.b)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Intersection() = %v, want %v", got, tt.want)
			}
		})
	}
}

type User struct {
	ID   int
	Name string
}

func TestIntersectionBy(t *testing.T) {
	u1 := User{1, "Alice"}
	u2 := User{2, "Bob"}
	u3 := User{3, "Charlie"}

	keySelector := func(u User) int { return u.ID }

	a := []User{u1, u2}
	b := []User{u2, u3}
	got := sliceutil.IntersectionBy(a, b, keySelector)
	want := []User{u2}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("IntersectionBy() = %v, want %v", got, want)
	}
}

func TestSymmetricDifference(t *testing.T) {
	tests := []struct {
		name string
		a, b []int
		want []int
	}{
		{"Normal", []int{1, 2, 3}, []int{3, 4, 5}, []int{1, 2, 4, 5}},
		{"Disjoint", []int{1}, []int{2}, []int{1, 2}},
		{"Identical", []int{1, 2}, []int{1, 2}, []int{}},
		{"EmptyA", []int{}, []int{1}, []int{1}},
		{"EmptyB", []int{1}, []int{}, []int{1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sliceutil.SymmetricDifference(tt.a, tt.b)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SymmetricDifference() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSymmetricDifferenceBy(t *testing.T) {
	u1 := User{1, "Alice"}
	u2 := User{2, "Bob"}
	u3 := User{3, "Charlie"}

	keySelector := func(u User) int { return u.ID }

	got := sliceutil.SymmetricDifferenceBy([]User{u1, u2}, []User{u2, u3}, keySelector)
	want := []User{u1, u3}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SymmetricDifferenceBy() = %v, want %v", got, want)
	}
}

func TestDifference(t *testing.T) {
	tests := []struct {
		name string
		a, b []int
		want []int
	}{
		{"Normal", []int{1, 2, 3}, []int{2, 3, 4}, []int{1}},
		{"NoOverlap", []int{1, 2}, []int{3, 4}, []int{1, 2}},
		{"EmptyA", []int{}, []int{1, 2}, []int{}},
		{"EmptyB", []int{1, 2}, []int{}, []int{1, 2}},
		{"BothEmpty", []int{}, []int{}, []int{}},
		{"DuplicatesInA", []int{1, 2, 2, 3}, []int{2}, []int{1, 3}},
		{"DuplicatesInB", []int{1, 2}, []int{2, 2, 3}, []int{1}},
		{"ASubsetOfB", []int{1, 2}, []int{1, 2, 3}, []int{}},
		{"BSubsetOfA", []int{1, 2, 3}, []int{2}, []int{1, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sliceutil.Difference(tt.a, tt.b)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Difference() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDifferenceBy(t *testing.T) {
	u1 := User{1, "Alice"}
	u2 := User{2, "Bob"}
	u3 := User{3, "Charlie"}

	keySelector := func(u User) int { return u.ID }

	tests := []struct {
		name string
		a, b []User
		want []User
	}{
		{"Normal", []User{u1, u2}, []User{u2, u3}, []User{u1}},
		{"EmptyA", []User{}, []User{u1}, []User{}},
		{"EmptyB", []User{u1, u2}, []User{}, []User{u1, u2}},
		{"DuplicatesInA", []User{u1, u2, u2}, []User{u2}, []User{u1}},
		{"ASubsetOfB", []User{u1, u2}, []User{u1, u2, u3}, []User{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sliceutil.DifferenceBy(tt.a, tt.b, keySelector)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DifferenceBy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnique(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected []int
	}{
		{"Empty", []int{}, []int{}},
		{"NoDuplicates", []int{1, 2, 3}, []int{1, 2, 3}},
		{"WithDuplicates", []int{1, 2, 2, 3, 1}, []int{1, 2, 3}},
		{"AllSame", []int{1, 1, 1}, []int{1}},
		{"SingleElement", []int{1}, []int{1}},
		{"LargeDuplicates", []int{1, 2, 3, 1, 2, 3, 1, 2, 3}, []int{1, 2, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Copy input to avoid modifying original
			input := make([]int, len(tt.input))
			copy(input, tt.input)
			got := sliceutil.Unique(input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("Unique() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestUniqueBy(t *testing.T) {
	u1 := User{1, "Alice"}
	u2 := User{2, "Bob"}
	u3 := User{3, "Charlie"}
	u4 := User{1, "Alice2"} // Same ID as u1

	keySelector := func(u User) int { return u.ID }

	tests := []struct {
		name     string
		input    []User
		expected []User
	}{
		{"Empty", []User{}, []User{}},
		{"NoDuplicates", []User{u1, u2, u3}, []User{u1, u2, u3}},
		{"WithDuplicates", []User{u1, u2, u1}, []User{u1, u2}},
		{"AllSame", []User{u1, u4}, []User{u1}}, // u4 has same ID as u1
		{"SingleElement", []User{u1}, []User{u1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := make([]User, len(tt.input))
			copy(input, tt.input)
			got := sliceutil.UniqueBy(input, keySelector)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("UniqueBy() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestUnion(t *testing.T) {
	tests := []struct {
		name string
		a, b []int
		want []int
	}{
		{"Normal", []int{1, 2}, []int{2, 3}, []int{1, 2, 3}},
		{"NoOverlap", []int{1, 2}, []int{3, 4}, []int{1, 2, 3, 4}},
		{"EmptyA", []int{}, []int{1, 2}, []int{1, 2}},
		{"EmptyB", []int{1, 2}, []int{}, []int{1, 2}},
		{"BothEmpty", []int{}, []int{}, []int{}},
		{"DuplicatesInA", []int{1, 2, 2}, []int{2, 3}, []int{1, 2, 3}},
		{"DuplicatesInB", []int{1, 2}, []int{2, 2, 3}, []int{1, 2, 3}},
		{"Identical", []int{1, 2}, []int{1, 2}, []int{1, 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sliceutil.Union(tt.a, tt.b)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Union() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnionBy(t *testing.T) {
	u1 := User{1, "Alice"}
	u2 := User{2, "Bob"}
	u3 := User{3, "Charlie"}
	u4 := User{1, "Alice2"} // Same ID as u1

	keySelector := func(u User) int { return u.ID }

	tests := []struct {
		name string
		a, b []User
		want []User
	}{
		{"Normal", []User{u1, u2}, []User{u2, u3}, []User{u1, u2, u3}},
		{"EmptyA", []User{}, []User{u1}, []User{u1}},
		{"EmptyB", []User{u1}, []User{}, []User{u1}},
		{"Duplicates", []User{u1, u4}, []User{u2}, []User{u1, u2}}, // u4 has same ID as u1
		{"Identical", []User{u1, u2}, []User{u1, u2}, []User{u1, u2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sliceutil.UnionBy(tt.a, tt.b, keySelector)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UnionBy() = %v, want %v", got, tt.want)
			}
		})
	}
}
