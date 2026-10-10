package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/interaction-requirement-learning-20261011/record"
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
}

func checkModels(root string, report r.Report) {
	need(len(report.Models) == 2, "two model artifacts")
	for _, name := range []string{"ordered", "interaction"} {
		m := report.Models[name]
		raw := load(root, "model-"+name+".json")
		parameters := 17890
		wantSHA := "9f2901905b0cd663ea61d678a0993e6f6c2abee3aef7689b1d00faa91e3f857d"
		var fp, schema, pooling string
		if name == "ordered" {
			model, err := contractdecision.DecodeOrderedRequirementConditioned(raw)
			must(err)
			fp, schema, pooling = model.Fingerprint(), model.ArtifactSchema(), model.Pooling()
		} else {
			parameters = 19034
			wantSHA = "4b61fe8d84df77c8dfac6a880eedcddfd8f5f903fa538d1e80532dd77f74b1ac"
			model, err := contractdecision.DecodeInteractionRequirementConditioned(raw)
			must(err)
			fp, schema, pooling = model.Fingerprint(), model.ArtifactSchema(), model.Pooling()
		}
		need(r.Hash(raw) == wantSHA && m.SHA == wantSHA && m.ArtifactBytes == len(raw), "original model bytes")
		need(m.Parameters == parameters && m.WeightBytes == parameters*4 && m.TrainingNS > 0, "model dimensions/cost")
		need(m.Fingerprint == fp && m.Schema == schema && pooling == contractdecision.MeanPooling, "model computation identity")
		history := decode[[]contractdecision.Epoch](load(root, "history-"+name+".json"))
		need(len(history) == 2000 && history[0].Loss == m.FirstLoss && history[1999].Loss == m.LastLoss, "original fit history")
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
	for _, modes := range [][2]string{{"deterministic", "ordered"}, {"deterministic", "interaction"}, {"ordered", "interaction"}} {
		var c Comparison
		for _, x := range sources {
			a, b := byMode[x.Spec.ID+"/"+modes[0]], byMode[x.Spec.ID+"/"+modes[1]]
			old, new := x.Candidates[a.Ranking.Proposed].Acceptable, x.Candidates[b.Ranking.Proposed].Acceptable
			if new && !old {
				c.Improved++
			} else if old && !new {
				c.Regressed++
				c.Regressions = append(c.Regressions, x.Spec.ID)
			} else {
				c.Unchanged++
			}
		}
		result.Comparisons[modes[0]+"->"+modes[1]] = c
	}
	for i := 1; i < len(sources); i += 2 {
		a, b := sources[i-1], sources[i]
		need(a.Spec.Pair == b.Spec.Pair && a.InputSHA == b.InputSHA && a.CaseSHA == b.CaseSHA, "condition-only pair source/output identity")
		if b.Acceptable == 0 {
			continue
		}
		for _, mode := range []string{"deterministic", "ordered", "interaction"} {
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
}

func audit(root string) (result Summary, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("%v", p)
		}
	}()
	report := decode[r.Report](load(root, "report.json"))
	need(report.Schema == "gooo/interaction-requirement-learning-study/v1" && report.Producer == producer && report.Compiler == r.Compiler && report.CorpusSHA == r.CorpusSHA, "source revisions")
	need(report.Sources == 80 && report.TrainingRows == 8 && report.Fits == 2 && report.Searches == 240 && report.ModelCalls == 144 && report.NativeExecutions == 0, "fixed run scope")
	need(report.OracleCandidates == 320 && report.OracleCompileCalls == 640 && report.OracleOutputEvaluations == 2560 && report.OracleConditionObservations == 4960, "oracle scope")
	need(report.Options == (contractdecision.FitOptions{Epochs: 2000, LearningRate: .3, L2: .0001, Seed: 17}), "fixed fit options")
	start := decode[map[string]string](load(root, "started.json"))
	need(start["producer"] == producer && start["compiler"] == r.Compiler, "original producer receipt")
	done := decode[struct {
		CountSources int `json:"sources"`
		CountFits    int `json:"fits"`
		CountCalls   int `json:"model_calls"`
	}](load(root, "completed.json"))
	need(done.CountSources == 80 && done.CountFits == 2 && done.CountCalls == 144, "completion receipt")
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
	need(r.Hash(trainRaw) == report.TrainingSHA && len(expected) == 8 && reflect.DeepEqual(decode[[]r.Training](trainRaw), expected), "training-only labels")
	searches := lines[r.Search](load(root, "searches.jsonl.gz"))
	need(len(searches) == 240, "search count")
	result = Summary{Schema: "gooo/interaction-requirement-learning-audit/v1", Sources: 80, TrainingRows: 8, Searches: 240, Comparisons: map[string]Comparison{}, Pairs: map[string]Pair{}, Groups: map[string]r.Group{}}
	byMode := map[string]r.Search{}
	times := map[string][]int64{}
	for i, row := range searches {
		mode := []string{"deterministic", "ordered", "interaction"}[i%3]
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
	need(result.ModelCalls == 144 && reflect.DeepEqual(result.Groups, report.Groups), "complete metric recount")
	for name, values := range times {
		slices.Sort(values)
		m := report.Models[name]
		need(values[len(values)/2] == m.PredictionMedianNS && values[(len(values)*95+99)/100-1] == m.PredictionP95NS, "latency recount")
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
