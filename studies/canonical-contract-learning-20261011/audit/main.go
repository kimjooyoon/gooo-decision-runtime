// Audit retained original data. No model prediction, fit, source lowering or
// candidate execution is performed. Candidate arithmetic is independently read.
package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-contract-learning-20261011/record"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func need(ok bool, reason string) {
	if !ok {
		panic(reason)
	}
}
func load(root, name string) []byte {
	raw, err := os.ReadFile(filepath.Join(root, name))
	if os.IsNotExist(err) {
		raw, err = os.ReadFile(filepath.Join(root, name+".gz"))
		must(err)
		z, err := gzip.NewReader(bytes.NewReader(raw))
		must(err)
		raw, err = io.ReadAll(io.LimitReader(z, 128<<20))
		must(err)
		must(z.Close())
	} else {
		must(err)
	}
	return raw
}
func decode[T any](raw []byte) T { var v T; must(json.Unmarshal(raw, &v)); return v }
func lines[T any](raw []byte) []T {
	var result []T
	s := bufio.NewScanner(bytes.NewReader(raw))
	s.Buffer(make([]byte, 4096), 4<<20)
	for s.Scan() {
		result = append(result, decode[T](s.Bytes()))
	}
	must(s.Err())
	return result
}

// Direct int64 reference for the source template and the recorded mask.
func actual(s r.Spec, mask uint16, x int64) (int64, bool) {
	predicate := x < s.K
	if mask&1 != 0 {
		predicate = s.K < x
	}
	first := predicate
	if (s.Reverse == 1) != (mask&2 != 0) {
		first = !first
	}
	a, b := x, s.K
	switch s.Family {
	case "distance":
		a, b = x-s.K, s.K-x
	case "affine":
		a, b = x+s.K, x*3
	case "bound":
	default:
		panic("unknown family")
	}
	if first {
		return a, predicate
	}
	return b, predicate
}

func observations(s r.Spec, mask uint16, got r.Observations, cases []pathplan.TestCase, conditions []pathplan.ConditionCase) bool {
	need(len(got.Outputs) == len(cases) && len(got.Conditions) == len(conditions), "observation scope")
	valid := true
	for i, c := range cases {
		value, _ := actual(s, mask, c.Input)
		want := pathplan.TestResult{Input: c.Input, Expected: c.Expected, Actual: value, Passed: value == c.Expected}
		need(got.Outputs[i] == want, "independent exact output")
		valid = valid && want.Passed
	}
	for i, c := range conditions {
		value, predicate := actual(s, mask, c.Input)
		o := got.Conditions[i]
		pass := predicate == c.Expected
		status := "MISMATCH"
		if pass {
			status = "MATCH"
		}
		need(o.Case == c && o.Observation.Reached && o.Observation.Value == predicate && o.Output.Type == decision.TypeInt && o.Output.Int == value && o.Passed == pass && o.Status == status, "independent condition")
		valid = valid && pass
	}
	need(valid == r.Passed(got), "observation aggregate")
	return valid
}

