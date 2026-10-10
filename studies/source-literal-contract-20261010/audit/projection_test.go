package main

import (
	"testing"

	r "github.com/kimjooyoon/gooo-decision-runtime/studies/contract-goals-20261010/record"
	q "github.com/kimjooyoon/gooo-decision-runtime/studies/source-literal-contract-20261010/record"
)

func TestProjectionAndConsumedTrainingMutations(t *testing.T) {
	sources := lines[r.Source](load("../../contract-goals-20261010/result", "sources.jsonl.gz"))
	training := decode[[]r.Training](load("../result", "training.json.gz"))
	projections := decode[[]q.Projection](load("../result", "projections.json.gz"))
	consumed := decode[[]r.Training](load("../result", "literal-training.json.gz"))
	for _, mutation := range []string{"literal", "relation", "integer_byte", "last_case", "drop_case", "label", "source_array", "training_case", "training_order"} {
		t.Run(mutation, func(t *testing.T) {
			p := decode[[]q.Projection](r.Encode(projections))
			c := decode[[]r.Training](r.Encode(consumed))
			switch mutation {
			case "literal":
				p[0].Literals[0]++
			case "relation":
				p[0].Cases[0][11] += .125
			case "integer_byte":
				p[0].Cases[0][12] += 1.0 / 255
			case "last_case", "drop_case":
				found := false
				for i := range p {
					if len(p[i].Cases) == 128 {
						found = true
						if mutation == "last_case" {
							p[i].Cases[127][31] += .125
						} else {
							p[i].Cases = p[i].Cases[:127]
						}
						break
					}
				}
				if !found {
					t.Fatal("128-case evidence required")
				}
			case "label":
				c[0].Acceptable ^= 1
			case "source_array":
				c[0].Inputs[0][0]++
			case "training_case":
				c[0].Cases[0][11] += .125
			case "training_order":
				c[0], c[1] = c[1], c[0]
			}
			// Direct comparison exercises the independent reference rather than
			// merely observing that a surrounding digest was not regenerated.
			rejected(t, func() { checkProjections(sources, training, p, c) })
		})
	}
}
