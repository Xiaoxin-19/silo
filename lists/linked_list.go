package lists

import (
	"fmt"
	"iter"
	"slices"
	"strings"
)

type node[T any] struct {
	prev *node[T]
	next *node[T]
	val  T
}

type LinkedList[T any] struct {
	headSentinel *node[T]
	tailSentinel *node[T]
	size         int
}

func NewLinkedList[T any]() *LinkedList[T] {
	ll := &LinkedList[T]{
		headSentinel: &node[T]{},
		tailSentinel: &node[T]{},
		size:         0,
	}
	ll.headSentinel.next = ll.tailSentinel
	ll.tailSentinel.prev = ll.headSentinel
	return ll
}

// insertNodeAt insert newNode after indexNode
// Bounds checking should be done by the caller.
func (ll *LinkedList[T]) insertNodeAt(indexNode *node[T], newNode *node[T]) {
	newNode.prev = indexNode
	newNode.next = indexNode.next
	indexNode.next.prev = newNode
	indexNode.next = newNode
	ll.size++
}

// findNodeAt returns the node at the specified index.
// Assumes index is valid (0 <= index <= ll.size).
// Bounds checking should be done by the caller.
func (ll *LinkedList[T]) findNodeAt(index int) *node[T] {
	if index == ll.size {
		return ll.tailSentinel
	}
	// Optimize traversal: start from head or tail depending on index
	if index < ll.size/2 {
		current := ll.headSentinel.next
		for range index {
			current = current.next
		}
		return current
	} else {
		current := ll.tailSentinel.prev
		for i := ll.size - 1; i > index; i-- {
			current = current.prev
		}
		return current
	}
}

// removeNode removes the specified node from the list and returns its value.
// Bounds checking should be done by the caller.
// After removal, the node's pointers are cleared to help with garbage collection, value is zeroed.
func (ll *LinkedList[T]) removeNode(targetNode *node[T]) T {
	targetNode.prev.next = targetNode.next
	targetNode.next.prev = targetNode.prev
	res := targetNode.val
	// Help GC
	targetNode.prev = nil
	targetNode.next = nil
	var zero T
	targetNode.val = zero
	ll.size--
	return res
}

// Add appends values to the end of the list.
func (ll *LinkedList[T]) Add(values ...T) {
	for _, value := range values {
		newNode := &node[T]{val: value}
		ll.insertNodeAt(ll.tailSentinel.prev, newNode)
	}
}

// Insert inserts value at the specified index.
// Assumes index is valid (0 <= index <= ll.size).
func (ll *LinkedList[T]) Insert(index int, value T) error {
	if index < 0 || index > ll.size {
		return ErrIndexOutOfBounds
	}

	targetNode := ll.findNodeAt(index)
	newNode := &node[T]{val: value}
	ll.insertNodeAt(targetNode.prev, newNode)
	return nil
}

// InsertAll inserts multiple values at the specified index.
// Assumes index is valid (0 <= index <= ll.size).
func (ll *LinkedList[T]) InsertAll(index int, values ...T) error {
	if index < 0 || index > ll.size {
		return ErrIndexOutOfBounds
	}
	if len(values) == 0 {
		return nil
	}

	targetNode := ll.findNodeAt(index)
	for _, value := range values {
		newNode := &node[T]{val: value}
		ll.insertNodeAt(targetNode.prev, newNode)
	}
	return nil
}

// Get retrieves the element at the specified index.
// Assumes index is valid (0 <= index < ll.size).
func (ll *LinkedList[T]) Get(index int) (val T, err error) {
	if index < 0 || index >= ll.size {
		return val, ErrIndexOutOfBounds
	}
	node := ll.findNodeAt(index)
	return node.val, nil
}

// Set replaces the element at the specified index.
// Assumes index is valid (0 <= index < ll.size).
func (ll *LinkedList[T]) Set(index int, value T) error {
	if index < 0 || index >= ll.size {
		return ErrIndexOutOfBounds
	}
	node := ll.findNodeAt(index)
	node.val = value
	return nil
}

