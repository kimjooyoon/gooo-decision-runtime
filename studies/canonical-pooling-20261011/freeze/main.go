// This command writes source and oracle values only. It imports no compiler
// adapter, model constructor, fit, prediction, or candidate evaluator.
package main

import (
	"compress/gzip"
	"fmt"
	"os"

	r "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-pooling-20261011/record"
)

func main() {
	if len(os.Args) != 2 {
		panic("usage: freeze NEW_CORPUS_GZIP")
	}
	f, err := os.OpenFile(os.Args[1], os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		panic(err)
	}
	z := gzip.NewWriter(f)
	for _, spec := range r.Specs() {
		if _, err := z.Write(r.Encode(r.Freeze(spec))); err != nil {
			panic(err)
		}
	}
	if err := z.Close(); err != nil {
		panic(err)
	}
	if err := f.Close(); err != nil {
		panic(err)
	}
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	fmt.Println(r.Hash(raw))
}
