package implicittreaplist

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
)

func mustInsert(t *testing.T, l *List, index int, values ...string) {
	t.Helper()
	if err := l.Insert(index, values); err != nil {
		t.Fatalf("Insert(%d, %q) failed: %v", index, values, err)
	}
}

func mustDelete(t *testing.T, l *List, left, right int) {
	t.Helper()
	if err := l.Delete(left, right); err != nil {
		t.Fatalf("Delete(%d, %d) failed: %v", left, right, err)
	}
}

func mustReverse(t *testing.T, l *List, left, right int) {
	t.Helper()
	if err := l.Reverse(left, right); err != nil {
		t.Fatalf("Reverse(%d, %d) failed: %v", left, right, err)
	}
}

func checkSnapshot(t *testing.T, l *List, want []string) {
	t.Helper()
	if got := l.Snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Snapshot() = %q, want %q", got, want)
	}
	if got := l.Len(); got != len(want) {
		t.Fatalf("Len() = %d, want %d", got, len(want))
	}
}

func TestInsertHeadTailMiddle(t *testing.T) {
	var l List
	mustInsert(t, &l, 0, "b", "c")      // tail of empty list
	mustInsert(t, &l, 0, "a")           // head
	mustInsert(t, &l, 3, "d", "e")      // tail
	mustInsert(t, &l, 2, "x", "y", "z") // middle
	checkSnapshot(t, &l, []string{"a", "b", "x", "y", "z", "c", "d", "e"})

	for i, want := range []string{"a", "b", "x", "y", "z", "c", "d", "e"} {
		got, err := l.Get(i)
		if err != nil {
			t.Fatalf("Get(%d) failed: %v", i, err)
		}
		if got != want {
			t.Fatalf("Get(%d) = %q, want %q", i, got, want)
		}
	}
}

func TestDeleteRanges(t *testing.T) {
	var l List
	mustInsert(t, &l, 0, "a", "b", "c", "d", "e", "f")
	mustDelete(t, &l, 0, 2) // head
	checkSnapshot(t, &l, []string{"c", "d", "e", "f"})
	mustDelete(t, &l, 2, 4) // tail
	checkSnapshot(t, &l, []string{"c", "d"})
	mustDelete(t, &l, 1, 2) // middle
	checkSnapshot(t, &l, []string{"c"})
	mustDelete(t, &l, 0, 1) // everything left
	checkSnapshot(t, &l, []string{})
}

func TestReverseRanges(t *testing.T) {
	var l List
	mustInsert(t, &l, 0, "a", "b", "c", "d", "e")
	mustReverse(t, &l, 1, 4) // middle
	checkSnapshot(t, &l, []string{"a", "d", "c", "b", "e"})
	mustReverse(t, &l, 0, 5) // whole list
	checkSnapshot(t, &l, []string{"e", "b", "c", "d", "a"})
	mustReverse(t, &l, 0, 2) // head
	checkSnapshot(t, &l, []string{"b", "e", "c", "d", "a"})
	mustReverse(t, &l, 3, 5) // tail
	checkSnapshot(t, &l, []string{"b", "e", "c", "a", "d"})
}

func TestConsecutiveLazyOps(t *testing.T) {
	var l List
	mustInsert(t, &l, 0, "a", "b", "c", "d", "e", "f")
	mustReverse(t, &l, 0, 3)  // c b a | d e f
	mustReverse(t, &l, 3, 6)  // c b a | f e d
	mustReverse(t, &l, 1, 5)  // c e f a | b d
	mustInsert(t, &l, 2, "x") // c e x f a b d
	mustDelete(t, &l, 0, 2)   // x f a b d
	mustReverse(t, &l, 0, 5)  // d b a f x
	checkSnapshot(t, &l, []string{"d", "b", "a", "f", "x"})

	// Get must resolve any lazy flags still pending on the path.
	for i, want := range []string{"d", "b", "a", "f", "x"} {
		got, err := l.Get(i)
		if err != nil {
			t.Fatalf("Get(%d) failed: %v", i, err)
		}
		if got != want {
			t.Fatalf("Get(%d) = %q, want %q", i, got, want)
		}
	}
}

