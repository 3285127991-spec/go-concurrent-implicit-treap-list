# go-concurrent-implicit-treap-list

A concurrency-safe list of strings backed by an **implicit treap**, written in
pure Go with no third-party dependencies. It provides a library, a small demo
CLI, and automated tests.

## Features

- `Insert(index, values)` — insert a non-empty slice before `index`
  (`0 <= index <= Len`), O(log n) expected.
- `Delete(left, right)` — remove the half-open interval `[left, right)`
  (`0 <= left < right <= Len`), O(log n) expected.
- `Reverse(left, right)` — reverse `[left, right)` via a **lazy reverse flag**
  on the split-out subtree, O(log n) expected; elements are never swapped
  one by one.
- `Get(index)` — point read, O(log n) expected.
- `Snapshot()` — independent in-order copy of the sequence, O(n).
- `Len()` — element count, O(1).
- `Close()` — idempotent; afterwards `Insert`/`Delete`/`Reverse` fail with
  `ErrClosed` while `Get`/`Snapshot`/`Len` keep working.

All operations are safe for concurrent use by multiple goroutines
(`sync.RWMutex`): writers are serialized, readers run in parallel, and
elements are never lost, duplicated, or reordered unexpectedly.

## How it works

- Each treap node stores its **subtree size**; the in-order position of a node
  is its implicit index. `split`/`merge` by position implement indexed
  insertion and interval deletion, and `size` is recomputed (`pull`) on every
  structural change and after every lazy push.
- `Reverse` splits out the target interval and toggles a `rev` flag on its
  root in O(1). The flag is pushed down lazily (swapping children) during
  later `split`/`merge`/`Get`/`Snapshot` traversals.
- Node priorities come from a strictly increasing internal sequence `Seq`
  passed through a fixed deterministic mixing function
  (`mix64`, splitmix64-style), so the treap shape is fully reproducible for a
  given operation history — no randomness involved.

## Layout

- `list.go` — the `implicittreaplist` library (zero value is ready to use).
- `cmd/itl/main.go` — CLI demo: middle insertion, interval reverse, delete,
  and reads after `Close`.
- `list_test.go` — 8 tests: head/tail/middle insert, delete, reverse,
  consecutive lazy ops, range errors, snapshot independence, close semantics,
  and basic concurrency.

## Usage

```go
var l implicittreaplist.List
_ = l.Insert(0, []string{"a", "b", "c", "d"})
_ = l.Insert(2, []string{"x", "y"}) // a b x y c d
_ = l.Reverse(1, 5)                 // a y x b c d
_ = l.Delete(2, 4)                  // a y c d
v, _ := l.Get(1)                    // "y"
snap := l.Snapshot()                // [a y c d], independent copy
l.Close()                           // mutations now return ErrClosed
```

## Run

```sh
go run ./cmd/itl        # demo CLI
go test -race ./...     # all tests with the race detector
```

