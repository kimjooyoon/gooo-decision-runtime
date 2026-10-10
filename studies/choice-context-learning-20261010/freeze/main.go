// Freeze source text and split identities before either model is fitted.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	r "github.com/kimjooyoon/gooo-decision-runtime/studies/choice-context-learning-20261010/record"
)

func main() {
	if len(os.Args) != 2 {
		panic("usage: freeze NEW_CORPUS_JSONL")
	}
	f, err := os.OpenFile(os.Args[1], os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	counts := map[string]int{}
	for _, spec := range r.Specs() {
		if err := json.NewEncoder(f).Encode(r.FrozenSource{Spec: spec, Gooo: r.Gooo(spec)}); err != nil {
			panic(err)
		}
		counts[spec.Split]++
	}
	if err := f.Close(); err != nil {
		panic(err)
	}
	fmt.Println(counts)
}
