package record

import "fmt"

// Eight training sources and forty separately authored evaluation sources.
// Constants, forms and wording are fixed before any fit.
func Specs() []Spec {
	var all []Spec
	add := func(k int64, form, split string, wording int) {
		for reverse := range 2 {
			for output := range 2 {
				pair := fmt.Sprintf("ordered-distance-k%d-r%d-%s-g%d", k, reverse, form, output)
				for condition := range 2 {
					all = append(all, Spec{ID: fmt.Sprintf("%s-c%d", pair, condition), Pair: pair,
						Family: "distance", Form: form, Split: split, K: k, Reverse: reverse,
						OutputGoal: output, ConditionGoal: condition, Wording: wording})
				}
			}
		}
	}
	add(37, "direct", "train", 0)
	for _, form := range []string{"direct", "alias", "assignment", "rebinding"} {
		add(53, form, "heldout_"+form, 1)
	}
	add(53, "rare", "unsupported", 1)
	return all
}
