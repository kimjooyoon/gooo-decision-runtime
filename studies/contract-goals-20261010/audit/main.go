// Read-only audit of the fixed original records. No fitting, prediction, source
// lowering, path preparation, compilation or program execution is performed.
package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/contract-goals-20261010/record"
)

const producer = "addd45c2b2a5c9a76ab5029174cfe1b9189f2275"
const modelSHA = "c53ef0386dfc8ee2422b2b9605b609b0c23315905599ecc2f4555c26de514b5f"

func must(e error) {
	if e != nil {
		panic(e)
	}
}
func need(ok bool, s string) {
	if !ok {
		panic(s)
	}
}
func load(root, name string) []byte {
	b, e := os.ReadFile(filepath.Join(root, name))
	must(e)
	if filepath.Ext(name) == ".gz" {
		g, e := gzip.NewReader(bytes.NewReader(b))
		must(e)
		b, e = io.ReadAll(io.LimitReader(g, 32<<20))
		must(e)
		must(g.Close())
	}
	return b
}
func decode[T any](raw []byte) T { var v T; must(json.Unmarshal(raw, &v)); return v }
func lines[T any](raw []byte) []T {
	var rows []T
	s := bufio.NewScanner(bytes.NewReader(raw))
	s.Buffer(make([]byte, 4096), 2<<20)
	for s.Scan() {
		rows = append(rows, decode[T](s.Bytes()))
	}
	must(s.Err())
	return rows
}

// Independent arithmetic reference for each recorded complete mask. This does
// not inspect or execute the compiled program; it checks the fixed template's
// meaning against the recorded results using int64 throughout.
func actual(s r.Spec, mask uint16, x int64) (int64, bool) {
	condition := x < s.K
	if mask&1 != 0 {
		condition = s.K < x
	}
	first := condition
	if (s.Reverse == 1) != (mask&2 != 0) {
		first = !first
	}
	var a, b int64
	switch s.Family {
	case "bound":
		a, b = x, s.K
	case "distance":
		a, b = x-s.K, s.K-x
	case "offset":
		a, b = x+s.K, x-s.K
	case "tag":
		a, b = -s.K, s.K
	default:
		panic("unknown reference family")
	}
	if first {
		return a, condition
	}
	return b, condition
}

func checkSource(x r.Source, s r.Spec) {
	need(x.Spec == s && x.Gooo == r.Gooo(s) && x.SourceSHA == r.Hash([]byte(x.Gooo)), "fixed source identity")
	need(x.PlanSHA == r.Hash(r.Encode(x.Document.Plan)) && x.CaseSHA == r.Hash(r.Encode(x.Document.TestCases)), "source/case digest")
	need(reflect.DeepEqual(x.Document.TestCases, r.Cases(s)) && len(x.Candidates) == 4 && len(x.Inputs) == 2 && len(x.CaseRows) == len(x.Document.TestCases), "source/case scope")
	need(len(x.Document.Plan.ConditionCases) == 3, "source condition count")
	for i, input := range []int64{-9007199254740995, s.K, 18014398509481990} {
		need(x.Document.Plan.ConditionCases[i] == (pathplan.ConditionCase{ChoiceID: "comparison", Input: input, Expected: input < s.K}), "authored source condition")
	}
	for i, row := range x.Inputs {
		need(r.FeatureSHA(row) == x.InputSHA[i], "source feature digest")
	}
	for i, c := range x.Document.TestCases {
		var row [32]float32
		must(decision.DeclaredCaseFeaturesInto(c.Input, c.Expected, &row))
		need(row == x.CaseRows[i], "declared case encoding")
	}
	var acceptable uint64
	for mask, c := range x.Candidates {
		need(c.Mask == uint16(mask) && len(c.Outputs) == len(x.Document.TestCases) && len(c.Conditions) == 3, "oracle candidate scope")
		valid := true
		for i, o := range c.Outputs {
			test := x.Document.TestCases[i]
			v, _ := actual(s, c.Mask, test.Input)
			pass := v == test.Expected
			need(o.Input == test.Input && o.Expected == test.Expected && o.Actual == v && o.Passed == pass, "exact candidate output")
			valid = valid && pass
		}
		for i, c := range c.Conditions {
			want := x.Document.Plan.ConditionCases[i]
			v, condition := actual(s, uint16(mask), want.Input)
			pass := condition == want.Expected
			need(c.Case == want && c.Observation.Reached && c.Observation.Value == condition && c.Output.Int == v && c.Output.Type == decision.TypeInt && c.Passed == pass, "exact condition observation")
			status := "MISMATCH"
			if pass {
				status = "MATCH"
			}
			need(c.Status == status, "condition outcome")
			valid = valid && pass
		}
		need(valid == c.Acceptable, "candidate acceptable label")
		if valid {
			acceptable |= 1 << mask
		}
	}
	need(x.Acceptable == acceptable && (acceptable == 0) == (s.Split == "contradiction"), "complete acceptable set")
}