func TestRangeErrors(t *testing.T) {
	var l List
	mustInsert(t, &l, 0, "a", "b", "c")

	cases := []struct {
		name string
		op   func() error
	}{
		{"insert negative index", func() error { return l.Insert(-1, []string{"x"}) }},
		{"insert past end", func() error { return l.Insert(4, []string{"x"}) }},
		{"insert empty values", func() error { return l.Insert(0, nil) }},
		{"delete negative left", func() error { return l.Delete(-1, 2) }},
		{"delete empty range", func() error { return l.Delete(1, 1) }},
		{"delete inverted range", func() error { return l.Delete(2, 1) }},
		{"delete past end", func() error { return l.Delete(0, 4) }},
		{"reverse negative left", func() error { return l.Reverse(-1, 2) }},
		{"reverse empty range", func() error { return l.Reverse(2, 2) }},
		{"reverse past end", func() error { return l.Reverse(1, 4) }},
	}
	for _, tc := range cases {
		if err := tc.op(); err == nil {
			t.Fatalf("%s: expected error, got nil", tc.name)
		}
	}
	for _, i := range []int{-1, 3} {
		if _, err := l.Get(i); err == nil {
			t.Fatalf("Get(%d): expected error, got nil", i)
		}
	}

	// Failed operations must not mutate the list.
	checkSnapshot(t, &l, []string{"a", "b", "c"})
}

func TestSnapshotIndependent(t *testing.T) {
	var l List
	mustInsert(t, &l, 0, "a", "b", "c")

	snap := l.Snapshot()
	snap[0] = "mutated" // mutating the copy must not affect the list
	if got, _ := l.Get(0); got != "a" {
		t.Fatalf("Get(0) = %q after mutating snapshot, want %q", got, "a")
	}

	mustReverse(t, &l, 0, 3)
	mustDelete(t, &l, 0, 1)
	mustInsert(t, &l, 2, "d")
	checkSnapshot(t, &l, []string{"b", "a", "d"})

	// The earlier snapshot keeps its own view.
	if want := []string{"mutated", "b", "c"}; !reflect.DeepEqual(snap, want) {
		t.Fatalf("snapshot changed after list mutations: %q, want %q", snap, want)
	}
}

func TestCloseSemantics(t *testing.T) {
	var l List
	mustInsert(t, &l, 0, "a", "b")
	l.Close()
	l.Close() // idempotent, must not panic or deadlock

	mutators := []struct {
		name string
		op   func() error
	}{
		{"Insert", func() error { return l.Insert(0, []string{"x"}) }},
		{"Delete", func() error { return l.Delete(0, 1) }},
		{"Reverse", func() error { return l.Reverse(0, 2) }},
	}
	for _, m := range mutators {
		if err := m.op(); !errors.Is(err, ErrClosed) {
			t.Fatalf("%s after Close: got %v, want ErrClosed", m.name, err)
		}
	}

	// Reads still work.
	checkSnapshot(t, &l, []string{"a", "b"})
	if got, err := l.Get(1); err != nil || got != "b" {
		t.Fatalf("Get(1) after Close = %q, %v; want %q, nil", got, err, "b")
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	var l List
	const writers = 4
	const perWriter = 25

	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				if err := l.Insert(l.Len(), []string{fmt.Sprintf("w%d-%d", id, i)}); err != nil {
					t.Errorf("concurrent Insert failed: %v", err)
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
				_ = l.Snapshot()
				_ = l.Len()
				if n := l.Len(); n > 0 {
					if _, err := l.Get(n - 1); err != nil {
						t.Errorf("concurrent Get failed: %v", err)
						return
					}
				}
			}
		}()
	}
	wg.Wait()

	if got, want := l.Len(), writers*perWriter; got != want {
		t.Fatalf("Len() = %d, want %d (lost or duplicated elements)", got, want)
	}
	seen := make(map[string]int)
	for _, v := range l.Snapshot() {
		seen[v]++
	}
	for w := 0; w < writers; w++ {
		for i := 0; i < perWriter; i++ {
			v := fmt.Sprintf("w%d-%d", w, i)
			if seen[v] != 1 {
				t.Fatalf("element %q appears %d times, want exactly 1", v, seen[v])
			}
		}
	}
}
