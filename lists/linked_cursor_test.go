package lists_test

import (
	"slices"
	"testing"

	"silo/lists"
)

// TestCursor_Navigation 测试基本的游标移动和值获取
func TestCursor_Navigation(t *testing.T) {
	l := lists.NewLinkedList[int]()
	l.Add(1, 2, 3)

	// 1. Front Cursor
	c := l.FrontCursor()
	if !c.IsValid() {
		t.Fatal("Front cursor should be valid")
	}
	if c.Value() != 1 {
		t.Errorf("Expected 1, got %d", c.Value())
	}

	// 2. Next
	c.Next()
	if c.Value() != 2 {
		t.Errorf("Expected 2, got %d", c.Value())
	}

	// 3. Back Cursor
	c2 := l.BackCursor()
	if c2.Value() != 3 {
		t.Errorf("Expected 3, got %d", c2.Value())
	}

	// 4. Prev
	c2.Prev()
	if c2.Value() != 2 {
		t.Errorf("Expected 2, got %d", c2.Value())
	}

	// 5. Move past end (Tail Sentinel)
	c = l.BackCursor() // at 3
	c.Next()           // at tailSentinel (invalid)
	if c.IsValid() {
		t.Error("Cursor at tail sentinel should be invalid")
	}

	// 6. Recovery from tail sentinel
	c.Prev()
	if !c.IsValid() || c.Value() != 3 {
		t.Error("Should recover from tail sentinel")
	}

	// 7. Move before start (Head Sentinel)
	c = l.FrontCursor() // at 1
	c.Prev()            // at headSentinel (invalid)
	if c.IsValid() {
		t.Error("Cursor at head sentinel should be invalid")
	}

	// 8. Recovery from head sentinel
	c.Next()
	if !c.IsValid() || c.Value() != 1 {
		t.Error("Should recover from head sentinel")
	}
}

// TestCursor_Modification_Insert 测试通过游标插入元素
func TestCursor_Modification_Insert(t *testing.T) {
	l := lists.NewLinkedList[int]()
	l.Add(1, 3)

	c := l.FrontCursor() // at 1

	// 1. InsertAfter
	if err := c.InsertAfter(2); err != nil {
		t.Fatalf("InsertAfter failed: %v", err)
	}
	// List: 1, 2, 3. Cursor at 1.
	if c.Value() != 1 {
		t.Errorf("Cursor moved unexpectedly")
	}

	c.Next() // at 2
	if c.Value() != 2 {
		t.Errorf("Expected 2, got %d", c.Value())
	}

	// 2. InsertBefore
	c.Next() // at 3
	if err := c.InsertBefore(25); err != nil {
		t.Fatalf("InsertBefore failed: %v", err)
	}
	// List: 1, 2, 25, 3. Cursor at 3.
	c.Prev() // at 25
	if c.Value() != 25 {
		t.Errorf("Expected 25, got %d", c.Value())
	}

	// 3. InsertBefore at tail (Append)
	c = l.BackCursor() // at 3
	c.Next()           // at tailSentinel (invalid)
	if c.IsValid() {
		t.Fatal("Should be invalid at tail")
	}
	// InsertBefore on tail sentinel is allowed (special case for append)
	if err := c.InsertBefore(4); err != nil {
		t.Fatalf("InsertBefore at tail failed: %v", err)
	}
	// List: ..., 3, 4. Cursor stays at tailSentinel.

	c.Prev() // Should be at 4
	if c.Value() != 4 {
		t.Errorf("Expected 4, got %d", c.Value())
	}
}

// TestCursor_Modification_Remove 测试通过游标删除元素
func TestCursor_Modification_Remove(t *testing.T) {
	l := lists.NewLinkedList[int]()
	l.Add(1, 2, 3)

	c := l.FrontCursor()
	c.Next() // at 2

	// 1. Remove middle element
	val, err := c.Remove()
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	if val != 2 {
		t.Errorf("Removed %d, want 2", val)
	}

	// Cursor should move to next (3)
	if c.Value() != 3 {
		t.Errorf("Cursor should be at 3, got %d", c.Value())
	}

	// 2. Remove last element
	val, err = c.Remove() // removes 3
	if val != 3 {
		t.Errorf("Removed %d, want 3", val)
	}

	// Cursor should be at tailSentinel (invalid)
	if c.IsValid() {
		t.Error("Cursor should be invalid after removing last element")
	}
}

