package lists

import (
	"fmt"
	"iter"
)

var ErrInvalidOperation = fmt.Errorf("invalid operation on cursor")

/*
LinkedListCursor for iteration with modification support
*/
type LinkedListCursor[T any] struct {
	current *node[T]
	list    *LinkedList[T]
}

// IsValid checks if the cursor is at a valid element
func (llc *LinkedListCursor[T]) IsValid() bool {
	// Check if current is nil, list is nil, or current is a sentinel
	// Also check llc.current.next != nil to ensure the node hasn't been removed from the list
	// (removeNode sets next and prev to nil)
	return llc.current != nil && llc.current.next != nil && llc.list != nil && llc.current != llc.list.headSentinel && llc.current != llc.list.tailSentinel
}

// Value returns the value at the current cursor position
// If the cursor is invalid, it returns the zero value of T
func (llc *LinkedListCursor[T]) Value() (val T) {
	if !llc.IsValid() {
		return val
	}
	return llc.current.val
}

// Next moves the cursor to the next element
// If already at the end, it becomes invalid
func (llc *LinkedListCursor[T]) Next() {
	if llc.current == nil {
		return
	}
	// Stop at tailSentinel to allow Prev() recovery
	if llc.current == llc.list.tailSentinel {
		return
	}
	llc.current = llc.current.next
}

// Prev moves the cursor to the previous element
// If already at the beginning, it becomes invalid
func (llc *LinkedListCursor[T]) Prev() {
	if llc.current == nil {
		return
	}
	// Stop at headSentinel to allow Next() recovery
	if llc.current == llc.list.headSentinel {
		return
	}
	llc.current = llc.current.prev
}

// MoveTo moves the cursor to the specified index
// Note: This is an O(N) operation
func (llc *LinkedListCursor[T]) MoveTo(index int) error {
	if index < 0 || index >= llc.list.size {
		return ErrIndexOutOfBounds
	}
	llc.current = llc.list.findNodeAt(index)
	return nil
}

// Seek moves the cursor by the specified offset
// a positive offset moves forward, negative moves backward
// Note: This is an O(N) operation
func (llc *LinkedListCursor[T]) Seek(offset int) error {
	// Allow seeking from sentinels as long as the cursor is attached to a list
	if llc.current == nil || llc.list == nil {
		return ErrInvalidOperation
	}
	switch {
	case offset > 0:
		for range offset {
			llc.Next()
		}
	case offset < 0:
		for range -offset {
			llc.Prev()
		}
	}
	return nil
}

// Index returns the index of the current cursor position
// If the cursor is invalid, it returns -1
// Note: This is an O(N) operation
func (llc *LinkedListCursor[T]) Index() int {
	if !llc.IsValid() {
		return -1
	}
	index := 0
	current := llc.list.headSentinel.next
	for current != llc.current {
		if current == llc.list.tailSentinel {
			return -1 // Node not found in list (detached or list empty)
		}
		current = current.next
		index++
	}
	return index
}

// Set sets the value at the current cursor position
// If the cursor is invalid, it returns an error
func (llc *LinkedListCursor[T]) Set(value T) error {
	if !llc.IsValid() {
		return ErrInvalidOperation
	}
	llc.current.val = value
	return nil
}

// Remove removes the element at the current cursor position.
// After removal, the cursor moves to the next element.
// If the cursor is invalid, it returns an error
func (llc *LinkedListCursor[T]) Remove() (val T, err error) {
	if !llc.IsValid() {
		return val, ErrInvalidOperation
	}
	// Capture the next node before removing the current one
	next := llc.current.next
	val = llc.list.removeNode(llc.current)

	// Move cursor to the next node (even if it is the tail sentinel)
	llc.current = next
	return val, nil
}

// InsertAfter inserts a new element after the current cursor position
// If the cursor is invalid, it returns an error
// If want insert elem to blank list, use InsertBefore on tailSentinel cursor
// Example:
//
//	l := lists.NewLinkedList[int]()
//	c := l.BackCursor() // points to tailSentinel
//	c.InsertBefore(1)   // inserts 1 into the empty list
func (llc *LinkedListCursor[T]) InsertAfter(value T) error {
	if !llc.IsValid() {
		return ErrInvalidOperation
	}
	newNode := &node[T]{val: value}
	llc.list.insertNodeAt(llc.current, newNode)
	return nil
}

// InsertBefore inserts a new element before the current cursor position
// If the cursor is invalid, it returns an error
func (llc *LinkedListCursor[T]) InsertBefore(value T) error {
	// Allow inserting before a valid element OR before the tail sentinel (append)
	if !llc.IsValid() && llc.current != llc.list.tailSentinel {
		return ErrInvalidOperation
	}
	newNode := &node[T]{val: value}
	llc.list.insertNodeAt(llc.current.prev, newNode)
	return nil
}

// String returns a string representation of the cursor
func (llc *LinkedListCursor[T]) String() string {
	if llc.IsValid() {
		return fmt.Sprintf("Cursor[%v]", llc.current.val)
	}
	return "Cursor[invalid]"
}

// Seq returns a sequence that iterates from the current cursor position
func (llc *LinkedListCursor[T]) Seq() iter.Seq[T] {
	return func(yield func(T) bool) {
		if !llc.IsValid() {
			return
		}
		// start from current position, cursor is not modified
		current := llc.current
		for current != llc.list.tailSentinel && current != nil {
			if !yield(current.val) {
				break
			}
			current = current.next
		}
	}
}

// BackwardSeq returns a sequence that iterates backwards from the current cursor position
func (llc *LinkedListCursor[T]) BackwardSeq() iter.Seq[T] {
	return func(yield func(T) bool) {
		if !llc.IsValid() {
			return
		}
		// start from current position, cursor is not modified
		current := llc.current
		for current != llc.list.headSentinel && current != nil {
			if !yield(current.val) {
				break
			}
			current = current.prev
		}
	}
}

// Clone creates a shallow copy of the cursor at the same position
func (llc *LinkedListCursor[T]) Clone() *LinkedListCursor[T] {
	return &LinkedListCursor[T]{
		current: llc.current,
		list:    llc.list,
	}
}

// CursorAt returns a cursor positioned at the specified index
// If the index is out of bounds, it returns an error
func (ll *LinkedList[T]) CursorAt(index int) (*LinkedListCursor[T], error) {
	if index < 0 || index >= ll.size {
		return nil, ErrIndexOutOfBounds
	}
	targetNode := ll.findNodeAt(index)
	return &LinkedListCursor[T]{
		current: targetNode,
		list:    ll,
	}, nil
}

// FrontCursor returns a cursor positioned at the first element
// If the list is empty, it returns an invalid cursor
func (ll *LinkedList[T]) FrontCursor() *LinkedListCursor[T] {
	// Always return head.next, even if it is tailSentinel (empty list)
	// This maintains the cursor's link to the list structure
	return &LinkedListCursor[T]{current: ll.headSentinel.next, list: ll}
}

// BackCursor returns a cursor positioned at the last element
// If the list is empty, it returns an invalid cursor
func (ll *LinkedList[T]) BackCursor() *LinkedListCursor[T] {
	// Always return tail.prev, even if it is headSentinel (empty list)
	// This maintains the cursor's link to the list structure
	return &LinkedListCursor[T]{current: ll.tailSentinel.prev, list: ll}
}
