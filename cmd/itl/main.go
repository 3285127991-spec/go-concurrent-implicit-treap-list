// Command itl demonstrates the implicittreaplist library: middle insertion,
// interval reverse, deletion, and reads after Close.
package main

import (
	"errors"
	"fmt"
	"log"

	"implicittreaplist"
)

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func printList(label string, l *implicittreaplist.List) {
	fmt.Printf("%-34s len=%d %q\n", label, l.Len(), l.Snapshot())
}

func main() {
	var l implicittreaplist.List

	must(l.Insert(0, []string{"a", "b", "c", "d"}))
	printList("insert a b c d at 0:", &l)

	must(l.Insert(2, []string{"x", "y"}))
	printList("insert x y before index 2:", &l)

	must(l.Reverse(1, 5))
	printList("reverse [1,5):", &l)

	must(l.Delete(2, 4))
	printList("delete [2,4):", &l)

	v, err := l.Get(1)
	must(err)
	fmt.Printf("get index 1: %q\n", v)

	l.Close()
	l.Close() // idempotent
	fmt.Println("closed (twice, idempotent)")

	if err := l.Insert(0, []string{"z"}); errors.Is(err, implicittreaplist.ErrClosed) {
		fmt.Println("insert after close: rejected with ErrClosed")
	}
	if err := l.Reverse(0, 1); errors.Is(err, implicittreaplist.ErrClosed) {
		fmt.Println("reverse after close: rejected with ErrClosed")
	}
	if err := l.Delete(0, 1); errors.Is(err, implicittreaplist.ErrClosed) {
		fmt.Println("delete after close: rejected with ErrClosed")
	}

	printList("reads still work after close:", &l)
	v, err = l.Get(0)
	must(err)
	fmt.Printf("get index 0 after close: %q\n", v)
}
