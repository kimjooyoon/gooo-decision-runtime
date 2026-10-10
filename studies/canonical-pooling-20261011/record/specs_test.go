package record

import "testing"

func TestPoolingSpecsScope(t *testing.T) {
	counts := map[string]int{}
	seen := map[string]bool{}
	for _, s := range Specs() {
		if seen[s.ID] {
			t.Fatal("duplicate identity", s.ID)
		}
		seen[s.ID] = true
		counts[s.Split]++
		if s.Split == "train" {
			if s.Form != "direct" || s.Wording > 1 || s.Family == "affine" || s.K != 89 && s.K != -59 {
				t.Fatal("training scope", s)
			}
		} else if s.Wording < 2 || s.K != 131 && s.K != -83 {
			t.Fatal("evaluation scope", s)
		}
	}
	if len(seen) != 256 || counts["train"] != 64 || counts["in_family"] != 128 || counts["unseen_family"] != 64 {
		t.Fatal(counts)
	}
}
