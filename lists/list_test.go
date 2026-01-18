package lists_test

import (
	"slices"
	"testing"

	"silo/lists"
)

// RunListTests is a reusable test suite for the List interface.
// It can be used to test any implementation of lists.List[T].
func RunListTests(t *testing.T, name string, factory func(vals ...int) lists.List[int]) {
	t.Helper()

	t.Run(name+"/Basic", func(t *testing.T) {
		l := factory()
		if !l.IsEmpty() {
			t.Error("New list should be empty")
		}
		if l.Size() != 0 {
			t.Errorf("New list size should be 0, got %d", l.Size())
		}

		l.Add(10, 20, 30)
		if l.IsEmpty() {
			t.Error("List should not be empty after Add")
		}
		if l.Size() != 3 {
			t.Errorf("Size should be 3, got %d", l.Size())
		}

		if v, err := l.Get(1); err != nil || v != 20 {
			t.Errorf("Get(1) = %d, %v; want 20, nil", v, err)
		}

		if err := l.Set(1, 25); err != nil {
			t.Errorf("Set(1) failed: %v", err)
		}
		if v, _ := l.Get(1); v != 25 {
			t.Errorf("Get(1) after Set = %d, want 25", v)
		}

		l.Clear()
		if !l.IsEmpty() {
			t.Error("List should be empty after Clear")
		}
		if l.Size() != 0 {
			t.Errorf("Size after Clear should be 0, got %d", l.Size())
		}
	})

	t.Run(name+"/Insert_Remove", func(t *testing.T) {
		l := factory(1, 2, 3)

		// Insert at middle
		if err := l.Insert(1, 10); err != nil {
			t.Fatalf("Insert(1, 10) failed: %v", err)
		}
		// Expect: [1, 10, 2, 3]
		want := []int{1, 10, 2, 3}
		if got := slices.Collect(l.Values()); !slices.Equal(got, want) {
			t.Errorf("After Insert: got %v, want %v", got, want)
		}

		// Insert at beginning
		if err := l.Insert(0, 0); err != nil {
			t.Fatalf("Insert(0, 0) failed: %v", err)
		}
		// Expect: [0, 1, 10, 2, 3]
		if v, _ := l.Get(0); v != 0 {
			t.Errorf("Index 0 should be 0, got %d", v)
		}

		// Insert at end
		if err := l.Insert(l.Size(), 99); err != nil {
			t.Fatalf("Insert(Size, 99) failed: %v", err)
		}
		// Expect: [0, 1, 10, 2, 3, 99]
		if v, _ := l.Get(l.Size() - 1); v != 99 {
			t.Errorf("Last element should be 99, got %d", v)
		}

		// Remove middle
		// Current: [0, 1, 10, 2, 3, 99]
		// Remove index 2 (value 10)
		val, err := l.Remove(2)
		if err != nil {
			t.Fatalf("Remove(2) failed: %v", err)
		}
		if val != 10 {
			t.Errorf("Remove(2) returned %d, want 10", val)
		}
		// Expect: [0, 1, 2, 3, 99]
		want = []int{0, 1, 2, 3, 99}
		if got := slices.Collect(l.Values()); !slices.Equal(got, want) {
			t.Errorf("After Remove: got %v, want %v", got, want)
		}
	})

	t.Run(name+"/Bulk_Operations", func(t *testing.T) {
		l := factory(1, 2, 3)

		// InsertAll at middle
		// [1, 2, 3] -> InsertAll(1, 8, 9) -> [1, 8, 9, 2, 3]
		if err := l.InsertAll(1, 8, 9); err != nil {
			t.Fatalf("InsertAll(1, ...) failed: %v", err)
		}
		want := []int{1, 8, 9, 2, 3}
		if got := slices.Collect(l.Values()); !slices.Equal(got, want) {
			t.Errorf("After InsertAll: got %v, want %v", got, want)
		}

		// InsertAll at end
		// [1, 8, 9, 2, 3] -> InsertAll(l.Size(), 4, 5) -> [1, 8, 9, 2, 3, 4, 5]
		if err := l.InsertAll(l.Size(), 4, 5); err != nil {
			t.Fatalf("InsertAll(Size, ...) failed: %v", err)
		}
		want = []int{1, 8, 9, 2, 3, 4, 5}
		if got := slices.Collect(l.Values()); !slices.Equal(got, want) {
			t.Errorf("After InsertAll at end: got %v, want %v", got, want)
		}

		// RemoveRange middle
		// [1, 8, 9, 2, 3, 4, 5] -> RemoveRange(1, 3) (removes 8, 9) -> [1, 2, 3, 4, 5]
		if err := l.RemoveRange(1, 3); err != nil {
			t.Fatalf("RemoveRange(1, 3) failed: %v", err)
		}
		want = []int{1, 2, 3, 4, 5}
		if got := slices.Collect(l.Values()); !slices.Equal(got, want) {
			t.Errorf("After RemoveRange: got %v, want %v", got, want)
		}

		// RemoveRange all
		if err := l.RemoveRange(0, l.Size()); err != nil {
			t.Fatalf("RemoveRange(0, Size) failed: %v", err)
		}
		if !l.IsEmpty() {
			t.Error("List should be empty after removing all elements")
		}

		// Boundary checks
		if err := l.InsertAll(-1, 1); err == nil {
			t.Error("InsertAll(-1) should fail")
		}
		if err := l.InsertAll(l.Size()+1, 1); err == nil {
			t.Error("InsertAll(Size+1) should fail")
		}

		// Test RemoveRange boundaries on a fresh list
		l2 := factory(1, 2, 3)
		if err := l2.RemoveRange(0, 4); err == nil {
			t.Error("RemoveRange end > len should fail")
		}
		if err := l2.RemoveRange(2, 1); err == nil {
			t.Error("RemoveRange start > end should fail")
		}
	})

	t.Run(name+"/Boundary_Empty", func(t *testing.T) {
		l := factory()

		// Accessors on empty list
		if _, err := l.Get(0); err == nil {
			t.Error("Get(0) on empty list should fail")
		}
		if err := l.Set(0, 1); err == nil {
			t.Error("Set(0) on empty list should fail")
		}
		if _, err := l.Remove(0); err == nil {
			t.Error("Remove(0) on empty list should fail")
		}

		// Deque operations on empty list
		if _, err := l.First(); err == nil {
			t.Error("First() on empty list should fail")
		}
		if _, err := l.Last(); err == nil {
			t.Error("Last() on empty list should fail")
		}
		if _, err := l.RemoveFirst(); err == nil {
			t.Error("RemoveFirst() on empty list should fail")
		}
		if _, err := l.RemoveLast(); err == nil {
			t.Error("RemoveLast() on empty list should fail")
		}
	})

	t.Run(name+"/Boundary_Indices", func(t *testing.T) {
		l := factory(1, 2, 3)
		size := l.Size()

		// Invalid indices for Get/Set/Remove
		invalidIndices := []int{-1, size, size + 1}
		for _, idx := range invalidIndices {
			if _, err := l.Get(idx); err == nil {
				t.Errorf("Get(%d) should fail", idx)
			}
			if err := l.Set(idx, 99); err == nil {
				t.Errorf("Set(%d) should fail", idx)
			}
			if _, err := l.Remove(idx); err == nil {
				t.Errorf("Remove(%d) should fail", idx)
			}
		}

		// Insert specific boundary checks
		// Insert allows index == size (append), but not size+1 or -1
		if err := l.Insert(-1, 99); err == nil {
			t.Error("Insert(-1) should fail")
		}
		if err := l.Insert(size+1, 99); err == nil {
			t.Error("Insert(size+1) should fail")
		}
	})

	t.Run(name+"/Swap", func(t *testing.T) {
		l := factory(1, 2, 3)

		// Valid swap
		l.Swap(0, 2)
		// Expect: [3, 2, 1]
		if v, _ := l.Get(0); v != 3 {
			t.Errorf("After Swap(0,2), index 0 = %d, want 3", v)
		}
		if v, _ := l.Get(2); v != 1 {
			t.Errorf("After Swap(0,2), index 2 = %d, want 1", v)
		}

		// Self swap (should be safe)
		l.Swap(1, 1)
		if v, _ := l.Get(1); v != 2 {
			t.Errorf("After Swap(1,1), index 1 = %d, want 2", v)
		}

		// Invalid swap (should be no-op)
		l.Swap(-1, 0)
		l.Swap(0, 5)

		// Verify state hasn't changed: [3, 2, 1]
		got := slices.Collect(l.Values())
		want := []int{3, 2, 1}
		if !slices.Equal(got, want) {
			t.Errorf("Invalid Swap modified list: got %v, want %v", got, want)
		}
	})

	t.Run(name+"/Deque", func(t *testing.T) {
		l := factory()
		l.AddFirst(1)
		l.AddFirst(2)
		// Expect: [2, 1]
		if v, _ := l.First(); v != 2 {
			t.Errorf("First() = %d, want 2", v)
		}
		if v, _ := l.Last(); v != 1 {
			t.Errorf("Last() = %d, want 1", v)
		}

		val, err := l.RemoveFirst()
		if err != nil || val != 2 {
			t.Errorf("RemoveFirst() = %d, %v; want 2, nil", val, err)
		}
		// Expect: [1]

		l.Add(3)
		// Expect: [1, 3]

		val, err = l.RemoveLast()
		if err != nil || val != 3 {
			t.Errorf("RemoveLast() = %d, %v; want 3, nil", val, err)
		}
		// Expect: [1]
	})

	t.Run(name+"/Functional", func(t *testing.T) {
		l := factory(1, 2, 3, 4, 5)

		// ContainsFunc
		if !l.ContainsFunc(func(x int) bool { return x == 3 }) {
			t.Error("ContainsFunc(3) should be true")
		}

		// IndexFunc
		if idx := l.IndexFunc(func(x int) bool { return x == 3 }); idx != 2 {
			t.Errorf("IndexFunc(3) = %d, want 2", idx)
		}

		// RemoveIf (remove odd numbers)
		removedCount := l.RemoveIf(func(x int) bool { return x%2 != 0 })
		// Removed 1, 3, 5. Remaining: 2, 4.
		if removedCount != 3 {
			t.Errorf("RemoveIf returned count %d, want 3", removedCount)
		}
		if l.Size() != 2 {
			t.Errorf("Size after RemoveIf = %d, want 2", l.Size())
		}
		want := []int{2, 4}
		if got := slices.Collect(l.Values()); !slices.Equal(got, want) {
			t.Errorf("After RemoveIf: got %v, want %v", got, want)
		}

		// Sort
		l2 := factory(5, 1, 3, 2, 4)
		l2.Sort(func(a, b int) int { return a - b })
		want2 := []int{1, 2, 3, 4, 5}
		if got := slices.Collect(l2.Values()); !slices.Equal(got, want2) {
			t.Errorf("After Sort: got %v, want %v", got, want2)
		}
	})
}

