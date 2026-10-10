package record

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

func arms(family, x, k string) (string, string) {
	switch family {
	case "bound":
		return x, k
	case "distance":
		return x + " - " + k, k + " - " + x
	case "negative_bound":
		return "0 - " + x, "0 - " + k
	}
	panic("unknown family")
}
func Body(s Spec) string {
	x, k, prefix := "input", fmt.Sprint(s.K), ""
	comparisonValue := x
	if s.Form == "rare" {
		prefix = fmt.Sprintf("let observed = %d; if input < 0 { observed = input }; ", s.K)
		comparisonValue = "observed"
	}
	if s.Form == "alias" || s.Form == "assignment" || s.Form == "rebinding" {
		x, k = "value", "limit"
		prefix = fmt.Sprintf("let value = input; let limit = %d; ", s.K)
	}
	if s.Form == "rebinding" {
		prefix = fmt.Sprintf("let value = 0; value = input; let limit = 0; limit = %d; ", s.K)
	}
	if s.Form != "rare" {
		comparisonValue = x
	}
	a, b := arms(s.Family, x, k)
	if s.Reverse == 1 {
		a, b = b, a
	}
	if s.Form == "assignment" {
		return prefix + "let result = input; if " + comparisonValue + " < " + k + " { result = " + a + " } else { result = " + b + " }; return result"
	}
	return prefix + "if " + comparisonValue + " < " + k + " { return " + a + " } else { return " + b + " }"
}
func Required(s Spec, x int64) int64 {
	yes := x < s.K
	if s.OutputGoal == 1 {
		yes = !yes
	}
	var a, b int64
	switch s.Family {
	case "bound":
		a, b = x, s.K
	case "distance":
		a, b = x-s.K, s.K-x
	case "negative_bound":
		a, b = -x, -s.K
	default:
		panic("unknown family")
	}
	if yes {
		return a
	}
	return b
}
func Cases(s Spec) []pathplan.TestCase {
	var cases []pathplan.TestCase
	inputs := []int64{-9007199254740995, 0, s.K - 1, s.K, s.K + 1, 9007199254740993, 9007199254740995, 18014398509481990}
	if s.Form == "rare" {
		inputs = []int64{-9007199254740995, -8, -7, -6, -5, -4, -3, -2}
	}
	for _, x := range inputs {
		cases = append(cases, pathplan.TestCase{Input: x, Expected: Required(s, x)})
	}
	return cases
}
func Conditions(s Spec) []pathplan.ConditionCase {
	if s.Form == "empty" {
		return nil
	}
	inputs := []int64{-9007199254740995, s.K, 18014398509481990}
	if s.Form == "rare" {
		inputs = make([]int64, 128)
		for i := range inputs {
			inputs[i] = int64(i)
		}
		inputs[127] = -9007199254740995
	}
	var cases []pathplan.ConditionCase
	for _, x := range inputs {
		value := x
		if s.Form == "rare" && x >= 0 {
			value = s.K
		}
		want := value < s.K
		if s.ConditionGoal == 1 {
			want = s.K < value
		}
		cases = append(cases, pathplan.ConditionCase{ChoiceID: "comparison", Input: x, Expected: want})
	}
	if s.Form == "contradiction" {
		cases[2].Expected = cases[0].Expected
		cases = append(cases, pathplan.ConditionCase{ChoiceID: "comparison", Input: s.K - 2, Expected: cases[0].Expected})
	}
	return cases
}
func Gooo(s Spec) string {
	var b strings.Builder
	intent := []string{"출력 예시와 중간 조건을 함께 만족하는 처리 순서를 고른다.", "Choose a route that satisfies both the listed outputs and intermediate conditions.", "예시의 결과와 비교 조건을 모두 지키도록 본문을 구성한다."}[s.Wording]
	fmt.Fprintf(&b, "package requirements\nnamespace requirements\nentity Integer id \"requirements://integer\"\nactivity Choose(Integer) -> Integer computes %s assembling {\n", strconv.Quote(Body(s)))
	choices := []string{` choice "comparison" operand_order at "0" intent "작성한 입력에서 비교 결과를 맞춘다. Match the declared comparison results."`, " choice \"branches\" branch_layout at \"0\" intent " + strconv.Quote(intent)}
	if s.Form == "rare" {
		for i := range choices {
			choices[i] = strings.Replace(choices[i], `at "0"`, `at "1"`, 1)
		}
	}
	if s.Order == 1 {
		choices[0], choices[1] = choices[1], choices[0]
	}
	for _, choice := range choices {
		fmt.Fprintln(&b, choice)
	}
	for _, c := range Cases(s) {
		fmt.Fprintf(&b, " case \"%d\" -> \"%d\"\n", c.Input, c.Expected)
	}
	for _, c := range Conditions(s) {
		fmt.Fprintf(&b, " condition_case \"comparison\" input \"%d\" -> \"%t\"\n", c.Input, c.Expected)
	}
	fmt.Fprintln(&b, " attempts \"4\"\n}")
	return b.String()
}