func checkSource(x r.Source, spec r.Spec) {
	need(x.Spec == spec && x.Gooo == r.Gooo(spec) && x.SourceSHA == r.Hash([]byte(x.Gooo)), "source identity")
	need(reflect.DeepEqual(x.Probes, r.Probes(spec)) && reflect.DeepEqual(x.ProbeConditions, r.ProbeConditions(spec)), "frozen probe expectations")
	need(reflect.DeepEqual(x.Document.TestCases, r.Cases(spec)) && reflect.DeepEqual(x.Document.Plan.ConditionCases, r.Conditions(spec)), "authored cases")
	p, err := x.Document.Prepare()
	must(err)
	input, err := p.InitialContractInput(x.Document.TestCases)
	must(err)
	need(p.PlanSHA256() == x.PlanSHA && input.CaseSHA256() == x.CasesSHA, "bound source identity")
	need(len(x.LegacyInputs) == 2 && len(x.CanonicalInputs) == 2 && len(x.Candidates) == 4, "source counts")
	for i, c := range x.Document.Plan.Decisions {
		var a, b [528]float32
		must(input.OrderedSourceFeaturesInto(c.ID, &a))
		must(input.CanonicalOrderedSourceFeaturesInto(c.ID, &b))
		need(a == x.LegacyInputs[i] && b == x.CanonicalInputs[i], "source arrays")
	}
	need(len(x.OutputRows) == input.CaseCount() && len(x.ConditionRows) == input.ConditionCount(), "goal array counts")
	for i, row := range x.OutputRows {
		want, e := input.CaseFeatures(i)
		must(e)
		need(row == want, "output array")
	}
	for i, row := range x.ConditionRows {
		want, e := input.ConditionFeatures(i)
		must(e)
		need(row == want, "condition array")
	}
	var declared, probe uint64
	for mask, c := range x.Candidates {
		need(c.Mask == uint16(mask) && c.Gooo != "" && c.Go != "", "candidate identity")
		d := observations(spec, c.Mask, c.Declared, x.Document.TestCases, x.Document.Plan.ConditionCases)
		p := observations(spec, c.Mask, c.Probe, x.Probes, x.ProbeConditions)
		need(d == c.Acceptable && p == c.ProbeAcceptable, "candidate labels")
		if d {
			declared |= 1 << mask
		}
		if p {
			probe |= 1 << mask
		}
	}
	need(x.Acceptable == declared && x.ProbeAcceptable == probe && probe != 0 && probe&(probe-1) == 0, "complete mask labels")
	if spec.Split == "sparse" {
		need(declared == 15, "sparse ambiguity")
	}
}

func selectedMask(d pathplan.Document, selected map[string]string) uint16 {
	var mask uint16
	need(len(selected) == 2, "complete selection")
	for i, c := range d.Plan.Decisions {
		label := selected[c.ID]
		need(label == c.Options[0].Label || label == c.Options[1].Label, "declared option")
		if label == c.Options[1].Label {
			mask |= 1 << i
		}
	}
	return mask
}

