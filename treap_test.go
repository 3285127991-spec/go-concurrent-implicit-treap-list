package implicittreaplist

import (
	"errors"
	"fmt"
	"math/rand"
	"slices"
	"sync"
	"testing"
)

func requireSnapshot(t *testing.T, l *List, want []string) {
	t.Helper()
	if got := l.Snapshot(); !slices.Equal(got, want) {
		t.Fatalf("snapshot = %v, want %v", got, want)
	}
	if l.Len() != len(want) {
		t.Fatalf("Len = %d, want %d", l.Len(), len(want))
	}
	for i, v := range want {
		got, err := l.Get(i)
		if err != nil || got != v {
			t.Fatalf("Get(%d) = %q, %v, want %q", i, got, err, v)
		}
	}
}

// checkInvariants verifies subtree sizes and the min-heap priority order.
func checkInvariants(t *testing.T, n *node, parent uint64) int {
	t.Helper()
	if n == nil {
		return 0
	}
	if n.priority < parent {
		t.Fatalf("heap violation at %q: priority %d < parent %d", n.value, n.priority, parent)
	}
	ls := checkInvariants(t, n.left, n.priority)
	rs := checkInvariants(t, n.right, n.priority)
	if n.size != ls+rs+1 {
		t.Fatalf("size violation at %q: size %d != %d + %d + 1", n.value, n.size, ls, rs)
	}
	return n.size
}

func TestInsertPositions(t *testing.T) {
	l := New()
	defer l.Close()

	if err := l.Insert(0, []string{"b"}); err != nil {
		t.Fatal(err)
	}
	if err := l.Insert(0, []string{"a"}); err != nil { // head
		t.Fatal(err)
	}
	if err := l.Insert(2, []string{"d", "e"}); err != nil { // tail
		t.Fatal(err)
	}
	if err := l.Insert(2, []string{"c"}); err != nil { // middle
		t.Fatal(err)
	}
	requireSnapshot(t, l, []string{"a", "b", "c", "d", "e"})
	checkInvariants(t, l.root, 0)
}

func TestDeleteRange(t *testing.T) {
	l := New()
	defer l.Close()

	if err := l.Insert(0, []string{"a", "b", "c", "d", "e", "f", "g"}); err != nil {
		t.Fatal(err)
	}
	if err := l.Delete(2, 5); err != nil { // middle
		t.Fatal(err)
	}
	requireSnapshot(t, l, []string{"a", "b", "f", "g"})
	if err := l.Delete(0, 1); err != nil { // head
		t.Fatal(err)
	}
	requireSnapshot(t, l, []string{"b", "f", "g"})
	if err := l.Delete(2, 3); err != nil { // tail
		t.Fatal(err)
	}
	requireSnapshot(t, l, []string{"b", "f"})
	if err := l.Delete(0, 2); err != nil { // everything
		t.Fatal(err)
	}
	requireSnapshot(t, l, nil)
	checkInvariants(t, l.root, 0)
}

func TestReverseRange(t *testing.T) {
	l := New()
	defer l.Close()

	if err := l.Insert(0, []string{"a", "b", "c", "d", "e"}); err != nil {
		t.Fatal(err)
	}
	if err := l.Reverse(1, 4); err != nil {
		t.Fatal(err)
	}
	requireSnapshot(t, l, []string{"a", "d", "c", "b", "e"})
	if err := l.Reverse(0, 5); err != nil { // whole list
		t.Fatal(err)
	}
	requireSnapshot(t, l, []string{"e", "b", "c", "d", "a"})
	if err := l.Reverse(2, 3); err != nil { // single element is a no-op
		t.Fatal(err)
	}
	requireSnapshot(t, l, []string{"e", "b", "c", "d", "a"})
	if err := l.Reverse(0, 2); err != nil { // head
		t.Fatal(err)
	}
	requireSnapshot(t, l, []string{"b", "e", "c", "d", "a"})
	if err := l.Reverse(3, 5); err != nil { // tail
		t.Fatal(err)
	}
	requireSnapshot(t, l, []string{"b", "e", "c", "a", "d"})
	checkInvariants(t, l.root, 0)
}

