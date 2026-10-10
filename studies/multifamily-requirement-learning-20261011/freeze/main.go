// Freeze source text only; never lowers, executes, trains or predicts.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	r "github.com/kimjooyoon/gooo-decision-runtime/studies/multifamily-requirement-learning-20261011/record"
)

func main() {
	if len(os.Args) != 2 {
		panic("usage: freeze FRESH_JSONL")
	}
	f, err := os.OpenFile(os.Args[1], os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	for _, spec := range r.Specs() {
		if err := json.NewEncoder(f).Encode(r.FrozenSource{Spec: spec, Gooo: r.Gooo(spec)}); err != nil {
			panic(err)
		}
	}
	if err := f.Close(); err != nil {
		panic(err)
	}
	fmt.Fprintln(os.Stderr, "256 source documents frozen; zero fits, predictions or candidate executions")
}