func checkSearch(x r.Source, row r.Search, report r.Report) {
	need(row.ID == x.Spec.ID && row.Split == x.Spec.Split && row.Family == x.Spec.Family && row.Form == x.Spec.Form, "search source")
	need(len(row.Stages) >= 1 && len(row.Stages) <= 3, "stage bound")
	cases, conditions := r.Cases(x.Spec), r.Conditions(x.Spec)
	for number, stage := range row.Stages {
		need(stage.Number == number && stage.Gooo == r.GoooWithCases(x.Spec, cases, conditions) && stage.SourceSHA == r.Hash([]byte(stage.Gooo)), "augmented Gooo source")
		need(reflect.DeepEqual(stage.Document.TestCases, cases) && reflect.DeepEqual(stage.Document.Plan.ConditionCases, conditions), "only selected counterexample added")
		p, e := stage.Document.Prepare()
		must(e)
		input, e := p.InitialContractInput(cases)
		must(e)
		need(stage.PlanSHA == p.PlanSHA256() && stage.CasesSHA == input.CaseSHA256(), "stage identity")
		ranking := stage.Ranking
		need(ranking.Proposed < 4 && ranking.ChoiceCount == 2 && ranking.Error == "" && stage.NS > 0, "bounded successful ranking")
		calls := 1
		if row.Mode == "fixed" {
			calls = 0
		} else {
			need(ranking.ModelFingerprint == report.Models[row.Mode].Fingerprint, "fixed saved model across stages")
		}
		need(ranking.Calls == calls && ranking.PlanSHA == stage.PlanSHA && ranking.CaseSHA == stage.CasesSHA && !ranking.Declined && ranking.CaseCount == len(cases), "ranking contract")
		if row.Mode == "canonical" {
			need(ranking.SourceFeatures == contractdecision.CanonicalOrderedSourceFeatureVersion, "canonical input route")
		}
		if row.Mode == "legacy" {
			need(ranking.SourceFeatures == contractdecision.OrderedSourceFeatureVersion, "legacy input route")
		}
		need(len(stage.Progress) >= 2 && len(stage.Progress) <= 5 && stage.Progress[0].Attempted == 0 && len(stage.Progress[0].NewAttempts) == 0, "prediction before execution")
		seen := map[uint16]bool{}
		for j, progress := range stage.Progress {
			need(progress.RankingSHA == ranking.SHA && progress.PredictionsThisAdvance == 0 && progress.CaseSHA == stage.CasesSHA, "bound progress")
			if j == 0 {
				continue
			}
			need(progress.Attempted == j && len(progress.NewAttempts) == 1, "one immediate candidate")
			attempt := progress.NewAttempts[0]
			need(attempt.Mask < 4 && !seen[attempt.Mask] && selectedMask(stage.Document, attempt.Choices) == attempt.Mask, "finite mask uniqueness")
			seen[attempt.Mask] = true
			if j == 1 {
				need(attempt.Mask == ranking.Proposed, "initial proposal used immediately")
			}
			observations(x.Spec, attempt.Mask, r.Observations{Outputs: attempt.Results, Conditions: attempt.Conditions}, cases, conditions)
			passed := 0
			for _, c := range attempt.Results {
				if c.Passed {
					passed++
				}
			}
			need(attempt.Passed == passed && attempt.Total == len(cases), "attempt fractions")
		}
		last := stage.Progress[len(stage.Progress)-1]
		mask := selectedMask(stage.Document, last.Selection.Choices)
		need(seen[mask] && last.Status == "TRAINING_COMPLETE" && last.SelectedPassed == len(cases) && last.Cases == len(cases), "selected declared completion")
		need(observations(x.Spec, mask, r.Observations{Outputs: last.BestCases, Conditions: last.Selection.Conditions}, cases, conditions), "selected declared observations")
		valid := observations(x.Spec, mask, stage.Probe, x.Probes, x.ProbeConditions)
		if number+1 == len(row.Stages) {
			need(stage.NextOutput == nil && stage.NextCondition == nil, "terminal stage has no unconsumed feedback")
			need(x.Spec.Split != "sparse" || valid || number == 2, "sparse loop stopped early")
			continue
		}
		need(x.Spec.Split == "sparse" && !valid, "feedback scope")
		var output *pathplan.TestCase
		var condition *pathplan.ConditionCase
		for _, c := range stage.Probe.Outputs {
			if !c.Passed {
				v := pathplan.TestCase{Input: c.Input, Expected: c.Expected}
				output = &v
				break
			}
		}
		if output == nil {
			for _, c := range stage.Probe.Conditions {
				if !c.Passed {
					v := c.Case
					condition = &v
					break
				}
			}
		}
		need(reflect.DeepEqual(stage.NextOutput, output) && reflect.DeepEqual(stage.NextCondition, condition), "first failed probe chosen")
		if output != nil {
			cases = append(cases, *output)
		} else {
			need(condition != nil, "missing counterexample")
			conditions = append(conditions, *condition)
		}
	}
}

type Comparison struct {
	Improved, Regressed, Unchanged int
	Regressions                    []string
}
type Pair struct{ Pairs, SameInput, SameScores, SameProposal int }
type Latency struct {
	Count           int
	MedianNS, P95NS int64
}
type Summary struct {
	Schema                                              string
	Sources, Searches, Stages, ModelCalls, TrainingRows int
	Comparisons                                         map[string]Comparison
	Pairs                                               map[string]Pair
	Latency                                             map[string]Latency
	Groups                                              map[string]r.Group
}

