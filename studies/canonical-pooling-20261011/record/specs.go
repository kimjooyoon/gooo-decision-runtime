package record

import (
	"fmt"

	previous "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-contract-learning-20261011/record"
)

const Compiler = "c96aaee02d35f50b591fd930309550f3d85d2691"
const SDKBase = "d3bab76e16d90244d5a64db9e905e6ea6c25000a"

// Bound to the source-only freeze before the producer is published or run.
const CorpusSHA = "9bde751be7f8c39cb7084868db41a3c372d10923b825e9bf682960a6c51d23c5"

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
					id := fmt.Sprintf("cp-%s-%s-k%d-w%d-%s-r%d-g%d-c%d", split, family, k, wording, form, reverse, output, condition)
					all = append(all, Spec{ID: id, Family: family, Form: form, Split: split, K: k,
						Reverse: reverse, OutputGoal: output, ConditionGoal: condition, Wording: wording})
				}
			}
		}
	}
	for _, family := range []string{"bound", "distance"} {
		for _, k := range []int64{89, -59} {
			for wording := range 2 {
				add(family, "direct", "train", k, wording)
			}
		}
	}
	for _, family := range []string{"bound", "distance", "affine"} {
		for _, k := range []int64{131, -83} {
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
