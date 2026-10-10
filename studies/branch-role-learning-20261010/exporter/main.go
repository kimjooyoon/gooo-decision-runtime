package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
	"github.com/kimjooyoon/meta-ontology-go/internal/bodycodegen"
)

const compilerRevision = "1ceb96b99fa6fabe12648f5ec6deac47a0d546e1"

type sourceRecord struct {
	ID       string            `json:"id"`
	Split    string            `json:"split"`
	Source   string            `json:"gooo_source"`
	SHA      string            `json:"source_sha256"`
	Document pathplan.Document `json:"document"`
}
type dataset struct {
	Schema   string         `json:"schema"`
	Producer string         `json:"producer_revision"`
	Compiler string         `json:"compiler_revision"`
	SDK      string         `json:"sdk_version"`
	Go       string         `json:"go_version"`
	Records  []sourceRecord `json:"records"`
}
type specification struct {
	id, split, body string
	choices         []string
	cases           []pathplan.TestCase
	conditions      []pathplan.ConditionCase
}

func hash(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func branch(condition, yes, no string, reversed bool) string {
	if reversed {
		yes, no = no, yes
	}
	return fmt.Sprintf("if %s { %s } else { %s }", condition, yes, no)
}
func choice(id, kind, occurrence, intent string) string {
	return fmt.Sprintf(" choice %q %s at %q intent %s\n", id, kind, occurrence, strconv.Quote(intent))
}
func inputs(k int64) []int64 {
	return []int64{-9007199254740995, k - 1, k, k + 1, 0, 9007199254740993, 9007199254740995, 18014398509481990}
}

func minmax(task string, k int64, reversed, local bool, wording int) specification {
	split := "train"
	if wording > 0 {
		split = "wording"
	}
	if local {
		split = "structure"
		if wording > 0 {
			split = "both"
		}
	}
	value, limit, prefix := "input", fmt.Sprint(k), ""
	if local {
		value, limit, prefix = "value", "limit", fmt.Sprintf("let value = input\nlet limit = %d\n", k)
	}
	s := specification{id: fmt.Sprintf("%s-k%d-r%t-local%t-w%d", task, k, reversed, local, wording), split: split}
	s.body = prefix + branch(value+" < "+limit, "return "+limit, "return "+value, reversed)
	comparison := [3]string{"Check whether the input is below the threshold. 입력이 기준값보다 작은지 판단한다.", "입력값 < 기준값 조건을 선택한다.", "Is the incoming integer smaller than the bound?"}[wording]
	intent := map[string][3]string{
		"max": {"Return the larger of the input and threshold. 입력과 기준값 중 큰 값을 반환한다.", "둘 중 더 큰 수를 결과로 삼는다.", "Keep the input at least as large as the bound."},
		"min": {"Return the smaller of the input and threshold. 입력과 기준값 중 작은 값을 반환한다.", "둘 중 더 작은 수를 결과로 삼는다.", "Cap the incoming value at the bound."},
	}[task][wording]
	s.choices = []string{choice("comparison", "operand_order", "0", comparison), choice("branches", "branch_layout", "0", intent)}
	for _, x := range inputs(k) {
		want := max(x, k)
		if task == "min" {
			want = min(x, k)
		}
		s.cases = append(s.cases, pathplan.TestCase{Input: x, Expected: want})
	}
	for _, x := range []int64{-9007199254740995, k, 9007199254740995} {
		s.conditions = append(s.conditions, pathplan.ConditionCase{ChoiceID: "comparison", Input: x, Expected: x < k})
	}
	return s
}

func clamp(k int64, outer, inner bool) specification {
	s := specification{id: fmt.Sprintf("clamp-k%d-outer%t-inner%t", k, outer, inner), split: "nested"}
	nested := branch(fmt.Sprintf("%d < input", k), fmt.Sprintf("return %d", k), "return input", inner)
	s.body = branch(fmt.Sprintf("input < %d", -k), fmt.Sprintf("return %d", -k), nested, outer)
	s.choices = []string{
		choice("lower", "branch_layout", "0", "Keep the result above the lower bound. 결과를 하한 이상으로 유지한다."),
		choice("upper", "branch_layout", "1", "Keep the result below the upper bound. 결과를 상한 이하로 유지한다."),
	}
	for _, x := range []int64{-9007199254740995, -k - 1, -k, -k + 1, 0, k - 1, k, k + 1, 9007199254740993, 9007199254740995, 18014398509481990} {
		s.cases = append(s.cases, pathplan.TestCase{Input: x, Expected: min(max(x, -k), k)})
	}
	return s
}

func sign(reversed bool, wording int) specification {
	s := specification{id: fmt.Sprintf("sign-r%t-w%d", reversed, wording), split: "literal_collision"}
	s.body = branch("input < 0", "return -1", "return 1", reversed)
	intent := [2]string{"Return -1 for a negative input and 1 otherwise.", "음수 입력에는 -1, 그 외에는 1을 반환한다."}[wording]
	s.choices = []string{choice("branches", "branch_layout", "0", intent)}
	for _, x := range []int64{-9007199254740995, -1, 0, 1, 9007199254740993, 9007199254740995, 18014398509481990} {
		want := int64(1)
		if x < 0 {
			want = -1
		}
		s.cases = append(s.cases, pathplan.TestCase{Input: x, Expected: want})
	}
	return s
}

func specifications() []specification {
	var specs []specification
	for _, task := range []string{"max", "min"} {
		for _, k := range []int64{-10, 10} {
			for _, reversed := range []bool{false, true} {
				for _, local := range []bool{false, true} {
					for wording := range 3 {
						specs = append(specs, minmax(task, k, reversed, local, wording))
					}
				}
			}
		}
	}
	for _, k := range []int64{10, 20} {
		for _, outer := range []bool{false, true} {
			for _, inner := range []bool{false, true} {
				specs = append(specs, clamp(k, outer, inner))
			}
		}
	}
	for _, reversed := range []bool{false, true} {
		for wording := range 2 {
			specs = append(specs, sign(reversed, wording))
		}
	}
	return specs
}

func lower(s specification) (sourceRecord, error) {
	var source strings.Builder
	fmt.Fprintf(&source, "package branchlearning\nnamespace branchlearning\nentity Integer id \"branchlearning://integer\"\nactivity Choose(Integer) -> Integer computes %s assembling {\n", strconv.Quote(s.body))
	for _, c := range s.choices {
		source.WriteString(c)
	}
	for _, c := range s.cases {
		fmt.Fprintf(&source, " case \"%d\" -> \"%d\"\n", c.Input, c.Expected)
	}
	for _, c := range s.conditions {
		fmt.Fprintf(&source, " condition_case %q input \"%d\" -> \"%t\"\n", c.ChoiceID, c.Input, c.Expected)
	}
	fmt.Fprintf(&source, " attempts \"%d\"\n}\n", 1<<len(s.choices))
	doc, err := bodycodegen.DecodeSourcePathDocument(context.Background(), s.id+".gooo", []byte(source.String()), "Choose", nil)
	return sourceRecord{s.id, s.split, source.String(), hash([]byte(source.String())), doc}, err
}

func producer() (string, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", fmt.Errorf("build identity unavailable")
	}
	var revision string
	modified, sdk := true, false
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			revision = s.Value
		}
		if s.Key == "vcs.modified" {
			modified = s.Value != "false"
		}
	}
	for _, dep := range info.Deps {
		if dep.Path == "github.com/kimjooyoon/gooo-decision-runtime" {
			sdk = dep.Version == "v0.2.31-experimental" && dep.Replace == nil
		}
	}
	if revision == "" || modified || !sdk {
		return "", fmt.Errorf("clean committed producer and public SDK31 required")
	}
	return revision, nil
}

