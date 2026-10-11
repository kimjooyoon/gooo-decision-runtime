package record

import (
	"fmt"

	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
	previous "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-contract-learning-20261011/record"
)

// The compiler snapshot has the exact released ee24a413 source tree. Original
// execution also requires its own CI to have passed before public source freeze.
const Compiler = "67ee67c7cf0cdeb22f4caead87144dfb19f29d54"
const SDKBase = "eb013ff65478fb883f13f0c8feb88834793953b8"
const CorpusSHA = "c7b0bdb60dbc34a30bbe9a2effbdc1ee1607ceb2d990d6303a63b6e77b43f9f6"

type Spec struct {
	previous.Spec
	Diagnostic string
}
type Frozen struct {
	Spec            Spec
	Gooo            string
	Probes          []pathplan.TestCase
	ProbeConditions []pathplan.ConditionCase
}

func Specs() []Spec {
	var all []Spec
	add := func(family, form, split string, k int64, wording int) {
		for reverse := range 2 {
			for output := range 2 {
				for condition := range 2 {
					id := fmt.Sprintf("sr-%s-%s-k%d-w%d-%s-r%d-g%d-c%d", split, family, k, wording, form, reverse, output, condition)
					all = append(all, Spec{Spec: previous.Spec{ID: id, Family: family, Form: form, Split: split, K: k,
						Reverse: reverse, OutputGoal: output, ConditionGoal: condition, Wording: wording}})
				}
			}
		}
	}
	for _, family := range []string{"bound", "distance"} {
		for _, k := range []int64{211, -131} {
			for wording := range 2 {
				add(family, "direct", "train", k, wording)
			}
		}
	}
	for _, family := range []string{"bound", "distance", "affine"} {
		for _, k := range []int64{257, -149} {
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
	for _, diagnostic := range []string{"missing_conditions", "unsatisfiable_conditions"} {
		for _, k := range []int64{257, -149} {
			for _, form := range []string{"direct", "assignment"} {
				for output := range 2 {
					for reverse := range 2 {
						id := fmt.Sprintf("sr-%s-k%d-%s-r%d-g%d", diagnostic, k, form, reverse, output)
						all = append(all, Spec{Spec: previous.Spec{ID: id, Family: "bound", Form: form, Split: diagnostic, K: k,
							Reverse: reverse, OutputGoal: output, ConditionGoal: 0, Wording: 2}, Diagnostic: diagnostic})
					}
				}
			}
		}
	}
	return all
}

func Freeze(s Spec) Frozen {
	conditions := previous.Conditions(s.Spec)
	switch s.Diagnostic {
	case "missing_conditions":
		conditions = nil
	case "unsatisfiable_conditions":
		for i := range conditions {
			if conditions[i].Input == s.K {
				conditions[i].Expected = true
			}
		}
	case "":
	default:
		panic("unknown source-relative diagnostic")
	}
	return Frozen{Spec: s, Gooo: previous.GoooWithCases(s.Spec, previous.Cases(s.Spec), conditions),
		Probes: previous.Probes(s.Spec), ProbeConditions: previous.ProbeConditions(s.Spec)}
}
