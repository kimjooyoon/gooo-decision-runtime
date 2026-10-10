package record

import "fmt"

// Eight training sources and seventy-two evaluation sources, all fresh instances.
// Constants, forms and wording are fixed before any fit.
func Specs() []Spec {
	var all []Spec
	add := func(k int64, form, split string, wording int) {
		for reverse := range 2 {
			for output := range 2 {
				pair := fmt.Sprintf("interaction-distance-k%d-r%d-w%d-%s-g%d", k, reverse, wording, form, output)
				for condition := range 2 {
					all = append(all, Spec{ID: fmt.Sprintf("%s-c%d", pair, condition), Pair: pair,
						Family: "distance", Form: form, Split: split, K: k, Reverse: reverse,
						OutputGoal: output, ConditionGoal: condition, Wording: wording})
				}
			}
		}
	}
	add(41, "direct", "train", 0)
	for wording := range 2 {
		for _, form := range []string{"direct", "alias", "assignment", "rebinding"} {
			add(67, form, fmt.Sprintf("heldout_w%d_%s", wording, form), wording)
		}
	}
	add(67, "rare", "unsupported", 1)
	return all
}
