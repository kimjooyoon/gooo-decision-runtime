// Freeze source text only; never lowers, executes, trains or predicts.
package main

import (
	"encoding/json"
	"fmt"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/ordered-requirement-learning-20261010/record"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		panic("usage: freeze FRESH_JSONL")
	}
	f, e := os.OpenFile(os.Args[1], os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		panic(e)
	}
	for _, s := range r.Specs() {
		if e := json.NewEncoder(f).Encode(r.FrozenSource{Spec: s, Gooo: r.Gooo(s)}); e != nil {
			panic(e)
		}
	}
	if e := f.Close(); e != nil {
		panic(e)
	}
	fmt.Fprintln(os.Stderr, "48 source documents frozen; zero fits, predictions or candidate executions")
}