// Remove removes the element at the specified index.
// Assumes index is valid (0 <= index < ll.size).
func (ll *LinkedList[T]) Remove(index int) (T, error) {
	var zero T
	if index < 0 || index >= ll.size {
		return zero, ErrIndexOutOfBounds
	}
	targetNode := ll.findNodeAt(index)
	return ll.removeNode(targetNode), nil
}

// RemoveRange removes elements from start (inclusive) to end (exclusive).
// Assumes start and end are valid (0 <= start <= end <= ll.size).
func (ll *LinkedList[T]) RemoveRange(start, end int) error {
	if start < 0 || end > ll.size || start > end {
		return ErrIndexOutOfBounds
	}
	if start == end {
		return nil
	}

	startNode := ll.findNodeAt(start)
	endNode := ll.findNodeAt(end)

	// Unlink the range [startNode, endNode)
	startNode.prev.next = endNode
	endNode.prev = startNode.prev
	// Help GC
	var zero T
	current := startNode
	for current != endNode {
		next := current.next
		current.prev = nil
		current.next = nil
		current.val = zero
		current = next
	}

	ll.size -= end - start
	return nil
}

func (ll *LinkedList[T]) Swap(i, j int) {
	if i < 0 || i >= ll.size || j < 0 || j >= ll.size {
		return
	}
	nodeI := ll.findNodeAt(i)
	nodeJ := ll.findNodeAt(j)
	nodeI.val, nodeJ.val = nodeJ.val, nodeI.val
}

// AddFirst prepends a value to the list.
func (ll *LinkedList[T]) AddFirst(value T) {
	newNode := &node[T]{val: value}
	ll.insertNodeAt(ll.headSentinel, newNode)
}

// RemoveFirst removes and returns the first element.
func (ll *LinkedList[T]) RemoveFirst() (T, error) {
	return ll.Remove(0)
}

// RemoveLast removes and returns the last element.
func (ll *LinkedList[T]) RemoveLast() (T, error) {
	return ll.Remove(ll.size - 1)
}

// First returns the first element without removing it.
func (ll *LinkedList[T]) First() (T, error) {
	return ll.Get(0)
}

// Last returns the last element without removing it.
func (ll *LinkedList[T]) Last() (T, error) {
	return ll.Get(ll.size - 1)
}

// RemoveIf removes all elements satisfying the predicate.
// This allows O(N) filtering for both ArrayList and LinkedList.
// Returns the number of removed elements.
func (ll *LinkedList[T]) RemoveIf(predicate func(T) bool) int {
	removedCount := 0
	current := ll.headSentinel.next
	for current != ll.tailSentinel {
		next := current.next
		if predicate(current.val) {
			ll.removeNode(current)
			removedCount++
		}
		current = next
	}
	return removedCount
}

// Sort sorts the list in-place.
// implementation chooses the best algorithm (QuickSort for Array, MergeSort for Linked).
// the resulting list is stable.
func (ll *LinkedList[T]) Sort(compare func(a, b T) int) {
	if ll.size < 2 {
		return
	}

	// Optimization: Use slice-based sort for small datasets (e.g. < 64).
	// This improves cache locality and avoids pointer chasing overhead.
	// We use SortStableFunc to maintain the stability guarantee of MergeSort.
	if ll.size < 64 {
		vals := make([]T, 0, ll.size)
		current := ll.headSentinel.next
		for current != ll.tailSentinel {
			vals = append(vals, current.val)
			current = current.next
		}
		slices.SortStableFunc(vals, compare)
		current = ll.headSentinel.next
		for _, v := range vals {
			current.val = v
			current = current.next
		}
		return
	}

	// 1. Detach the list from sentinels to treat as a simple chain
	first := ll.headSentinel.next
	ll.tailSentinel.prev.next = nil // Break the link to tailSentinel

	// 2. Perform Merge Sort
	sortedHead := mergeSort(first, compare)

	// 3. Reconstruct the doubly linked list (fix prev pointers and sentinels)
	current := sortedHead
	prev := ll.headSentinel
	ll.headSentinel.next = current

	for current != nil {
		current.prev = prev
		prev = current
		current = current.next
	}

	// Fix tail sentinel
	prev.next = ll.tailSentinel
	ll.tailSentinel.prev = prev
}

