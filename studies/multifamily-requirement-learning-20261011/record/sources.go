package record

import (
	"fmt"
	"strconv"
	"strings"
)

// Both intents change with language/wording, while authored outputs and Boolean
// conditions determine the target. No new study reuses a historical frozen row.
func Gooo(s Spec) string {
	comparison := []string{
		"입력과 기준값의 비교 방향을 사례에 맞게 고른다.",
		"Choose the comparison direction to match the declared Boolean cases.",
		"각 입력에서 요구한 참과 거짓이 나오도록 비교식을 배치한다.",
		"Arrange the predicate operands so each listed true or false result is met.",
	}[s.Wording]
	branch := []string{
		"정해진 출력과 비교의 참거짓이 모두 맞도록 분기를 고른다.",
		"Choose branch returns that satisfy the output cases and comparison conditions together.",
		"출력 사례와 중간 조건을 동시에 만족하는 반환 경로를 구성한다.",
		"Arrange these two return arms to meet every output example and intermediate condition.",
	}[s.Wording]
	var b strings.Builder
	fmt.Fprintf(&b, "package requirements\nnamespace requirements\nentity Integer id \"requirements://integer\"\nactivity Choose(Integer) -> Integer computes %s assembling {\n", strconv.Quote(Body(s)))
	fmt.Fprintf(&b, " choice \"comparison\" operand_order at \"0\" intent %s\n", strconv.Quote(comparison))
	fmt.Fprintf(&b, " choice \"branches\" branch_layout at \"0\" intent %s\n", strconv.Quote(branch))
	for _, c := range Cases(s) {
		fmt.Fprintf(&b, " case \"%d\" -> \"%d\"\n", c.Input, c.Expected)
	}
	for _, c := range Conditions(s) {
		fmt.Fprintf(&b, " condition_case \"comparison\" input \"%d\" -> \"%t\"\n", c.Input, c.Expected)
	}
	fmt.Fprintln(&b, " attempts \"4\"\n}")
	return b.String()
}
