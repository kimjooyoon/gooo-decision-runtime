// Freeze only source documents and independently written probe expectations.
// No compiler, model, labels, body evaluation or training is called here.
package main

import (
	"encoding/json"
	"fmt"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-contract-learning-20261011/record"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		panic("usage: freeze FRESH_JSONL")
	}
	f, err := os.OpenFile(os.Args[1], os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	for _, s := range r.Specs() {
		if err := json.NewEncoder(f).Encode(r.Frozen{Spec: s, Gooo: r.Gooo(s), Probes: r.Probes(s), ProbeConditions: r.ProbeConditions(s)}); err != nil {
			panic(err)
		}
	}
	if err := f.Close(); err != nil {
		panic(err)
	}
	fmt.Fprintln(os.Stderr, "320 fresh source documents and probes frozen; zero fits, predictions or candidate executions")
}
