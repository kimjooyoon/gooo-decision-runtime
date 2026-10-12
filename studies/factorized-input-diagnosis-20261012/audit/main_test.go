package main

import "testing"

func TestConflictingSharedInputsRetainHonestUpperBound(t *testing.T) {
	g := &group{}
	add(g, row{Specification: specification{Split: "train"}, Acceptable: 1})
	add(g, row{Specification: specification{Split: "evaluation"}, Acceptable: 2})
	got := summarize(map[[32]byte]*group{{}: g})
	if got.Rows != 2 || got.ConflictingGroups != 1 || got.ConflictingRows != 2 ||
		got.CrossSplitGroups != 1 || got.SharedChoiceUpperBound != 1 {
		t.Fatal(got)
	}
}

func TestNeutralInputIdentityClearsOnlyProse(t *testing.T) {
	base := row{}
	prose := base
	prose.V3[0][64] = 0.125
	if inputKey(base, "v3-prose-neutral") != inputKey(prose, "v3-prose-neutral") ||
		inputKey(base, "v3-full") == inputKey(prose, "v3-full") {
		t.Fatal("prose policy")
	}
	requirement := base
	requirement.Outputs[0][12] = 0.125
	if inputKey(base, "v3-prose-neutral") == inputKey(requirement, "v3-prose-neutral") {
		t.Fatal("lost requirement")
	}
	ordered := base
	ordered.V3[1][400] = 0.125
	if inputKey(base, "v3-prose-neutral") == inputKey(ordered, "v3-prose-neutral") {
		t.Fatal("lost source order")
	}
}

func TestCommonAcceptedChoiceIsNotAConflict(t *testing.T) {
	g := &group{}
	add(g, row{Acceptable: 3})
	add(g, row{Acceptable: 6})
	got := summarize(map[[32]byte]*group{{}: g})
	if got.ConflictingGroups != 0 || got.SharedChoiceUpperBound != 2 {
		t.Fatal(got)
	}
}

func TestNeutralIdentityRetainsEveryNonProseBoundary(t *testing.T) {
	base := row{}
	key := inputKey(base, "v3-prose-neutral")
	for choice := range 2 {
		for _, cell := range []int{0, 63, 192, 527} {
			changed := base
			changed.V3[choice][cell] = 0.125
			if inputKey(changed, "v3-prose-neutral") == key {
				t.Fatalf("lost source cell %d:%d", choice, cell)
			}
		}
	}
	for index := range 6 {
		changed := base
		changed.Outputs[index][31] = 0.125
		if inputKey(changed, "v3-prose-neutral") == key {
			t.Fatalf("lost output row %d", index)
		}
	}
	for index := range 3 {
		changed := base
		changed.Conditions[index][31] = 0.125
		if inputKey(changed, "v3-prose-neutral") == key {
			t.Fatalf("lost Boolean row %d", index)
		}
	}
}
