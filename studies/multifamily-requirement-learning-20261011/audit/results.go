package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/multifamily-requirement-learning-20261011/record"
)

type Pair struct{ Pairs, BothValid, BothInvalid, SameProposal, DifferentScores int }
type Comparison struct {
	Improved, Regressed, Unchanged int
	Regressions                    []string
}
type Summary struct {
	Schema                                      string
	Sources, TrainingRows, Searches, ModelCalls int
	Comparisons                                 map[string]Comparison
	Pairs                                       map[string]Pair
	Groups                                      map[string]r.Group
	InputCollisions                             map[string]*contractdecision.InputAudit
	Interactions                                []InteractionRow
	DecisionDiagnostics                         map[string]DecisionDiagnostic
	Presentations                               map[string]Presentation
}

type Presentation struct{ Pairs, SameSource, SameScores, SameProposal, BothValid, BothInvalid int }

func cohort(s r.Spec) string {
	if s.Split == "train" {
		return "train"
	}
	if strings.HasPrefix(s.Split, "unseen_family_") {
		return "family_evaluation"
	}
	return "in_family_evaluation"
}

func checkModels(root string, report r.Report) {
	need(len(report.Models) == 2, "two model artifacts")
	for _, name := range []string{"released", "multifamily"} {
		m := report.Models[name]
		raw := load(root, "model-multifamily.json")
		wantSHA := "871a829633e8763d86091d384816f4452353fa96acc10cfa4e57d187299b8d5d"
		if name == "released" {
			raw = load(filepath.Join(root, "..", "..", "interaction-requirement-learning-20261011", "result"), "model-interaction.json")
			wantSHA = r.ReleasedModelSHA
		}
		model, err := contractdecision.DecodeInteractionRequirementConditioned(raw)
		must(err)
		need(r.Hash(raw) == wantSHA && m.SHA == wantSHA && m.ArtifactBytes == len(raw), "original model bytes")
		need(m.Parameters == 19034 && m.WeightBytes == 76136, "model dimensions")
		need(m.Fingerprint == model.Fingerprint() && m.Schema == model.ArtifactSchema() && model.Pooling() == contractdecision.MeanPooling, "model computation identity")
		if name == "released" {
			need(!m.FitInThisStudy && m.TrainingNS == 0 && m.FirstLoss == nil && m.LastLoss == nil, "comparator has no new fit")
			continue
		}
		need(m.FitInThisStudy && m.TrainingNS > 0 && m.FirstLoss != nil && m.LastLoss != nil, "new fit cost")
		history := decode[[]contractdecision.Epoch](load(root, "history-multifamily.json"))
		need(len(history) == 2000 && history[0].Loss == *m.FirstLoss && history[1999].Loss == *m.LastLoss, "original fit history")
		for i, h := range history {
			need(h.Number == i+1 && h.Loss >= 0, "epoch sequence")
		}
	}
}

func recount(groups map[string]r.Group, row r.Search, x r.Source) {
	first := row.Progress[1].NewAttempts[0]
	last := row.Progress[len(row.Progress)-1]
	keys := []string{row.Mode + "/all", row.Mode + "/" + row.Split, row.Mode + "/family-" + row.Family}
	if row.Split != "train" && row.Split != "contradiction" {
		keys = append(keys, row.Mode+"/heldout_satisfiable")
	}
	if strings.HasPrefix(row.Split, "unseen_family_") {
		keys = append(keys, row.Mode+"/family_evaluation")
	} else if strings.HasPrefix(row.Split, "heldout_") {
		keys = append(keys, row.Mode+"/in_family_evaluation")
	}
	for _, key := range keys {

		g := groups[key]
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
		g.FirstOutputTotal += len(first.Results)
		for _, v := range first.Results {
			if v.Passed {
				g.FirstOutputPassed++
			}
		}
		g.FirstConditionTotal += len(first.Conditions)
		for _, v := range first.Conditions {
			if v.Passed {
				g.FirstConditionPassed++
			}
		}
		groups[key] = g
	}
}

