// Read-only audit of the fixed original records. No fitting, prediction, source
// lowering, path preparation, compilation or program execution is performed.
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

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/ordered-requirement-learning-20261010/record"
)

const producer = "74a987d017aecc1bc3f72372c7bd864ffc42c895"

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
	value := x
	if s.Form == "rare" && x >= 0 {
		value = s.K
	}
	condition := value < s.K
	if mask>>s.Order&1 != 0 {
		condition = s.K < value
	}
	first := condition
	if (s.Reverse == 1) != (mask>>(1-s.Order)&1 != 0) {
		first = !first
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
	need(reflect.DeepEqual(x.Document.Plan.ConditionCases, r.Conditions(s)), "authored source conditions")
	need(len(x.ConditionRows) == len(x.Document.Plan.ConditionCases), "all declared condition rows")
	checkConditionRows(x)
	checkOrderedRows(x)
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
		need(c.Mask == uint16(mask) && len(c.Outputs) == len(x.Document.TestCases) && len(c.Conditions) == len(x.Document.Plan.ConditionCases), "oracle candidate scope")
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
	need(row.ID == x.Spec.ID && row.Split == x.Spec.Split && row.Family == x.Spec.Family && (row.Mode == "requirements" || row.Mode == "ordered" || row.Mode == "deterministic") && row.NS > 0, "search source/mode")
	rank := row.Ranking
	saved := rank.SHA
	rank.SHA = ""
	need(saved == r.Hash(r.Encode(rank)), "ranking digest")
	schema := "gooo/contract-path-ranking/v1"
	calls := 0
	if row.Mode != "deterministic" {
		schema = "gooo/requirement-path-ranking/v1"
		featureVersion := decision.RelationalFlowFeatureVersion
		hashes := x.InputSHA
		if row.Mode == "ordered" {
			schema = "gooo/ordered-requirement-path-ranking/v1"
			featureVersion = contractdecision.OrderedSourceFeatureVersion
			hashes = x.OrderedSHA
		}
		need(rank.ModelFingerprint == fingerprint && rank.SourceFeatures == featureVersion && rank.CaseFeatures == decision.DeclaredCaseFeatureVersion, "model ABI")
		need(rank.ConditionFeatures == decision.DeclaredConditionFeatureVersion && rank.ConditionCount == len(x.ConditionRows) && rank.ConditionFeatureSHA == conditionSHA(x.ConditionRows), "complete condition binding")
		if row.Mode == "ordered" && x.OrderedReason != "" {
			need(rank.Declined && !rank.Applied && rank.Error == x.OrderedReason && rank.Proposed == 0 && rank.PredictNS == 0, "unsupported model fallback")
			need(rank.FeatureSHA == [16]string{} && rank.Logits == [16][2]float32{}, "unsupported source has no fabricated scores")
		} else {
			calls = 1
			need(rank.Applied && !rank.Declined && rank.Error == "" && rank.PredictNS > 0, "successful prediction")
			var mask uint16
			for i := range 2 {
				need(rank.FeatureSHA[i] == hashes[i], "full source feature digest")
				if rank.Logits[i][1] > rank.Logits[i][0] {
					mask |= 1 << i
				}
			}
			need(rank.Proposed == mask, "score argmax")
		}
	} else {
		need(!rank.Applied && !rank.Declined && rank.Error == "" && rank.ModelFingerprint == "" && rank.Proposed == 0 && rank.PredictNS == 0, "deterministic selection")
		need(rank.ConditionFeatures == "" && rank.ConditionFeatureSHA == "" && rank.ConditionCount == 0, "deterministic model inputs absent")
	}
	need(rank.Schema == schema && rank.PlanSHA == x.PlanSHA && rank.CaseSHA == x.CaseSHA && rank.CaseCount == 8 && rank.ChoiceCount == 2, "ranking source binding")
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
		need(last.ConditionRejected == 4 && row.GoSource == "" && row.GoooBody == "" && len(last.BestCases) == 0, "condition-rejected body returned as usable")
		return
	}
	var selected uint16
	for i, c := range x.Document.Plan.Decisions {
		if last.Selection.Choices[c.ID] == c.Options[1].Label {
			selected |= 1 << i
		}
	}
	need(row.GoSource == x.Candidates[selected].GoSource && reflect.DeepEqual(last.BestCases, x.Candidates[selected].Outputs), "selected body/outcomes")
}