func run(compiler, out string) error {
	revision, err := producer()
	if err != nil {
		return err
	}
	head, err := exec.Command("git", "-C", compiler, "rev-parse", "HEAD").Output()
	if err != nil || strings.TrimSpace(string(head)) != compilerRevision {
		return fmt.Errorf("compiler source revision differs")
	}
	status, err := exec.Command("git", "-C", compiler, "status", "--porcelain").Output()
	if err != nil || len(status) != 0 {
		return fmt.Errorf("clean compiler checkout required")
	}
	if err := os.Mkdir(out, 0700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "started.txt"), []byte("Original v2 source export; preserve partial output.\n"), 0600); err != nil {
		return err
	}
	d := dataset{"gooo/branch-role-source-dataset/v1", revision, compilerRevision, "v0.2.31-experimental", runtime.Version(), nil}
	for _, spec := range specifications() {
		record, err := lower(spec)
		if err != nil {
			return fmt.Errorf("%s: %w", spec.id, err)
		}
		d.Records = append(d.Records, record)
	}
	raw, err := json.Marshal(d)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "dataset.json"), raw, 0600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, "completed.txt"), []byte("60 source documents lowered; 0 training calls; 0 model predictions; 0 native executions.\n"), 0600)
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: branch-role-export COMPILER_CHECKOUT FRESH_OUTPUT_DIRECTORY")
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
