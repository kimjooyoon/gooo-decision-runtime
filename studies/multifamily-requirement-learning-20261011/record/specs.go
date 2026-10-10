package record

import "fmt"

// Training uses two families, two constants and two languages. All evaluation
// constants and wordings are excluded from fitting; one family is also excluded.
func Specs() []Spec {
	var result []Spec
	add := func(family, form, split string, k int64, wording int) {
		for reverse := range 2 {
			for output := range 2 {
				pair := fmt.Sprintf("mf-%s-k%d-r%d-w%d-%s-g%d", family, k, reverse, wording, form, output)
				for condition := range 2 {
					result = append(result, Spec{ID: fmt.Sprintf("%s-c%d", pair, condition), Pair: pair,
						Family: family, Form: form, Split: split, K: k, Reverse: reverse,
						OutputGoal: output, ConditionGoal: condition, Wording: wording})
				}
			}
		}
	}
	for _, family := range []string{"bound", "distance"} {
		for _, k := range []int64{29, -17} {
			for wording := range 2 {
				add(family, "direct", "train", k, wording)
			}
		}
	}
	for _, family := range []string{"bound", "distance", "negative_bound"} {
		for _, k := range []int64{53, -37} {
			for _, wording := range []int{2, 3} {
				for _, form := range []string{"direct", "assignment"} {
					split := fmt.Sprintf("heldout_w%d_%s", wording, form)
					if family == "negative_bound" {
						split = fmt.Sprintf("unseen_family_w%d_%s", wording, form)
					}
					add(family, form, split, k, wording)
				}
			}
		}
	}
	return result
}