func TestArrayList_Specifics(t *testing.T) {
	t.Run("Clone", func(t *testing.T) {
		l := lists.NewArrayList[int](10)
		l.Add(1, 2, 3)
		clone := l.Clone()

		if l.Size() != clone.Size() {
			t.Errorf("Clone size mismatch: got %d, want %d", clone.Size(), l.Size())
		}
		if !slices.Equal(slices.Collect(l.Values()), slices.Collect(clone.Values())) {
			t.Error("Clone content mismatch")
		}

		// Verify independence
		l.Set(0, 99)
		v, _ := clone.Get(0)
		if v == 99 {
			t.Error("Clone should be independent of original")
		}
	})

	t.Run("String", func(t *testing.T) {
		l := lists.NewArrayList[int](0)
		l.Add(1, 2)
		if s := l.String(); s != "[1 2]" {
			t.Errorf("String() = %q, want \"[1 2]\"", s)
		}
	})

	t.Run("ResizeToFit", func(t *testing.T) {
		l := lists.NewArrayList[int](100)
		l.Add(1, 2, 3)
		l.ResizeToFit()
		// Verify data is still there
		if l.Size() != 3 {
			t.Errorf("Size changed after ResizeToFit")
		}
	})
}

func TestArrayList(t *testing.T) {
	RunListTests(t, "ArrayList", func(vals ...int) lists.List[int] {
		l := lists.NewArrayList[int](len(vals))
		l.Add(vals...)
		return l
	})
}