// TestCursor_Seek_MoveTo 测试随机访问和跳转
func TestCursor_Seek_MoveTo(t *testing.T) {
	l := lists.NewLinkedList[int]()
	for i := 0; i < 10; i++ {
		l.Add(i)
	}

	c := l.FrontCursor()

	// 1. Seek forward
	if err := c.Seek(5); err != nil {
		t.Fatalf("Seek(5) failed: %v", err)
	}
	if c.Value() != 5 {
		t.Errorf("Expected 5, got %d", c.Value())
	}

	// 2. Seek backward
	if err := c.Seek(-2); err != nil {
		t.Fatalf("Seek(-2) failed: %v", err)
	}
	if c.Value() != 3 {
		t.Errorf("Expected 3, got %d", c.Value())
	}

	// 3. MoveTo
	if err := c.MoveTo(8); err != nil {
		t.Fatalf("MoveTo(8) failed: %v", err)
	}
	if c.Value() != 8 {
		t.Errorf("Expected 8, got %d", c.Value())
	}

	// 4. Index
	if idx := c.Index(); idx != 8 {
		t.Errorf("Index() = %d, want 8", idx)
	}
}

// TestCursor_Safety_NodeRemoval 测试当节点被外部移除时的安全性（防止 Panic）
func TestCursor_Safety_NodeRemoval(t *testing.T) {
	l := lists.NewLinkedList[int]()
	l.Add(10, 20, 30)

	c1 := l.FrontCursor() // at 10
	c1.Next()             // at 20

	// 模拟外部操作：通过 List API 移除节点 20
	l.Remove(1)

	// c1 现在指向一个被移除的节点（游离节点）
	// 它的 next 和 prev 指针应该已经被置为 nil

	// 1. IsValid 应该返回 false
	if c1.IsValid() {
		t.Error("Cursor pointing to removed node should be invalid")
	}

	// 2. 操作应该优雅失败，而不是 Panic
	if err := c1.InsertAfter(99); err == nil {
		t.Error("InsertAfter on removed node should fail")
	}

	if err := c1.Set(99); err == nil {
		t.Error("Set on removed node should fail")
	}

	// 3. 导航应该安全处理
	// Next() 在 nil 节点上应该安全返回或变为 nil
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Next() panicked on removed node: %v", r)
			}
		}()
		c1.Next()
	}()

	if c1.IsValid() {
		t.Error("Cursor should remain invalid after Next() on removed node")
	}
}

// TestCursor_EmptyList 测试空链表的边界情况
func TestCursor_EmptyList(t *testing.T) {
	l := lists.NewLinkedList[int]()
	c := l.FrontCursor()

	// 1. IsValid
	if c.IsValid() {
		t.Error("Cursor on empty list should be invalid")
	}

	// 2. InsertAfter (需要有效节点)
	if err := c.InsertAfter(1); err == nil {
		t.Error("InsertAfter on empty list cursor should fail")
	}

	// 3. InsertBefore (允许在 tailSentinel 前插入，即空链表插入)
	// FrontCursor 在空链表中指向 tailSentinel
	if err := c.InsertBefore(1); err != nil {
		t.Errorf("InsertBefore on empty list (tail sentinel) should succeed, got %v", err)
	}

	// 验证插入结果
	if l.Size() != 1 {
		t.Errorf("List size should be 1, got %d", l.Size())
	}
	if v, _ := l.Get(0); v != 1 {
		t.Errorf("Element should be 1, got %d", v)
	}
}

// TestCursor_Iterators 测试 Go 1.23 迭代器支持
func TestCursor_Iterators(t *testing.T) {
	l := lists.NewLinkedList[int]()
	l.Add(1, 2, 3, 4, 5)

	c := l.FrontCursor()
	c.Seek(2) // at 3

	// 1. Seq from 3: [3, 4, 5]
	got := slices.Collect(c.Seq())
	want := []int{3, 4, 5}
	if !slices.Equal(got, want) {
		t.Errorf("Seq() got %v, want %v", got, want)
	}

	// 2. BackwardSeq from 3: [3, 2, 1]
	gotBack := slices.Collect(c.BackwardSeq())
	wantBack := []int{3, 2, 1}
	if !slices.Equal(gotBack, wantBack) {
		t.Errorf("BackwardSeq() got %v, want %v", gotBack, wantBack)
	}
}

// TestCursor_Clone 测试游标克隆的独立性
func TestCursor_Clone(t *testing.T) {
	l := lists.NewLinkedList[int]()
	l.Add(1, 2, 3)
	c1 := l.FrontCursor()
	c2 := c1.Clone()

	c1.Next()
	if c1.Value() != 2 {
		t.Error("c1 should move")
	}
	if c2.Value() != 1 {
		t.Error("c2 should stay at original position")
	}
}
