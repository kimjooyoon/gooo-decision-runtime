package record

import "fmt"

func Specs() []Spec {
	var all []Spec
	add := func(family, form, split string, k int64, reverse, wording, order int) {
		for output := range 2 {
			pair := fmt.Sprintf("req-%s-k%d-r%d-w%d-o%d-%s-g%d", family, k, reverse, wording, order, form, output)
			for condition := range 2 {
				all = append(all, Spec{ID: fmt.Sprintf("%s-c%d", pair, condition), Pair: pair,
					Family: family, Form: form, Split: split, K: k, Reverse: reverse,
					OutputGoal: output, ConditionGoal: condition, Wording: wording, Order: order})
			}
		}
	}
	for _, family := range []string{"bound", "distance"} {
		for _, k := range []int64{-3, 6, -11, 18} {
			split := "train"
			if k == -11 || k == 18 {
				split = "constant"
			}
			for reverse := range 2 {
				add(family, "direct", split, k, reverse, 0, 0)
			}
		}
		for _, form := range []string{"alias", "assignment", "rebinding"} {
			for reverse := range 2 {
				add(family, form, "representation", 6, reverse, 0, 0)
			}
		}
		for _, wording := range []int{1, 2} {
			for reverse := range 2 {
				add(family, "direct", "wording", -3, reverse, wording, 0)
			}
		}
		add(family, "direct", "choice_order", 6, 0, 0, 1)
	}
	for reverse := range 2 {
		add("negative_bound", "direct", "unseen_family", 8, reverse, 0, 0)
	}
	add("bound", "rare", "rare_tail", 18, 0, 0, 0)
	add("bound", "empty", "empty_conditions", 18, 0, 0, 0)
	add("bound", "contradiction", "contradiction", 18, 0, 0, 0)
	return all
}