func checkSearch(row r.Search, x r.Source, fingerprint string) {
	need(row.ID == x.Spec.ID && row.Split == x.Spec.Split && row.Family == x.Spec.Family && (row.Mode == "model" || row.Mode == "deterministic") && row.NS > 0, "search source/mode")
	rank := row.Ranking
	saved := rank.SHA
	rank.SHA = ""
	need(saved == r.Hash(r.Encode(rank)), "ranking digest")
	need(rank.Schema == "gooo/contract-path-ranking/v1" && rank.PlanSHA == x.PlanSHA && rank.CaseSHA == x.CaseSHA && rank.CaseCount == len(x.Document.TestCases) && rank.ChoiceCount == 2 && !rank.Declined && rank.Error == "", "ranking source binding")
	calls := 0
	if row.Mode == "model" {
		calls = 1
		need(rank.Applied && rank.ModelFingerprint == fingerprint && rank.SourceFeatures == decision.RelationalFlowFeatureVersion && rank.CaseFeatures == decision.DeclaredCaseFeatureVersion && rank.PredictNS > 0, "model ABI")
		var mask uint16
		for i := range 2 {
			need(rank.FeatureSHA[i] == x.InputSHA[i], "ranking inputs")
			if rank.Logits[i][1] > rank.Logits[i][0] {
				mask |= 1 << i
			}
		}
		need(rank.Proposed == mask, "score argmax")
	} else {
		need(!rank.Applied && rank.ModelFingerprint == "" && rank.Proposed == 0 && rank.PredictNS == 0, "deterministic fallback")
	}
	need(rank.Calls == calls && len(row.Progress) >= 2 && len(row.Progress) <= 5, "actual prediction count")
	previous := ""
	seen := map[uint16]bool{}
	conditionsRejected := 0
	for index, p := range row.Progress {
		outer := p.SHA
		p.SHA = ""
		need(outer == r.Hash(r.Encode(p)) && p.RankingSHA == saved, "rank/progress binding")
		core := p.SessionProgress
		coreSHA := core.SHA
		core.SHA = ""
		need(coreSHA == r.Hash(r.Encode(core)) && core.PreviousSHA == previous, "progress chain")
		previous = coreSHA
		need(core.Sequence == index+1 && core.CaseSHA == x.CaseSHA && core.Selection.PlanSHA256 == x.PlanSHA && core.Selection.ModelCalls == calls && core.Selection.ExternalCalls == 0 && core.PredictionsThisAdvance == 0 && core.FeedbackPredictions == 0 && core.LatestFeedbackSHA == "", "search identity/calls")
		need(core.Attempted == index && core.Evaluated == index && core.TypeRejected == 0 && core.Cases == len(x.Document.TestCases) && core.Declared == 4, "search counters")
		if index == 0 {
			need(len(core.NewAttempts) == 0, "initial execution")
			continue
		}
		need(len(core.NewAttempts) == 1, "step size")
		a := core.NewAttempts[0]
		need(a.Mask < 4 && !seen[a.Mask], "candidate repeated")
		seen[a.Mask] = true
		if index == 1 {
			need(a.Mask == rank.Proposed, "first selection differs from ranking")
		}
		c := x.Candidates[a.Mask]
		need(reflect.DeepEqual(a.Results, c.Outputs) && reflect.DeepEqual(a.Conditions, c.Conditions) && a.GoooSHA == r.Hash([]byte(c.GoooSource)), "executed selection differs from frozen oracle")
		passed := 0
		for _, v := range c.Outputs {
			if v.Passed {
				passed++
			}
		}
		need(a.Passed == passed && a.Total == len(c.Outputs), "attempt completeness")
		for i, choice := range x.Document.Plan.Decisions {
			need(a.Choices[choice.ID] == choice.Options[a.Mask>>i&1].Label, "attempt choice mapping")
		}
		status := "EVALUATED"
		if !pathplan.ConditionsPassed(c.Conditions) {
			status = "CONDITION_REJECTED"
			conditionsRejected++
		}
		need(a.Status == status && core.ConditionRejected == conditionsRejected, "condition acceptance")
		need((core.Status == "TRAINING_COMPLETE") == c.Acceptable, "finite acceptance")
	}
	last := row.Progress[len(row.Progress)-1]
	need((last.Status == "TRAINING_COMPLETE") == (x.Acceptable != 0), "all-source completion")
	if x.Acceptable == 0 {
		need(last.Exhausted && last.Attempted == 4, "contradictory requirements accepted")
	}
	var selected uint16
	for i, c := range x.Document.Plan.Decisions {
		if last.Selection.Choices[c.ID] == c.Options[1].Label {
			selected |= 1 << i
		}
	}
	need(row.GoSource == x.Candidates[selected].GoSource && reflect.DeepEqual(last.BestCases, x.Candidates[selected].Outputs), "selected body/outcomes")
}

