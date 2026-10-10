package record

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

func Body(s Spec) string {
	x, k, prefix := "input", fmt.Sprint(s.K), ""
	if s.Form == "assignment" {
		x, k = "quantity", "threshold"
		prefix = fmt.Sprintf("let quantity = input; let threshold = %d; let chosen = input; ", s.K)
	}
	a, b := x, k
	switch s.Family {
	case "distance":
		a, b = x+" - "+k, k+" - "+x
	case "affine":
		a, b = x+" + "+k, x+" * 3"
	case "bound":
	default:
		panic("unknown family")
	}
	if s.Reverse == 1 {
		a, b = b, a
	}
	if s.Form == "assignment" {
		return prefix + "if " + x + " < " + k + " { chosen = " + a + " } else { chosen = " + b + " }; return chosen"
	}
	return "if " + x + " < " + k + " { return " + a + " } else { return " + b + " }"
}

// The oracle is ordinary Go over the declared family. It does not inspect
// generated code, model outputs or candidate labels. int64 operations wrap.
func RequiredCondition(s Spec, x int64) bool {
	if s.ConditionGoal == 1 {
		return s.K < x
	}
	return x < s.K
}

func Required(s Spec, x int64) int64 {
	yes := x < s.K
	a, b := x, s.K
	switch s.Family {
	case "distance":
		a, b = x-s.K, s.K-x
	case "affine":
		yes = RequiredCondition(s, x)
		a, b = x+s.K, x*3
	case "bound":
	default:
		panic("unknown family")
	}
	if s.OutputGoal == 1 {
		yes = !yes
	}
	if yes {
		return a
	}
	return b
}

func Cases(s Spec) []pathplan.TestCase {
	inputs := []int64{-113, -1, 0, s.K - 1, s.K, s.K + 1, 1, 127}
	if s.Split == "sparse" {
		inputs = []int64{s.K}
	}
	var result []pathplan.TestCase
	for _, x := range inputs {
		result = append(result, pathplan.TestCase{Input: x, Expected: Required(s, x)})
	}
	return result
}

func Conditions(s Spec) []pathplan.ConditionCase {
	inputs := []int64{s.K - 1, s.K, s.K + 1}
	if s.Split == "sparse" {
		inputs = []int64{s.K}
	}
	var result []pathplan.ConditionCase
	for _, x := range inputs {
		result = append(result, pathplan.ConditionCase{ChoiceID: "comparison", Input: x, Expected: RequiredCondition(s, x)})
	}
	return result
}

func Probes(s Spec) []pathplan.TestCase {
	inputs := []int64{s.K - 5, s.K - 3, s.K - 2, s.K + 2, s.K + 3, s.K + 5,
		math.MinInt64, math.MinInt64 + 1, math.MaxInt64 - 1, math.MaxInt64,
		-9007199254740991, 9007199254740991, -5, 5, -101, 103}
	var result []pathplan.TestCase
	for _, x := range inputs {
		result = append(result, pathplan.TestCase{Input: x, Expected: Required(s, x)})
	}
	return result
}

func ProbeConditions(s Spec) []pathplan.ConditionCase {
	var result []pathplan.ConditionCase
	for _, x := range []int64{s.K - 3, s.K + 3, s.K} {
		result = append(result, pathplan.ConditionCase{ChoiceID: "comparison", Input: x, Expected: RequiredCondition(s, x)})
	}
	return result
}

func Gooo(s Spec) string { return GoooWithCases(s, Cases(s), Conditions(s)) }

func GoooWithCases(s Spec, cases []pathplan.TestCase, conditions []pathplan.ConditionCase) string {
	compare := []string{
		"사례에 적힌 참과 거짓을 얻도록 입력과 기준의 순서를 정한다.",
		"Order the input and threshold to produce the listed Boolean results.",
		"명시된 조건 결과를 보고 비교에 사용할 두 값의 배치를 정한다.",
		"Place the two comparison operands according to the stated condition examples.",
	}[s.Wording]
	branch := []string{
		"각 사례의 요구값을 돌려주도록 두 계산의 반환 위치를 정한다.",
		"Place the two computations in the return arms to meet each requested result.",
		"주어진 출력 사례와 조건 사례를 함께 지키도록 반환할 계산을 선택한다.",
		"Select the return layout that meets the provided outputs and condition cases together.",
	}[s.Wording]
	var b strings.Builder
	fmt.Fprintf(&b, "package canonicalstudy\nnamespace canonicalstudy\nentity Integer id \"canonicalstudy://integer\"\nactivity Choose(Integer) -> Integer computes %s assembling {\n", strconv.Quote(Body(s)))
	fmt.Fprintf(&b, " choice \"comparison\" operand_order at \"0\" intent %s\n", strconv.Quote(compare))
	fmt.Fprintf(&b, " choice \"branches\" branch_layout at \"0\" intent %s\n", strconv.Quote(branch))
	for _, c := range cases {
		fmt.Fprintf(&b, " case \"%d\" -> \"%d\"\n", c.Input, c.Expected)
	}
	for _, c := range conditions {
		fmt.Fprintf(&b, " condition_case %s input \"%d\" -> \"%t\"\n", strconv.Quote(c.ChoiceID), c.Input, c.Expected)
	}
	fmt.Fprintln(&b, " attempts \"4\"\n}")
	return b.String()
}
