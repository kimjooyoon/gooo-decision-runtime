package record

import (
	"fmt"

	previous "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-contract-learning-20261011/record"
)

const Compiler = "17153175844967a0153e0424cf2668353600164e"
const SDKBase = "6835253d45954ae89cf47cf255b8b23d8d99852a"

// Bound to the source-only freeze before the producer is published or run.
const CorpusSHA = "9b9beb81a12d81ed5d9c7c0fbe9ab2f29b3710391d534ba0e8059af231f1fcc7"

type Spec = previous.Spec
type Frozen = previous.Frozen

// The known templates and Korean/English descriptions are reused explicitly.
// Only constants and document identities are new; this is not a new task family.
func Specs() []Spec {
	var all []Spec
	add := func(family, form, split string, k int64, wording int) {
		for reverse := range 2 {
			for output := range 2 {
				for condition := range 2 {
					id := fmt.Sprintf("cr-%s-%s-k%d-w%d-%s-r%d-g%d-c%d", split, family, k, wording, form, reverse, output, condition)
					all = append(all, Spec{ID: id, Family: family, Form: form, Split: split, K: k,
						Reverse: reverse, OutputGoal: output, ConditionGoal: condition, Wording: wording})
				}
			}
		}
	}
	for _, family := range []string{"bound", "distance"} {
		for _, k := range []int64{113, -71} {
			for wording := range 2 {
				add(family, "direct", "train", k, wording)
			}
		}
	}
	for _, family := range []string{"bound", "distance", "affine"} {
		for _, k := range []int64{167, -97} {
			for _, wording := range []int{2, 3} {
				for _, form := range []string{"direct", "assignment"} {
					split := "in_family"
					if family == "affine" {
						split = "unseen_family"
					}
					add(family, form, split, k, wording)
				}
			}
		}
	}
	return all
}

func Freeze(s Spec) Frozen {
	return Frozen{Spec: s, Gooo: previous.Gooo(s), Probes: previous.Probes(s), ProbeConditions: previous.ProbeConditions(s)}
}