func compare(result *Summary, sources []r.Source, byMode map[string]r.Search) {
	for _, modes := range [][2]string{{"deterministic", "released"}, {"deterministic", "multifamily"}, {"released", "multifamily"}} {
		for _, x := range sources {
			a, b := byMode[x.Spec.ID+"/"+modes[0]], byMode[x.Spec.ID+"/"+modes[1]]
			old, new := x.Candidates[a.Ranking.Proposed].Acceptable, x.Candidates[b.Ranking.Proposed].Acceptable
			for _, key := range []string{modes[0] + "->" + modes[1], modes[0] + "->" + modes[1] + "/" + cohort(x.Spec)} {
				c := result.Comparisons[key]
				if new && !old {
					c.Improved++
				} else if old && !new {
					c.Regressed++
					c.Regressions = append(c.Regressions, x.Spec.ID)
				} else {
					c.Unchanged++
				}
				result.Comparisons[key] = c
			}
		}
	}
	for i := 1; i < len(sources); i += 2 {
		a, b := sources[i-1], sources[i]
		need(a.Spec.Pair == b.Spec.Pair && a.InputSHA == b.InputSHA && a.CaseSHA == b.CaseSHA, "condition-only pair source/output identity")
		if b.Acceptable == 0 {
			continue
		}
		for _, mode := range []string{"deterministic", "released", "multifamily"} {
			left, right := byMode[a.Spec.ID+"/"+mode], byMode[b.Spec.ID+"/"+mode]
			lv, rv := a.Candidates[left.Ranking.Proposed].Acceptable, b.Candidates[right.Ranking.Proposed].Acceptable
			for _, key := range []string{mode + "/all", mode + "/" + b.Spec.Split} {
				p := result.Pairs[key]
				p.Pairs++
				if lv && rv {
					p.BothValid++
				}
				if !lv && !rv {
					p.BothInvalid++
				}
				if left.Ranking.Proposed == right.Ranking.Proposed {
					p.SameProposal++
				}
				if left.Ranking.Logits != right.Ranking.Logits {
					p.DifferentScores++
				}
				result.Pairs[key] = p
			}
		}
	}
	result.Presentations = map[string]Presentation{}
	for i, x := range sources {
		for _, y := range sources[i+1:] {
			a, b := x.Spec, y.Spec
			if a.Family != b.Family || a.K != b.K || a.Reverse != b.Reverse || a.OutputGoal != b.OutputGoal || a.ConditionGoal != b.ConditionGoal || cohort(a) != cohort(b) {
				continue
			}
			kind := ""
			if a.Form == b.Form && a.Wording != b.Wording {
				kind = "wording"
			}
			if a.Form != b.Form && a.Wording == b.Wording {
				kind = "form"
			}
			if kind == "" {
				continue
			}
			for _, mode := range []string{"deterministic", "released", "multifamily"} {
				left, right := byMode[a.ID+"/"+mode], byMode[b.ID+"/"+mode]
				key := mode + "/" + cohort(a) + "/" + kind
				p := result.Presentations[key]
				p.Pairs++
				if x.OrderedSHA == y.OrderedSHA {
					p.SameSource++
				}
				if left.Ranking.Logits == right.Ranking.Logits {
					p.SameScores++
				}
				if left.Ranking.Proposed == right.Ranking.Proposed {
					p.SameProposal++
				}
				lv, rv := x.Candidates[left.Ranking.Proposed].Acceptable, y.Candidates[right.Ranking.Proposed].Acceptable
				if lv && rv {
					p.BothValid++
				}
				if !lv && !rv {
					p.BothInvalid++
				}
				result.Presentations[key] = p
			}
		}
	}
}