// TestMixedLazyOperations drives long chains of interleaved inserts,
// deletes and reverses (many overlapping lazy flags) and compares every
// step against a naive slice model.
func TestMixedLazyOperations(t *testing.T) {
	l := New()
	defer l.Close()
	var model []string
	rng := rand.New(rand.NewSource(1))

	for step := 0; step < 200; step++ {
		switch rng.Intn(3) {
		case 0:
			idx := rng.Intn(len(model) + 1)
			vals := make([]string, 1+rng.Intn(3))
			for i := range vals {
				vals[i] = fmt.Sprintf("s%03d-%d", step, i)
			}
			if err := l.Insert(idx, vals); err != nil {
				t.Fatalf("step %d insert: %v", step, err)
			}
			model = slices.Insert(model, idx, vals...)
		case 1:
			if len(model) == 0 {
				continue
			}
			left := rng.Intn(len(model))
			right := left + 1 + rng.Intn(len(model)-left)
			if err := l.Delete(left, right); err != nil {
				t.Fatalf("step %d delete: %v", step, err)
			}
			model = slices.Delete(model, left, right)
		case 2:
			if len(model) == 0 {
				continue
			}
			left := rng.Intn(len(model))
			right := left + 1 + rng.Intn(len(model)-left)
			if err := l.Reverse(left, right); err != nil {
				t.Fatalf("step %d reverse: %v", step, err)
			}
			slices.Reverse(model[left:right])
		}
		if got := l.Snapshot(); !slices.Equal(got, model) {
			t.Fatalf("step %d: snapshot = %v, want %v", step, got, model)
		}
		if l.Len() != len(model) {
			t.Fatalf("step %d: Len = %d, want %d", step, l.Len(), len(model))
		}
		if len(model) > 0 {
			i := rng.Intn(len(model))
			if got, err := l.Get(i); err != nil || got != model[i] {
				t.Fatalf("step %d: Get(%d) = %q, %v, want %q", step, i, got, err, model[i])
			}
		}
	}
	checkInvariants(t, l.root, 0)
}

func TestBoundaryErrors(t *testing.T) {
	l := New()
	defer l.Close()
	if err := l.Insert(0, []string{"a", "b", "c"}); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		err  error
		want error
	}{
		{"insert negative index", l.Insert(-1, []string{"x"}), ErrIndexOutOfRange},
		{"insert past end", l.Insert(4, []string{"x"}), ErrIndexOutOfRange},
		{"insert nil values", l.Insert(0, nil), ErrEmptyValues},
		{"insert empty values", l.Insert(0, []string{}), ErrEmptyValues},
		{"delete negative left", l.Delete(-1, 2), ErrInvalidRange},
		{"delete empty range", l.Delete(1, 1), ErrInvalidRange},
		{"delete inverted range", l.Delete(2, 1), ErrInvalidRange},
		{"delete past end", l.Delete(2, 4), ErrInvalidRange},
		{"reverse negative left", l.Reverse(-1, 2), ErrInvalidRange},
		{"reverse empty range", l.Reverse(0, 0), ErrInvalidRange},
		{"reverse past end", l.Reverse(1, 5), ErrInvalidRange},
	} {
		if !errors.Is(tc.err, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, tc.err, tc.want)
		}
	}
	if _, err := l.Get(-1); !errors.Is(err, ErrIndexOutOfRange) {
		t.Errorf("Get(-1): got %v, want %v", err, ErrIndexOutOfRange)
	}
	if _, err := l.Get(3); !errors.Is(err, ErrIndexOutOfRange) {
		t.Errorf("Get(3): got %v, want %v", err, ErrIndexOutOfRange)
	}
	// Failed operations must not modify the list.
	requireSnapshot(t, l, []string{"a", "b", "c"})
}

