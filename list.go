// Package implicittreaplist provides a concurrency-safe list of strings
// backed by an implicit treap.
//
// The list supports index-based insertion, half-open interval deletion and
// reversal, point reads, and O(n) snapshots. Structural operations run in
// O(log n) expected time. Priorities are derived from a strictly increasing
// internal sequence through a fixed deterministic mixing function, so the
// treap shape is reproducible for a given operation history.
package implicittreaplist

import (
	"errors"
	"fmt"
	"sync"
)

// ErrClosed is returned by mutating operations after Close has been called.
var ErrClosed = errors.New("implicittreaplist: list is closed")

// mix64 is a fixed deterministic mixing function (a splitmix64-style finalizer).
func mix64(x uint64) uint64 {
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	x ^= x >> 31
	return x
}

type node struct {
	value    string
	priority uint64
	size     int
	rev      bool
	left     *node
	right    *node
}

func sizeOf(n *node) int {
	if n == nil {
		return 0
	}
	return n.size
}

// pull recomputes n.size from its children. Callers must invoke pull after
// any structural change below n and after any lazy push swaps its children.
func (n *node) pull() {
	n.size = 1 + sizeOf(n.left) + sizeOf(n.right)
}

// push applies a pending lazy reverse flag: children are swapped in O(1) and
// the flag propagates downward. The node's own size is unchanged.
func (n *node) push() {
	if n == nil || !n.rev {
		return
	}
	n.left, n.right = n.right, n.left
	if n.left != nil {
		n.left.rev = !n.left.rev
	}
	if n.right != nil {
		n.right.rev = !n.right.rev
	}
	n.rev = false
}

// split divides t into [0,k) and [k,size) by implicit position.
func split(t *node, k int) (l, r *node) {
	if t == nil {
		return nil, nil
	}
	t.push()
	if sizeOf(t.left) >= k {
		l, t.left = split(t.left, k)
		t.pull()
		return l, t
	}
	t.right, r = split(t.right, k-sizeOf(t.left)-1)
	t.pull()
	return t, r
}

// merge combines treaps a and b, where every in-order position of a precedes
// every position of b.
func merge(a, b *node) *node {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if a.priority < b.priority {
		a.push()
		a.right = merge(a.right, b)
		a.pull()
		return a
	}
	b.push()
	b.left = merge(a, b.left)
	b.pull()
	return b
}

// free drops the whole subtree rooted at t, letting nodes be collected.
func free(t *node) {
	if t == nil {
		return
	}
	free(t.left)
	free(t.right)
	t.left = nil
	t.right = nil
}

// List is a concurrency-safe implicit treap holding a sequence of strings.
// The zero value is ready to use.
type List struct {
	mu     sync.RWMutex
	root   *node
	length int
	seq    uint64
	closed bool
}

// Insert inserts the non-empty slice values so that the first element lands
// at position index, shifting existing elements right. It requires
// 0 <= index <= Len().
func (l *List) Insert(index int, values []string) error {
	if len(values) == 0 {
		return errors.New("implicittreaplist: Insert requires a non-empty values slice")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return ErrClosed
	}
	if index < 0 || index > l.length {
		return fmt.Errorf("implicittreaplist: Insert index %d out of range [0,%d]", index, l.length)
	}
	var mid *node
	for _, v := range values {
		l.seq++
		mid = merge(mid, &node{value: v, priority: mix64(l.seq), size: 1})
	}
	left, right := split(l.root, index)
	l.root = merge(merge(left, mid), right)
	l.length += len(values)
	return nil
}

// Delete removes the half-open interval [left,right). It requires
// 0 <= left < right <= Len().
func (l *List) Delete(left, right int) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return ErrClosed
	}
	if err := l.checkRange(left, right); err != nil {
		return err
	}
	a, bc := split(l.root, left)
	b, c := split(bc, right-left)
	free(b)
	l.root = merge(a, c)
	l.length -= right - left
	return nil
}

// Reverse reverses the half-open interval [left,right) in place using lazy
// propagation, without visiting each element. It requires
// 0 <= left < right <= Len().
func (l *List) Reverse(left, right int) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return ErrClosed
	}
	if err := l.checkRange(left, right); err != nil {
		return err
	}
	a, bc := split(l.root, left)
	b, c := split(bc, right-left)
	if b != nil {
		b.rev = !b.rev
	}
	l.root = merge(a, merge(b, c))
	return nil
}

// checkRange validates a half-open interval against the current length.
// Callers must hold the lock.
func (l *List) checkRange(left, right int) error {
	if left < 0 || left >= right || right > l.length {
		return fmt.Errorf("implicittreaplist: invalid range [%d,%d) for length %d, require 0<=left<right<=%d", left, right, l.length, l.length)
	}
	return nil
}

// Get returns the element at index. It requires 0 <= index < Len().
func (l *List) Get(index int) (string, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if index < 0 || index >= l.length {
		return "", fmt.Errorf("implicittreaplist: Get index %d out of range [0,%d)", index, l.length)
	}
	n := l.root
	for {
		n.push()
		if sizeOf(n.left) > index {
			n = n.left
		} else if sizeOf(n.left) == index {
			return n.value, nil
		} else {
			index -= sizeOf(n.left) + 1
			n = n.right
		}
	}
}

// Snapshot returns an independent copy of the current sequence in O(n).
// Mutating the result does not affect the list.
func (l *List) Snapshot() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]string, 0, l.length)
	var walk func(n *node)
	walk = func(n *node) {
		if n == nil {
			return
		}
		n.push()
		walk(n.left)
		out = append(out, n.value)
		walk(n.right)
	}
	walk(l.root)
	return out
}

// Len returns the number of elements in the list.
func (l *List) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.length
}

// Close marks the list closed. Insert, Delete and Reverse are rejected with
// ErrClosed afterwards, while Get, Snapshot and Len keep working. Close is
// idempotent.
func (l *List) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.closed = true
}