func audit(root string) Summary {
	report := decode[r.Report](load(root, "report.json"))
	sourceRaw, searchRaw := load(root, "sources.jsonl"), load(root, "searches.jsonl")
	need(r.Hash(sourceRaw) == report.SourcesSHA && r.Hash(searchRaw) == report.SearchesSHA, "original record digests")
	sources, searches := lines[r.Source](sourceRaw), lines[r.Search](searchRaw)
	need(len(sources) == 320 && len(searches) == 960 && report.TrainingRows == 64 && report.Fits == 2 && report.Compiler == r.Compiler && report.CorpusSHA == r.CorpusSHA && report.SDKBase == "0ac7eae98ebfde1a1a4ead0bd4e855ac54065cbc" && report.Producer == "46f3c1355d43c5ccca4398d3e32ec1b4d1494567", "fixed execution scope")
	result := Summary{Schema: "gooo/canonical-contract-learning-audit/v1", Sources: len(sources), Searches: len(searches), TrainingRows: 64,
		Comparisons: map[string]Comparison{}, Pairs: map[string]Pair{}, Latency: map[string]Latency{}, Groups: map[string]r.Group{}}
	byID := map[string]r.Source{}
	bySearch := map[string]r.Search{}
	latency := map[string][]int64{}
	for i, x := range sources {
		checkSource(x, r.Specs()[i])
		byID[x.Spec.ID] = x
	}
	checkTrainingAndModels(root, sources, report)
	checkAudits(sources, report)
	for _, row := range searches {
		x, ok := byID[row.ID]
		need(ok, "known search document")
		key := row.ID + "/" + row.Mode
		_, duplicate := bySearch[key]
		need(!duplicate, "search occurred once")
		need(row.Mode == "fixed" || row.Mode == "legacy" || row.Mode == "canonical", "known mode")
		checkSearch(x, row, report)
		bySearch[key] = row
		recount(result.Groups, x, row)
		result.Stages += len(row.Stages)
		for _, stage := range row.Stages {
			result.ModelCalls += stage.Ranking.Calls
		}
		if row.Mode != "fixed" {
			latency[row.Mode] = append(latency[row.Mode], row.Stages[0].Ranking.PredictNS)
		}
	}
	need(reflect.DeepEqual(result.Groups, report.Groups) && report.ModelCalls == result.ModelCalls && report.InitialSearches == 960 && report.FeedbackSearches == result.Stages-960, "all report metrics")
	for name, values := range latency {
		slices.Sort(values)
		middle := len(values) / 2
		median := values[middle]
		if len(values)%2 == 0 {
			median = values[middle-1] + (median-values[middle-1])/2
		}
		result.Latency[name] = Latency{len(values), median, values[(len(values)*95+99)/100-1]}
	}
	for _, x := range sources {
		for _, modes := range [][2]string{{"fixed", "legacy"}, {"fixed", "canonical"}, {"legacy", "canonical"}} {
			a, b := bySearch[x.Spec.ID+"/"+modes[0]], bySearch[x.Spec.ID+"/"+modes[1]]
			old, new := x.Candidates[a.Stages[0].Ranking.Proposed].ProbeAcceptable, x.Candidates[b.Stages[0].Ranking.Proposed].ProbeAcceptable
			key := modes[0] + "->" + modes[1] + "/" + x.Spec.Split
			c := result.Comparisons[key]
			if !old && new {
				c.Improved++
			} else if old && !new {
				c.Regressed++
				c.Regressions = append(c.Regressions, x.Spec.ID)
			} else {
				c.Unchanged++
			}
			result.Comparisons[key] = c
		}
		if x.Spec.Form != "assignment" {
			continue
		}
		directID := strings.Replace(x.Spec.ID, "-assignment-", "-direct-", 1)
		direct, ok := byID[directID]
		need(ok, "paired direct source")
		for _, mode := range []string{"legacy", "canonical"} {
			a, b := bySearch[directID+"/"+mode].Stages[0].Ranking, bySearch[x.Spec.ID+"/"+mode].Stages[0].Ranking
			key := mode + "/" + x.Spec.Split
			p := result.Pairs[key]
			p.Pairs++
			left, right := direct.LegacyInputs, x.LegacyInputs
			if mode == "canonical" {
				left, right = direct.CanonicalInputs, x.CanonicalInputs
			}
			if reflect.DeepEqual(left, right) {
				p.SameInput++
			}
			if a.Logits == b.Logits {
				p.SameScores++
			}
			if a.Proposed == b.Proposed {
				p.SameProposal++
			}
			result.Pairs[key] = p
		}
	}
	return result
}

func main() {
	if len(os.Args) != 2 {
		panic("usage: audit RETAINED_RESULT_DIRECTORY")
	}
	result := audit(os.Args[1])
	must(json.NewEncoder(os.Stdout).Encode(result))
}
