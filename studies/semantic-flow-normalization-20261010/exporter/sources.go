package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

type specification struct {
	ID, Group, Form, Split, Body, Intent string
	K                                    int64
	Task                                 string
}

func branch(condition, yes, no string, reverse bool) string {
	if reverse {
		yes, no = no, yes
	}
	return fmt.Sprintf("if %s { %s } else { %s }", condition, yes, no)
}

func specifications() []specification {
	var all []specification
	for _, task := range []string{"max", "min"} {
		for _, k := range []int64{7, 13, 11, 17} {
			for reverse := range 2 {
				for wording := range 2 {
					group := fmt.Sprintf("%s-k%d-r%d-w%d", task, k, reverse, wording)
					intent := map[string][2]string{"max": {"큰 값을 반환한다. Return the larger value.", "Choose the greater integer. 더 큰 정수를 고른다."}, "min": {"작은 값을 반환한다. Return the smaller value.", "Choose the lesser integer. 더 작은 정수를 고른다."}}[task][wording]
					constant := fmt.Sprint(k)
					rev := reverse == 1
					bodies := []string{
						branch("input < "+constant, "return input", "return "+constant, rev),
						"let value = input; let limit = " + constant + "; " + branch("value < limit", "return value", "return limit", rev),
						"let value = input; let limit = " + constant + "; let result = input; " + branch("value < limit", "result = value", "result = limit", rev) + "; return result",
						"let value = input; let saved = value; value = " + constant + "; " + branch("saved < value", "return saved", "return value", rev),
						"let result = input; " + branch("input < "+constant, "result = input", "result = "+constant, rev) + "; let saved = result; result = " + constant + "; return saved",
					}
					for i, form := range []string{"direct", "alias", "assignment", "copy_overwrite", "copy_after_branch"} {
						split := "constant_transfer"
						if k == 7 || k == 13 {
							split = "representation_transfer"
							if i == 0 {
								split = "future_train"
							}
						}
						all = append(all, specification{group + "-" + form, group, form, split, bodies[i], intent, k, task})
					}
				}
			}
		}
	}
	all = append(all,
		specification{"control-nested", "control-nested", "nested", "control", "if input < 10 { if input < 0 { return 0 } else { return input } } else { return 10 }", "큰 값을 반환한다. Return the larger value.", 10, "max"},
		specification{"control-arithmetic", "control-arithmetic", "arithmetic", "control", "if input < 10 { return input + 1 } else { return 10 }", "작은 입력에는 1을 더한다. Add one below the threshold.", 10, "increment_below"})
	return all
}

func source(s specification) string {
	var b strings.Builder
	fmt.Fprintf(&b, "package semanticflow\nnamespace semanticflow\nentity Integer id \"semanticflow://integer\"\nactivity Choose(Integer) -> Integer computes %s assembling {\n", strconv.Quote(s.Body))
	fmt.Fprintln(&b, ` choice "comparison" operand_order at "0" intent "입력이 기준값보다 작은지 판단한다. Compare input with the threshold."`)
	fmt.Fprintf(&b, " choice \"branches\" branch_layout at \"0\" intent %s\n", strconv.Quote(s.Intent))
	for _, c := range cases(s) {
		fmt.Fprintf(&b, " case \"%d\" -> \"%d\"\n", c.Input, c.Expected)
	}
	for _, input := range []int64{-9007199254740995, s.K, 18014398509481990} {
		fmt.Fprintf(&b, " condition_case \"comparison\" input \"%d\" -> \"%t\"\n", input, input < s.K)
	}
	fmt.Fprintln(&b, " attempts \"4\"\n}")
	return b.String()
}

func cases(s specification) []pathplan.TestCase {
	var rows []pathplan.TestCase
	for _, input := range []int64{-9007199254740995, 0, s.K - 1, s.K, s.K + 1, 9007199254740993, 9007199254740995, 18014398509481990} {
		expected := max(input, s.K)
		if s.Task == "min" {
			expected = min(input, s.K)
		}
		if s.Task == "increment_below" {
			expected = s.K
			if input < s.K {
				expected = input + 1
			}
		}
		rows = append(rows, pathplan.TestCase{Input: input, Expected: expected})
	}
	return rows
}
