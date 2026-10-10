package record

import "fmt"

const Compiler = "3aaaeea1abd20f7e52a7f86d8a1550882071c267"
const SDKBase = "0ac7eae98ebfde1a1a4ead0bd4e855ac54065cbc"

// Set after the source-only corpus has been written, before original execution.
const CorpusSHA = "5a0b422c4f74a5e627252c963d5d2437904b6cd9c9cadb3075bf5a2bcaef9d8a"

type Spec struct {
	ID, Family, Form, Split                     string
	K                                           int64
	Reverse, OutputGoal, ConditionGoal, Wording int
}

func Specs() []Spec {
	var all []Spec
	add := func(family, form, split string, k int64, wording int) {
		for reverse := range 2 {
			for output := range 2 {
				for condition := range 2 {
					id := fmt.Sprintf("cc-%s-%s-k%d-w%d-%s-r%d-g%d-c%d", split, family, k, wording, form, reverse, output, condition)
					all = append(all, Spec{id, family, form, split, k, reverse, output, condition, wording})
				}
			}
		}
	}
	for _, family := range []string{"bound", "distance"} {
		for _, k := range []int64{41, -23} {
			for wording := range 2 {
				add(family, "direct", "train", k, wording)
			}
		}
	}
	for _, family := range []string{"bound", "distance", "affine"} {
		for _, k := range []int64{67, -43} {
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
	for _, k := range []int64{71, -47} {
		for _, wording := range []int{2, 3} {
			for _, form := range []string{"direct", "assignment"} {
				add("distance", form, "sparse", k, wording)
			}
		}
	}
	return all
}