func audit(root string) (result Summary, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("%v", p)
		}
	}()
	report := decode[r.Report](load(root, "report.json"))
	need(report.Schema == "gooo/multifamily-requirement-learning-study/v1" && report.Producer == producer && report.Compiler == r.Compiler && report.CorpusSHA == r.CorpusSHA, "source revisions")
	need(report.Sources == 256 && report.TrainingRows == 64 && report.Fits == 1 && report.Searches == 768 && report.ModelCalls == 512 && report.NativeExecutions == 0, "fixed run scope")
	need(report.OracleCandidates == 1024 && report.OracleCompileCalls == 2048 && report.OracleOutputEvaluations == 8192 && report.OracleConditionObservations == 3072, "oracle scope")
	need(report.Options == (contractdecision.FitOptions{Epochs: 2000, LearningRate: .3, L2: .0001, Seed: 23}), "fixed fit options")
	start := decode[map[string]string](load(root, "started.json"))
	need(start["producer"] == producer && start["compiler"] == r.Compiler, "original producer receipt")
	done := decode[struct {
		CountSources int `json:"sources"`
		CountFits    int `json:"fits"`
		CountCalls   int `json:"model_calls"`
	}](load(root, "completed.json"))
	need(done.CountSources == 256 && done.CountFits == 1 && done.CountCalls == 512, "completion receipt")
	checkModels(root, report)
	corpusPath := filepath.Join(root, "..", "corpus.jsonl.gz")
	compressed, e := os.ReadFile(corpusPath)
	must(e)
	need(r.Hash(compressed) == r.CorpusSHA, "frozen corpus digest")
	frozen := lines[r.FrozenSource](load(filepath.Dir(corpusPath), filepath.Base(corpusPath)))
	specs := r.Specs()
	need(len(frozen) == len(specs), "corpus count")
	sourceRaw := load(root, "sources.jsonl.gz")
	need(r.Hash(sourceRaw) == report.RecordsSHA, "original source bytes")
	sources := lines[r.Source](sourceRaw)
	need(len(sources) == len(specs), "source count")
	byID := map[string]r.Source{}
	var expected []r.Training
	for i, x := range sources {
		need(frozen[i].Spec == specs[i] && frozen[i].Gooo == x.Gooo, "frozen source text")
		checkSource(x, specs[i])
		need(byID[x.Spec.ID].Spec.ID == "", "unique ID")
		byID[x.Spec.ID] = x
		if x.Spec.Split == "train" {
			expected = append(expected, r.Training{ID: x.Spec.ID, PlanSHA: x.PlanSHA, CaseSHA: x.CaseSHA, Inputs: x.Inputs, OrderedInputs: x.OrderedInputs, Cases: x.CaseRows, Conditions: x.ConditionRows, Masks: []uint16{0, 1, 2, 3}, Acceptable: x.Acceptable})
		}
	}
	checkInputAudits(sources, report)
	trainRaw := load(root, "training.json.gz")
	need(r.Hash(trainRaw) == report.TrainingSHA && len(expected) == 64 && reflect.DeepEqual(decode[[]r.Training](trainRaw), expected), "training-only labels")
	fit := decode[struct {
		Rows    int                         `json:"training_rows"`
		SHA     string                      `json:"training_sha"`
		Options contractdecision.FitOptions `json:"options"`
	}](load(root, "fit-started.json"))
	need(fit.Rows == 64 && fit.SHA == report.TrainingSHA && fit.Options == report.Options, "fixed original fit boundary")
	searches := lines[r.Search](load(root, "searches.jsonl.gz"))
	need(len(searches) == 768, "search count")
	result = Summary{Schema: "gooo/multifamily-requirement-learning-audit/v1", Sources: 256, TrainingRows: 64, Searches: 768, Comparisons: map[string]Comparison{}, Pairs: map[string]Pair{}, Groups: map[string]r.Group{}}
	byMode := map[string]r.Search{}
	times := map[string][]int64{}
	for i, row := range searches {
		mode := []string{"deterministic", "released", "multifamily"}[i%3]
		need(row.ID == specs[i/3].ID && row.Mode == mode, "fixed search order")
		x := byID[row.ID]
		checkSearch(row, x, report.Models[mode].Fingerprint)
		byMode[row.ID+"/"+mode] = row
		result.ModelCalls += row.Ranking.Calls
		if row.Ranking.Calls != 0 {
			times[mode] = append(times[mode], row.Ranking.PredictNS)
		}
		recount(result.Groups, row, x)
	}
	need(result.ModelCalls == 512 && reflect.DeepEqual(result.Groups, report.Groups), "complete metric recount")
	for name, values := range times {
		slices.Sort(values)
		m := report.Models[name]
		need((values[(len(values)-1)/2]+values[len(values)/2])/2 == m.PredictionMedianNS && values[(len(values)*95+99)/100-1] == m.PredictionP95NS, "latency recount")
	}
	compare(&result, sources, byMode)
	result.InputCollisions = report.InputAudits
	result.Interactions, result.DecisionDiagnostics = diagnose(sources, byMode)
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
