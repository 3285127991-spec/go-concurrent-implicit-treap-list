// Package implicittreaplist provides a concurrency-safe list of strings
// backed by an implicit treap with lazy range reversal.
//
// Every node stores its subtree size, so split/merge can locate positions
// by index in O(log n) expected time. Range reversal only toggles a lazy
// flag on the root of the affected subtree instead of swapping elements
// one by one. Node priorities are derived from a strictly increasing
// internal sequence through a fixed deterministic mix function, so the
// tree shape is fully reproducible for a given operation history.
package implicittreaplist

import (
	"errors"
	"sync"
)

var (
	// ErrIndexOutOfRange is returned when an index is outside [0, Len].
	ErrIndexOutOfRange = errors.New("implicittreaplist: index out of range")
	// ErrInvalidRange is returned when a half-open range [left, right)
	// does not satisfy 0 <= left < right <= Len.
	ErrInvalidRange = errors.New("implicittreaplist: invalid range, need 0 <= left < right <= Len")
	// ErrEmptyValues is returned by Insert when no values are given.
	ErrEmptyValues = errors.New("implicittreaplist: values must not be empty")
	// ErrClosed is returned by mutating operations after Close.
	ErrClosed = errors.New("implicittreaplist: list is closed")
)

type node struct {
	value    string
	priority uint64
	size     int
	rev      bool // lazy reverse flag for the whole subtree
	left     *node
	right    *node
}

// mix is a fixed deterministic hash (splitmix64 finalizer). Priorities are
// mix(seq) where seq strictly increases per node, making the resulting
// treap shape reproducible across runs.
func mix(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

// List is a concurrency-safe implicit treap list of strings.
type List struct {
	mu     sync.RWMutex
	root   *node
	seq    uint64
	closed bool
}

// New returns an empty list.
func New() *List { return &List{} }

func size(n *node) int {
	if n == nil {
		return 0
	}
	return n.size
}

func update(n *node) {
	n.size = 1 + size(n.left) + size(n.right)
}

// push propagates the lazy reverse flag one level down. It preserves the
// logical sequence of the subtree.
func push(n *node) {
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

// split divides the sequence rooted at n into [0, k) and [k, size).
func split(n *node, k int) (*node, *node) {
	if n == nil {
		return nil, nil
	}
	push(n)
	if size(n.left) >= k {
		l, r := split(n.left, k)
		n.left = r
		update(n)
		return l, n
	}
	l, r := split(n.right, k-size(n.left)-1)
	n.right = l
	update(n)
	return n, r
}

// merge concatenates two treaps; every element of a precedes every
// element of b. Priorities form a min-heap.
func merge(a, b *node) *node {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if a.priority < b.priority {
		push(a)
		a.right = merge(a.right, b)
		update(a)
		return a
	}
	push(b)
	b.left = merge(a, b.left)
	update(b)
	return b
}

// Insert inserts the non-empty values slice so that the first value lands
// at position index. It requires 0 <= index <= Len.
func (l *List) Insert(index int, values []string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return ErrClosed
	}
	if len(values) == 0 {
		return ErrEmptyValues
	}
	if index < 0 || index > size(l.root) {
		return ErrIndexOutOfRange
	}
	left, right := split(l.root, index)
	for _, v := range values {
		nd := &node{value: v, priority: mix(l.seq), size: 1}
		l.seq++
		left = merge(left, nd)
	}
	l.root = merge(left, right)
	return nil
}

// Delete removes the half-open range [left, right). It requires
// 0 <= left < right <= Len.
func (l *List) Delete(left, right int) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return ErrClosed
	}
	if left < 0 || left >= right || right > size(l.root) {
		return ErrInvalidRange
	}
	a, bc := split(l.root, left)
	_, c := split(bc, right-left) // the dropped middle subtree is garbage collected
	l.root = merge(a, c)
	return nil
}

// Reverse reverses the half-open range [left, right) in place by toggling
// a lazy flag on the root of the affected subtree. It requires
// 0 <= left < right <= Len.
func (l *List) Reverse(left, right int) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return ErrClosed
	}
	if left < 0 || left >= right || right > size(l.root) {
		return ErrInvalidRange
	}
	a, bc := split(l.root, left)
	b, c := split(bc, right-left)
	if b != nil {
		b.rev = !b.rev
	}
	l.root = merge(merge(a, b), c)
	return nil
}

// Get returns the element at index. It requires 0 <= index < Len.
// The walk respects pending lazy flags without mutating the tree, so
// concurrent readers are never blocked by structural changes.
func (l *List) Get(index int) (string, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if index < 0 || index >= size(l.root) {
		return "", ErrIndexOutOfRange
	}
	n := l.root
	k := index
	rev := false
	for n != nil {
		e := rev != n.rev
		first, second := n.left, n.right
		if e {
			first, second = n.right, n.left
		}
		fs := size(first)
		switch {
		case k < fs:
			n = first
		case k == fs:
			return n.value, nil
		default:
			k -= fs + 1
			n = second
		}
		rev = e
	}
	return "", ErrIndexOutOfRange // unreachable after the bounds check
}

// Snapshot returns an independent copy of the current sequence in O(n).
func (l *List) Snapshot() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]string, 0, size(l.root))
	var walk func(n *node, rev bool)
	walk = func(n *node, rev bool) {
		if n == nil {
			return
		}
		e := rev != n.rev
		first, second := n.left, n.right
		if e {
			first, second = n.right, n.left
		}
		walk(first, e)
		out = append(out, n.value)
		walk(second, e)
	}
	walk(l.root, false)
	return out
}

// Len returns the number of elements.
func (l *List) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return size(l.root)
}

// Close marks the list closed. It is idempotent. Insert, Delete and
// Reverse are rejected afterwards, while Get, Snapshot and Len keep
// working.
func (l *List) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.closed = true
}
