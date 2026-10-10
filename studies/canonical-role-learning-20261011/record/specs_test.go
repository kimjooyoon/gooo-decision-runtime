package record

import "testing"

func TestRoleSpecsScope(t *testing.T) {
	counts := map[string]int{}
	seen := map[string]bool{}
	for _, s := range Specs() {
		if seen[s.ID] {
			t.Fatal("duplicate identity", s.ID)
		}
		seen[s.ID] = true
		counts[s.Split]++
		if s.Split == "train" {
			if s.Form != "direct" || s.Wording > 1 || s.Family == "affine" || s.K != 113 && s.K != -71 {
				t.Fatal("training scope", s)
			}
		} else if s.Wording < 2 || s.K != 167 && s.K != -97 {
			t.Fatal("evaluation scope", s)
		}
	}
	if len(seen) != 256 || counts["train"] != 64 || counts["in_family"] != 128 || counts["unseen_family"] != 64 {
		t.Fatal(counts)
	}
}

func TestProjectRoleRetainsOnlyNamedDecision(t *testing.T) {
	for mask := range 4 {
		for role := range 2 {
			got, err := ProjectRole(1<<mask, role)
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for candidate := range 4 {
				included := got>>candidate&1 == 1
				if included != (candidate>>role&1 == mask>>role&1) {
					t.Fatal(mask, role, candidate, got)
				}
				if included {
					count++
				}
			}
			if count != 2 {
				t.Fatal("role must leave the other choice free", got)
			}
		}
	}
	for _, x := range []struct {
		mask uint64
		role int
	}{{0, 0}, {3, 0}, {16, 1}, {1, -1}, {1, 2}} {
		if _, err := ProjectRole(x.mask, x.role); err == nil {
			t.Fatal("accepted invalid role target", x)
		}
	}
}
