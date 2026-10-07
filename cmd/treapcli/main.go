// Command treapcli demonstrates the implicittreaplist package: middle
// insertion, range reversal, range deletion, and reads after Close.
package main

import (
	"fmt"

	"implicittreaplist"
)

func main() {
	l := implicittreaplist.New()

	show := func(step string) {
		fmt.Printf("%-24s -> len=%d %v\n", step, l.Len(), l.Snapshot())
	}
	must := func(err error) {
		if err != nil {
			panic(err)
		}
	}

	must(l.Insert(0, []string{"apple", "banana", "cherry", "date"}))
	show("insert 4 fruits")

	must(l.Insert(2, []string{"fig", "grape"}))
	show("insert at 2 (middle)")

	must(l.Reverse(1, 5))
	show("reverse [1,5)")

	must(l.Delete(0, 2))
	show("delete [0,2)")

	if v, err := l.Get(1); err == nil {
		fmt.Printf("Get(1)                   -> %q\n", v)
	}

	l.Close()
	l.Close() // Close is idempotent
	fmt.Println("-- closed (twice) --")
	show("snapshot still readable")
	if v, err := l.Get(0); err == nil {
		fmt.Printf("Get(0) still works       -> %q\n", v)
	}
	if err := l.Insert(0, []string{"kiwi"}); err != nil {
		fmt.Println("insert after close       ->", err)
	}
	if err := l.Delete(0, 1); err != nil {
		fmt.Println("delete after close       ->", err)
	}
	if err := l.Reverse(0, 1); err != nil {
		fmt.Println("reverse after close      ->", err)
	}
}
