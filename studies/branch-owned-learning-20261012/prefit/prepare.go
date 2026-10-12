package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type probe struct {
	Input    int64 `json:"input"`
	Expected int64 `json:"expected"`
}
type specification struct {
	ID        string   `json:"id"`
	Split     string   `json:"split"`
	Family    int      `json:"family"`
	Parameter int      `json:"parameter"`
	Goal      int      `json:"goal"`
	Language  string   `json:"language"`
	Wording   int      `json:"wording"`
	Parity    int      `json:"parity"`
	Form      int      `json:"form"`
	Contract  string   `json:"contract"`
	PartsSHA  string   `json:"parts_sha256"`
	Source    string   `json:"source"`
	SourceSHA string   `json:"source_sha256"`
	Probes    [4]probe `json:"probes"`
}

var operators = [4]string{"+", "-", "*", "/"}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func digest(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func calculate(x int64, operator string, n int64) int64 {
	switch operator {
	case "+":
		return x + n
	case "-":
		return x - n
	case "*":
		return x * n
	case "/":
		return x / n
	}
	panic("operator")
}
func intentions(language string, wording, goal int, cut int64, first, second string) (string, string) {
	if language == "en" {
		word := "below"
		if goal == 1 {
			word = "above"
		}
		if wording == 0 {
			return fmt.Sprintf("Check whether input is %s %d.", word, cut), fmt.Sprintf("When that condition holds use %s; otherwise use %s.", first, second)
		}
		return fmt.Sprintf("The comparison should be true only for an input strictly %s %d.", word, cut), fmt.Sprintf("Return %s in the true case and %s in the false case.", first, second)
	}
	word := "미만"
	if goal == 1 {
		word = "초과"
	}
	if wording == 0 {
		return fmt.Sprintf("입력이%d %s인지 확인한다.", cut, word), fmt.Sprintf("조건이 맞으면%s, 그 밖에는%s을 반환한다.", first, second)
	}
	return fmt.Sprintf("%d %s인 입력에서만 비교 결과를 참으로 둔다.", cut, word), fmt.Sprintf("참인 가지는%s을 계산한다. 거짓인 가지는%s을 계산한다.", first, second)
}
func main() {
	if len(os.Args) != 2 {
		panic("one prefit directory required")
	}
	root := os.Args[1]
	if _, err := os.Stat(filepath.Join(root, "specifications.json")); !os.IsNotExist(err) {
		panic("specifications already exist")
	}
	must(os.MkdirAll(filepath.Join(root, "sources"), 0700))
	rows := make([]specification, 0, 1536)
	counts := map[string]int{}
	contracts := map[string]bool{}
	ids := map[string]bool{}
	for family := range 16 {
		split := "evaluation"
		if (7*family+3)%16 < 8 {
			split = "train"
		} else if (7*family+3)%16 < 12 {
			split = "validation"
		}
		for parameter := range 3 {
			cut := int64(811 + 19*family + 37*parameter)
			firstN, secondN := int64(17+family%7), int64(29+(3*family)%11)
			if parameter == 1 {
				firstN, secondN = -firstN, -secondN
			}
			if parameter == 2 {
				firstN += 9007199254740993
				secondN += 9007199254741033
			}
			firstOp, secondOp := operators[family/4], operators[family%4]
			initial := [8]int64{-9007199254741021, -41, -1, 0, cut - 1, cut, cut + 1, 9007199254741025}
			probeInputs := [4]int64{-9007199254741063, cut - 17, cut + 17, 9007199254741059}
			for _, x := range probeInputs {
				if slices.Contains(initial[:], x) {
					panic("probe overlaps declared outputs")
				}
			}
			for goal := range 2 {
				condition := func(x int64) bool {
					if goal == 0 {
						return x < cut
					}
					return x > cut
				}
				want := func(x int64) int64 {
					if condition(x) {
						return calculate(x, firstOp, firstN)
					}
					return calculate(x, secondOp, secondN)
				}
				contract := fmt.Sprintf("branch-owned-20261012://f%02d-p%d-g%d", family, parameter, goal)
				contracts[contract] = true
				for _, language := range [2]string{"ko", "en"} {
					for wording := range 1 {
						firstText, secondText := fmt.Sprintf("input %s %d", firstOp, firstN), fmt.Sprintf("input %s %d", secondOp, secondN)
						comparison, branch := intentions(language, wording, goal, cut, firstText, secondText)
						for parity := range 4 {
							for form := range 2 {
								id := fmt.Sprintf("owned-%02d-%d-%d-%s-%d-%d-%d", family, parameter, goal, language, wording, parity, form)
								if ids[id] {
									panic("duplicate id")
								}
								ids[id] = true
								left, right := fmt.Sprint(cut), "load"
								if parity&1 != 0 {
									left, right = right, left
								}
								first, second := fmt.Sprintf("load %s %d", firstOp, firstN), fmt.Sprintf("load %s %d", secondOp, secondN)
								if parity&2 != 0 {
									first, second = second, first
								}
								body := fmt.Sprintf("let load = input\nif %s < %s { return %s } else { return %s }", left, right, first, second)
								if form == 1 {
									body = fmt.Sprintf("let load = input\nlet result = load\nif %s < %s { result = %s } else { result = %s }\nreturn result", left, right, first, second)
								}
								var source strings.Builder
								fmt.Fprintf(&source, "package ownedroute\nnamespace ownedroute\nentity Integer id %q\nactivity Choose(Integer) -> Integer id %q computes `%s` assembling {\n", contract+"/integer", contract+"/choose", body)
								fmt.Fprintf(&source, " choice \"comparison\" operand_order at \"0\" intent %q\n choice \"branches\" branch_layout at \"0\" intent %q\n", comparison, branch)
								for _, x := range initial {
									fmt.Fprintf(&source, " case %q -> %q\n", fmt.Sprint(x), fmt.Sprint(want(x)))
								}
								for _, x := range [3]int64{cut - 1, cut, cut + 1} {
									fmt.Fprintf(&source, " condition_case \"comparison\" input %q -> %q\n", fmt.Sprint(x), fmt.Sprint(condition(x)))
								}
								source.WriteString(" attempts \"4\"\n}\n")
								raw := []byte(source.String())
								name := "sources/" + id + ".gooo"
								must(os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), raw, 0600))
								row := specification{ID: id, Split: split, Family: family, Parameter: parameter, Goal: goal, Language: language, Wording: wording, Parity: parity, Form: form, Contract: contract, PartsSHA: digest([]byte(body)), Source: name, SourceSHA: digest(raw)}
								for i, x := range probeInputs {
									row.Probes[i] = probe{Input: x, Expected: want(x)}
								}
								rows = append(rows, row)
								counts[split]++
							}
						}
					}
				}
			}
		}
	}
	if len(rows) != 1536 || len(contracts) != 96 || counts["train"] != 768 || counts["validation"] != 384 || counts["evaluation"] != 384 {
		panic("factor coverage")
	}
	raw, err := json.MarshalIndent(rows, "", "  ")
	must(err)
	must(os.WriteFile(filepath.Join(root, "specifications.json"), append(raw, '\n'), 0600))
	fmt.Println("Prepared1536 source documents/96goal contracts/16operator families;train768/validation384/evaluation384;model calls0/fits0/native executions0.")
}