func TestSnapshotAndClose(t *testing.T) {
	l := New()
	if err := l.Insert(0, []string{"a", "b", "c"}); err != nil {
		t.Fatal(err)
	}

	snap := l.Snapshot()
	snap[0] = "MUTATED" // mutating the copy must not affect the list
	if v, _ := l.Get(0); v != "a" {
		t.Fatalf("snapshot shares state with list: Get(0) = %q", v)
	}
	if err := l.Reverse(0, 3); err != nil { // later mutations must not affect the copy
		t.Fatal(err)
	}
	if !slices.Equal(snap, []string{"MUTATED", "b", "c"}) {
		t.Fatalf("snapshot changed after list mutation: %v", snap)
	}

	l.Close()
	l.Close() // idempotent

	if got := l.Len(); got != 3 {
		t.Errorf("Len after Close = %d, want 3", got)
	}
	if v, err := l.Get(2); err != nil || v != "a" {
		t.Errorf("Get after Close = %q, %v, want %q", v, err, "a")
	}
	if got := l.Snapshot(); !slices.Equal(got, []string{"c", "b", "a"}) {
		t.Errorf("Snapshot after Close = %v", got)
	}
	for name, err := range map[string]error{
		"Insert":  l.Insert(0, []string{"x"}),
		"Delete":  l.Delete(0, 1),
		"Reverse": l.Reverse(0, 1),
	} {
		if !errors.Is(err, ErrClosed) {
			t.Errorf("%s after Close: got %v, want %v", name, err, ErrClosed)
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	l := New()
	defer l.Close()
	const writers = 4
	const rounds = 5
	const batch = 5

	value := func(w, r, i int) string { return fmt.Sprintf("w%d-r%d-%d", w, r, i) }

	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for r := 0; r < rounds; r++ {
				vals := make([]string, batch)
				for i := range vals {
					vals[i] = value(id, r, i)
				}
				if err := l.Insert(l.Len(), vals); err != nil {
					t.Errorf("insert: %v", err)
					return
				}
			}
		}(w)
	}
	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				if n := l.Len(); n > 0 {
					if _, err := l.Get(n - 1); err != nil {
						t.Errorf("Get(%d): %v", n-1, err)
						return
					}
				}
				_ = l.Snapshot()
			}
		}()
	}
	wg.Wait()

	total := writers * rounds * batch
	if got := l.Len(); got != total {
		t.Fatalf("Len = %d, want %d", got, total)
	}
	snap := l.Snapshot()
	seen := make(map[string]int, len(snap))
	for _, v := range snap {
		seen[v]++
	}
	for w := 0; w < writers; w++ {
		for r := 0; r < rounds; r++ {
			for i := 0; i < batch; i++ {
				v := value(w, r, i)
				if seen[v] != 1 {
					t.Fatalf("value %q appears %d times, want exactly 1", v, seen[v])
				}
			}
		}
	}
	// Values inserted by one Insert call land contiguously and no later
	// insert reorders existing elements, so each batch must keep its
	// relative order in the final sequence.
	for w := 0; w < writers; w++ {
		for r := 0; r < rounds; r++ {
			next := 0
			for _, v := range snap {
				if next < batch && v == value(w, r, next) {
					next++
				}
			}
			if next != batch {
				t.Fatalf("batch w%d-r%d: only %d/%d values in order", w, r, next, batch)
			}
		}
	}
	checkInvariants(t, l.root, 0)
}

// TestDeterministicStructure applies the same operation history to two
// lists and requires identical internal tree shapes, proving that the
// Seq-derived priorities make the structure reproducible.
func TestDeterministicStructure(t *testing.T) {
	build := func() *List {
		l := New()
		l.Insert(0, []string{"a", "b", "c"})
		l.Insert(1, []string{"x", "y"})
		l.Reverse(0, 3)
		l.Insert(5, []string{"z"})
		l.Delete(1, 3)
		l.Reverse(0, 2)
		return l
	}
	a, b := build(), build()
	defer a.Close()
	defer b.Close()

	var dump func(n *node) string
	dump = func(n *node) string {
		if n == nil {
			return "#"
		}
		return fmt.Sprintf("(%s p=%d s=%d r=%t %s %s)",
			n.value, n.priority, n.size, n.rev, dump(n.left), dump(n.right))
	}
	if da, db := dump(a.root), dump(b.root); da != db {
		t.Fatalf("structure not reproducible:\n%s\n%s", da, db)
	}
	if mix(7) != mix(7) || mix(1) == mix(2) {
		t.Fatal("mix must be deterministic and non-constant")
	}
	requireSnapshot(t, a, b.Snapshot())
	checkInvariants(t, a.root, 0)
}