func mergeSort[T any](head *node[T], compare func(a, b T) int) *node[T] {
	if head == nil || head.next == nil {
		return head
	}

	// Find middle using slow/fast pointers
	slow, fast := head, head.next
	for fast != nil && fast.next != nil {
		slow = slow.next
		fast = fast.next.next
	}

	mid := slow.next
	slow.next = nil // Split the list

	left := mergeSort(head, compare)
	right := mergeSort(mid, compare)

	return merge(left, right, compare)
}

func merge[T any](a, b *node[T], compare func(a, b T) int) *node[T] {
	dummy := &node[T]{}
	tail := dummy

	for a != nil && b != nil {
		if compare(a.val, b.val) <= 0 {
			tail.next = a
			a = a.next
		} else {
			tail.next = b
			b = b.next
		}
		tail = tail.next
	}

	if a != nil {
		tail.next = a
	} else {
		tail.next = b
	}

	return dummy.next
}

func (ll *LinkedList[T]) Size() int {
	return ll.size
}

func (ll *LinkedList[T]) IsEmpty() bool {
	return ll.size == 0
}

func (ll *LinkedList[T]) Clear() {
	// Clear all nodes to help GC
	current := ll.headSentinel.next
	var zero T
	for current != ll.tailSentinel {
		next := current.next
		current.prev = nil
		current.next = nil
		current.val = zero
		current = next
	}
	ll.headSentinel.next = ll.tailSentinel
	ll.tailSentinel.prev = ll.headSentinel
	ll.size = 0
}

func (ll *LinkedList[T]) ContainsFunc(predicate func(T) bool) bool {
	current := ll.headSentinel.next
	for current != ll.tailSentinel {
		if predicate(current.val) {
			return true
		}
		current = current.next
	}
	return false
}

func (ll *LinkedList[T]) IndexFunc(predicate func(T) bool) int {
	current := ll.headSentinel.next
	index := 0
	for current != ll.tailSentinel {
		if predicate(current.val) {
			return index
		}
		current = current.next
		index++
	}
	return -1
}

func (ll *LinkedList[T]) Values() iter.Seq[T] {
	return func(yield func(T) bool) {
		current := ll.headSentinel.next
		for current != ll.tailSentinel {
			if !yield(current.val) {
				break
			}
			current = current.next
		}
	}
}

func (ll *LinkedList[T]) All() iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		current := ll.headSentinel.next
		index := 0
		for current != ll.tailSentinel {
			if !yield(index, current.val) {
				break
			}
			current = current.next
			index++
		}
	}
}

func (ll *LinkedList[T]) Backward() iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		current := ll.tailSentinel.prev
		index := ll.size - 1
		for current != ll.headSentinel {
			if !yield(index, current.val) {
				break
			}
			current = current.prev
			index--
		}
	}
}

func (ll *LinkedList[T]) String() string {
	// use strBuilder for better performance
	strBuilder := strings.Builder{}
	strBuilder.WriteString("[")
	current := ll.headSentinel.next
	for current != ll.tailSentinel {
		strBuilder.WriteString(fmt.Sprintf("%v", current.val))
		if current.next != ll.tailSentinel {
			strBuilder.WriteString(", ")
		}
		current = current.next
	}
	strBuilder.WriteString("]")
	return strBuilder.String()
}

// Clone returns a shallow copy of the list.
// It allocates new nodes and copies the elements.
// Note: If T is a pointer or reference type, the referenced data is shared.
func (ll *LinkedList[T]) Clone() LinkedList[T] {
	clone := NewLinkedList[T]()
	current := ll.headSentinel.next
	for current != ll.tailSentinel {
		clone.Add(current.val)
		current = current.next
	}
	return *clone
}