type Pair struct{ Pairs, BothValid, BothInvalid, SameProposal, DifferentScores int }
type Summary struct {
	Schema                                      string
	Sources, TrainingRows, Searches, ModelCalls int
	Improved, Regressed, Unchanged              int
	Regressions                                 []string
	Pairs                                       map[string]Pair
	Groups                                      map[string]r.Group
}

func audit(root string) (result Summary, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("%v", p)
		}
	}()
	report := decode[r.Report](load(root, "report.json"))
	need(report.Producer == producer && report.Compiler == r.Compiler && report.Sources == 196 && report.TrainingRows == 24 && report.Fits == 1 && report.Searches == 392 && report.ModelCalls == 196 && report.NativeExecutions == 0, "fixed run header")
	need(report.Parameters == 9746 && report.WeightBytes == 38984 && report.OracleCandidates == 784 && report.OracleCompileCalls == 1568 && report.OracleOutputEvaluations == 8192 && report.OracleConditionObservations == 2352, "bounded architecture/oracle scope")
	need(report.Options == (contractdecision.FitOptions{Epochs: 600, LearningRate: 0.3, L2: 0.0001, Seed: 17}) && report.TrainingNS > 0, "fixed fit")
	modelRaw := load(root, "model.json")
	need(r.Hash(modelRaw) == modelSHA && report.ModelSHA == modelSHA && len(modelRaw) == report.ArtifactBytes, "model bytes")
	model, e := contractdecision.Decode(modelRaw)
	must(e)
	need(model.Fingerprint() == report.Fingerprint, "model identity")
	history := decode[[]contractdecision.Epoch](load(root, "history.json"))
	need(len(history) == 600 && history[0].Loss == report.FirstLoss && history[599].Loss == report.LastLoss, "training loss history")
	for i, h := range history {
		need(h.Number == i+1 && h.Loss >= 0, "epoch sequence")
	}
	sourceRaw := load(root, "sources.jsonl.gz")
	need(r.Hash(sourceRaw) == report.RecordsSHA, "source file bytes")
	sources := lines[r.Source](sourceRaw)
	specs := r.Specs()
	need(len(sources) == len(specs), "fixed source count")
	byID := map[string]r.Source{}
	var expected []r.Training
	for i, x := range sources {
		checkSource(x, specs[i])
		byID[x.Spec.ID] = x
		if x.Spec.Split == "train" {
			expected = append(expected, r.Training{ID: x.Spec.ID, PlanSHA: x.PlanSHA, CaseSHA: x.CaseSHA, Inputs: x.Inputs, Cases: x.CaseRows, Masks: []uint16{0, 1, 2, 3}, Acceptable: x.Acceptable})
		}
	}
	trainingRaw := load(root, "training.json.gz")
	need(r.Hash(trainingRaw) == report.TrainingSHA && reflect.DeepEqual(decode[[]r.Training](trainingRaw), expected) && len(expected) == 24, "training-only inputs/labels")
	searches := lines[r.Search](load(root, "searches.jsonl.gz"))
	need(len(searches) == 392, "search count")
	result = Summary{Schema: "gooo/contract-goals-audit/v1", Sources: len(sources), TrainingRows: len(expected), Searches: len(searches), Pairs: map[string]Pair{}, Groups: map[string]r.Group{}}
	byMode := map[string]r.Search{}
	var times []int64
	for i, row := range searches {
		x, ok := byID[row.ID]
		need(ok && row.ID == specs[i/2].ID, "search order/identity")
		mode := "deterministic"
		if i%2 == 1 {
			mode = "model"
		}
		need(row.Mode == mode, "fixed timing order")
		checkSearch(row, x, report.Fingerprint)
		byMode[row.ID+"/"+row.Mode] = row
		result.ModelCalls += row.Ranking.Calls
		if row.Mode == "model" {
			times = append(times, row.Ranking.PredictNS)
		}
		first := row.Progress[1].NewAttempts[0]
		last := row.Progress[len(row.Progress)-1]
		for _, key := range []string{row.Mode + "/" + row.Split, row.Mode + "/family-" + row.Family} {
			g := result.Groups[key]
			g.Programs++
			g.Attempts += last.Attempted
			g.ModelCalls += row.Ranking.Calls
			g.NS += row.NS
			g.PredictNS += row.Ranking.PredictNS
			if x.Candidates[first.Mask].Acceptable {
				g.FirstValid++
			}
			if last.Status == "TRAINING_COMPLETE" {
				g.Complete++
			}
			g.FirstOutputTotal += first.Total
			for _, c := range first.Results {
				if c.Passed {
					g.FirstOutputPassed++
				}
			}
			g.FirstConditionTotal += len(first.Conditions)
			for _, c := range first.Conditions {
				if c.Passed {
					g.FirstConditionPassed++
				}
			}
			result.Groups[key] = g
		}
	}
	need(result.ModelCalls == 196 && reflect.DeepEqual(result.Groups, report.Groups), "group/count recount")
	slices.Sort(times)
	need(times[98] == report.PredictionMedianNS && times[186] == report.PredictionP95NS, "observed timing percentiles")
	for i, x := range sources {
		a, b := byMode[x.Spec.ID+"/deterministic"], byMode[x.Spec.ID+"/model"]
		old, new := x.Candidates[a.Ranking.Proposed].Acceptable, x.Candidates[b.Ranking.Proposed].Acceptable
		if new && !old {
			result.Improved++
		} else if old && !new {
			result.Regressed++
			result.Regressions = append(result.Regressions, x.Spec.ID)
		} else {
			result.Unchanged++
		}
		if i%2 == 0 {
			continue
		}
		first := sources[i-1]
		need(first.Spec.Pair == x.Spec.Pair && first.InputSHA == x.InputSHA && first.PlanSHA == x.PlanSHA && first.CaseSHA != x.CaseSHA, "paired initial source identity")
		if x.Acceptable == 0 {
			continue
		}
		p := byMode[first.Spec.ID+"/model"]
		g := result.Pairs[x.Spec.Split]
		g.Pairs++
		left := first.Candidates[p.Ranking.Proposed].Acceptable
		if left && new {
			g.BothValid++
		}
		if !left && !new {
			g.BothInvalid++
		}
		if p.Ranking.Proposed == b.Ranking.Proposed {
			g.SameProposal++
		}
		if p.Ranking.Logits != b.Ranking.Logits {
			g.DifferentScores++
		}
		result.Pairs[x.Spec.Split] = g
	}
	return result, nil
}
func main() {
	if len(os.Args) != 2 {
		panic("usage: audit RESULT_DIRECTORY")
	}
	result, err := audit(os.Args[1])
	must(err)
	must(json.NewEncoder(os.Stdout).Encode(result))
}
