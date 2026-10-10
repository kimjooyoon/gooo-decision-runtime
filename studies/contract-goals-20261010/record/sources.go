package record

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

type Spec struct {
	ID, Pair, Family, Form, Split string
	K                             int64
	Reverse, Goal, Wording        int
}

func Specs() []Spec {
	var all []Spec
	add := func(family, form, split string, k int64, rev, word int) {
		pair := fmt.Sprintf("%s-k%d-r%d-w%d-%s", family, k, rev, word, form)
		for goal := range 2 {
			all = append(all, Spec{fmt.Sprintf("%s-g%d", pair, goal), pair, family, form, split, k, rev, goal, word})
		}
	}
	for _, family := range []string{"bound", "distance", "offset"} {
		for _, k := range []int64{7, 13, 11, 17} {
			for rev := range 2 {
				for _, form := range []string{"direct", "alias", "assignment"} {
					split := "constant"
					if k == 7 || k == 13 {
						split = "representation"
						if form == "direct" {
							split = "train"
						}
					}
					add(family, form, split, k, rev, 0)
				}
			}
		}
	}
	for _, family := range []string{"bound", "distance", "offset"} {
		for _, k := range []int64{7, 13} {
			for rev := range 2 {
				add(family, "direct", "wording", k, rev, 1)
			}
		}
	}
	for _, k := range []int64{7, 13} {
		for rev := range 2 {
			for _, form := range []string{"direct", "alias", "assignment"} {
				add("tag", form, "unseen_family", k, rev, 0)
			}
		}
	}
	add("bound", "rare", "rare_tail", 7, 0, 0)
	add("bound", "contradiction", "contradiction", 7, 0, 0)
	return all
}

func arms(family, value, limit string) (string, string) {
	switch family {
	case "bound":
		return value, limit
	case "distance":
		return value + " - " + limit, limit + " - " + value
	case "offset":
		return value + " + " + limit, value + " - " + limit
	case "tag":
		return "0 - " + limit, limit
	}
	panic("unknown source family")
}
func Body(s Spec) string {
	x, k := "input", fmt.Sprint(s.K)
	prefix := ""
	if s.Form == "alias" || s.Form == "assignment" {
		x, k = "value", "limit"
		prefix = fmt.Sprintf("let value = input; let limit = %d; ", s.K)
	}
	a, b := arms(s.Family, x, k)
	if s.Reverse == 1 {
		a, b = b, a
	}
	if s.Form == "assignment" {
		return prefix + "let result = input; if " + x + " < " + k + " { result = " + a + " } else { result = " + b + " }; return result"
	}
	return prefix + "if " + x + " < " + k + " { return " + a + " } else { return " + b + " }"
}

// Authored goals are mathematical functions independent of original arm order.
func Required(s Spec, x int64) int64 {
	a, b := Values(s.Family, x, s.K)
	yes := x < s.K
	if s.Goal == 1 {
		yes = !yes
	}
	if yes {
		return a
	}
	return b
}
func Values(family string, x, k int64) (int64, int64) {
	switch family {
	case "bound":
		return x, k
	case "distance":
		return x - k, k - x
	case "offset":
		return x + k, x - k
	case "tag":
		return -k, k
	}
	panic("unknown source family")
}
func Cases(s Spec) []pathplan.TestCase {
	if s.Form == "rare" || s.Form == "contradiction" {
		cases := make([]pathplan.TestCase, 128)
		for i := range cases {
			cases[i] = pathplan.TestCase{Input: s.K, Expected: s.K}
		}
		if s.Form == "rare" {
			x := int64(-9007199254740995)
			cases[127] = pathplan.TestCase{Input: x, Expected: Required(s, x)}
		} else {
			cases[127].Expected = s.K + 1
			if s.Goal == 1 {
				cases[127].Expected = s.K - 1
			}
		}
		return cases
	}
	var cases []pathplan.TestCase
	for _, x := range []int64{-9007199254740995, 0, s.K - 1, s.K, s.K + 1, 9007199254740993, 9007199254740995, 18014398509481990} {
		cases = append(cases, pathplan.TestCase{Input: x, Expected: Required(s, x)})
	}
	return cases
}
func Gooo(s Spec) string {
	var b strings.Builder
	intent := "Choose the path matching every declared input and expected result."
	if s.Wording == 0 {
		intent = "선언된 입출력 사례에 맞는 경로를 선택한다."
	}
	fmt.Fprintf(&b, "package contractgoals\nnamespace contractgoals\nentity Integer id \"contractgoals://integer\"\nactivity Choose(Integer) -> Integer computes %s assembling {\n", strconv.Quote(Body(s)))
	fmt.Fprintln(&b, ` choice "comparison" operand_order at "0" intent "입력이 기준보다 작은지 확인한다. Compare input with the threshold."`)
	fmt.Fprintf(&b, " choice \"branches\" branch_layout at \"0\" intent %s\n", strconv.Quote(intent))
	for _, c := range Cases(s) {
		fmt.Fprintf(&b, " case \"%d\" -> \"%d\"\n", c.Input, c.Expected)
	}
	for _, x := range []int64{-9007199254740995, s.K, 18014398509481990} {
		fmt.Fprintf(&b, " condition_case \"comparison\" input \"%d\" -> \"%t\"\n", x, x < s.K)
	}
	fmt.Fprintln(&b, " attempts \"4\"\n}")
	return b.String()
}
