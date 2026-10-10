package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/contract-goals-20261010/record"
)

type report struct {
	Schema        string                      `json:"schema"`
	CaseFeatures  string                      `json:"case_feature_version"`
	ProjectionSHA string                      `json:"projection_sha256"`
	ConsumedSHA   string                      `json:"consumed_training_sha256"`
	FitForwards   int                         `json:"fit_forward_passes"`
	Producer      string                      `json:"producer"`
	ModelSHA      string                      `json:"model_sha256"`
	Fingerprint   string                      `json:"fingerprint"`
	TrainingSHA   string                      `json:"training_sha256"`
	SourceSHA     string                      `json:"source_raw_sha256"`
	Options       contractdecision.FitOptions `json:"fit_options"`
	Fits          int                         `json:"fits"`
	Rows          int                         `json:"training_rows"`
	Calls         int                         `json:"model_calls"`
	Sources       int                         `json:"sources"`
	Searches      int                         `json:"searches"`
	Native        int                         `json:"native_processes"`
	Feedback      int                         `json:"post_failure_calls"`
	Parameters    int                         `json:"parameters"`
	Bytes         int                         `json:"weight_bytes"`
	ArtifactBytes int                         `json:"artifact_bytes"`
	FitNS         int64                       `json:"training_ns"`
	MedianNS      int64                       `json:"prediction_median_ns"`
	P95NS         int64                       `json:"prediction_p95_ns"`
	FirstLoss     float64                     `json:"first_loss"`
	LastLoss      float64                     `json:"last_loss"`
}

type Pair struct{ Count, BothValid, BothInvalid, SameProposal, DifferentScores int }
type Summary struct {
	Schema                         string
	Improved, Regressed, Unchanged int
	Regressions                    []string
	Improvements                   []string
	Groups                         map[string]r.Group
	Pairs                          map[string]Pair
}

func firstValid(row r.Search, source r.Source) bool {
	return source.Acceptable>>row.Ranking.Proposed&1 != 0
}

func add(groups map[string]r.Group, mode string, row r.Search, source r.Source) {
	last := row.Progress[len(row.Progress)-1]
	first := row.Progress[1].NewAttempts[0]
	for _, name := range []string{"all", source.Spec.Split} {
		key := mode + "/" + name
		g := groups[key]
		g.Programs++
		if firstValid(row, source) {
			g.FirstValid++
		}
		if last.Status == "TRAINING_COMPLETE" {
			g.Complete++
		}
		g.Attempts += last.Attempted
		g.ModelCalls += row.Ranking.Calls
		g.NS += row.NS
		g.PredictNS += row.Ranking.PredictNS
		g.FirstOutputTotal += first.Total
		for _, output := range first.Results {
			if output.Passed {
				g.FirstOutputPassed++
			}
		}
		g.FirstConditionTotal += len(first.Conditions)
		for _, condition := range first.Conditions {
			if condition.Passed {
				g.FirstConditionPassed++
			}
		}
		groups[key] = g
	}
}

func readPinned(root, name, hash string) []byte {
	raw, err := os.ReadFile(filepath.Join(root, name))
	must(err)
	need(r.Hash(raw) == hash, "frozen input hash "+name)
	return load(root, name)
}

func audit(root, frozen, baseline string) (result Summary, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("%v", p)
		}
	}()
	rep := decode[report](load(root, "report.json"))
	need(rep.Schema == "gooo/source-literal-contract-study/v1" && rep.FitForwards == 14400 && rep.CaseFeatures == decision.SourceLiteralCaseFeatureVersion, "fixed source-literal objective")
	need(rep.Producer == producer && rep.ModelSHA == modelSHA && rep.Fits == 1 && rep.Rows == 24 && rep.Calls == 196 && rep.Sources == 196 && rep.Searches == 196 && rep.Native == 0 && rep.Feedback == 0, "one fixed original run")
	need(rep.Parameters == 9746 && rep.Bytes == 38984 && rep.FitNS > 0 && rep.Options == (contractdecision.FitOptions{Epochs: 600, LearningRate: .3, L2: .0001, Seed: 17}), "fixed fit and shape")
	raw := load(root, "model.json")
	need(r.Hash(raw) == modelSHA && len(raw) == rep.ArtifactBytes, "new model bytes")
	m, e := contractdecision.Decode(raw)
	must(e)
	need(m.ArtifactSchema() == contractdecision.SourceLiteralSchema && m.CaseFeatureVersion() == rep.CaseFeatures && m.Pooling() == contractdecision.ExtremePooling && m.Fingerprint() == rep.Fingerprint, "new computation")
	history := decode[[]contractdecision.Epoch](load(root, "history.json.gz"))
	need(len(history) == 600 && history[0].Loss == rep.FirstLoss && history[599].Loss == rep.LastLoss, "all losses")
	for i, h := range history {
		need(h.Number == i+1 && h.Loss >= 0, "epoch sequence")
	}
	sourceRaw := readPinned(frozen, "sources.jsonl.gz", "773db4e279b497465a92ad2d7df683f0a2898d72365f6f48ac351e5f7036b413")
	need(r.Hash(sourceRaw) == rep.SourceSHA, "source raw digest")
	sources := lines[r.Source](sourceRaw)
	need(len(sources) == 196, "source count")
	specs := r.Specs()
	byID := map[string]r.Source{}
	var expected []r.Training
	for i, source := range sources {
		checkSource(source, specs[i])
		byID[source.Spec.ID] = source
		if source.Spec.Split == "train" {
			expected = append(expected, r.Training{ID: source.Spec.ID, PlanSHA: source.PlanSHA, CaseSHA: source.CaseSHA, Inputs: source.Inputs, Cases: source.CaseRows, Masks: []uint16{0, 1, 2, 3}, Acceptable: source.Acceptable})
		}
	}
	training := load(root, "training.json.gz")
	frozenTraining := readPinned(frozen, "training.json.gz", "e8e07ad2cbc8ff5adac35b81999000717baef4e0143c93c204dc483cbdcf7d1d")
	need(r.Hash(training) == rep.TrainingSHA && reflect.DeepEqual(training, frozenTraining) && reflect.DeepEqual(expected, decode[[]r.Training](training)), "only original training members/arrays/labels")
	checkProjectionTraining(root, sources, expected, rep)
	oldRows := lines[r.Search](readPinned(baseline, "searches.jsonl.gz", "ccdf0b3312c123d7b4c3bc804e9443d43489f38ac024578af7bbb85f25e2b827"))
	readPinned(baseline, "model.json", "f69944394dbe66b3449c5f87cb5ee51b09a9f34c4a2483e5fc119a005160e8b7")
	newRows := lines[r.Search](load(root, "searches.jsonl.gz"))
	need(len(oldRows) == 196 && len(newRows) == 196, "complete original/new records")
	result = compare(byID, oldRows, newRows, rep.Fingerprint)
	var times []int64
	for _, row := range newRows {
		times = append(times, row.Ranking.PredictNS)
	}
	slices.Sort(times)
	need(times[98] == rep.MedianNS && times[186] == rep.P95NS, "recorded latency quantiles")
	return result, nil
}

func compare(sources map[string]r.Source, oldRows, newRows []r.Search, fingerprint string) Summary {
	result := Summary{Schema: "gooo/source-literal-contract-audit/v1", Groups: map[string]r.Group{}, Pairs: map[string]Pair{}}
	old := map[string]r.Search{}
	paired := map[string][]r.Search{}
	for _, row := range oldRows {
		source, ok := sources[row.ID]
		need(ok, "baseline source")
		checkSearch(row, source, "32de9753de5637c3575b188261c80e510212f17194d0710b040e885fc05d0f8d")
		add(result.Groups, "saved_"+row.Mode, row, source)
		if row.Mode == "signed_max_abs" {
			need(old[row.ID].ID == "", "duplicate saved model source")
			old[row.ID] = row
			if source.Acceptable != 0 {
				paired["unpaired/"+source.Spec.Pair] = append(paired["unpaired/"+source.Spec.Pair], row)
			}
		}
	}
	seen := map[string]bool{}
	for _, row := range newRows {
		source, ok := sources[row.ID]
		need(ok && !seen[row.ID] && row.Mode == "source_literals", "new source/mode")
		seen[row.ID] = true
		checkSearch(row, source, fingerprint)
		add(result.Groups, "source_literals", row, source)
		before, after := firstValid(old[row.ID], source), firstValid(row, source)
		switch {
		case !before && after:
			result.Improved++
			result.Improvements = append(result.Improvements, row.ID)
		case before && !after:
			result.Regressed++
			result.Regressions = append(result.Regressions, row.ID)
		default:
			result.Unchanged++
		}
		if source.Acceptable != 0 {
			paired["source_literals/"+source.Spec.Pair] = append(paired["source_literals/"+source.Spec.Pair], row)
		}
	}
	for _, rows := range paired {
		addPair(&result, rows, sources)
	}
	return result
}

func addPair(result *Summary, rows []r.Search, sources map[string]r.Source) {
	need(len(rows) == 2, "goal pair")
	a, b := sources[rows[0].ID], sources[rows[1].ID]
	need(a.Spec.Pair == b.Spec.Pair && a.Spec.Goal != b.Spec.Goal && reflect.DeepEqual(a.Inputs, b.Inputs), "same-source opposite goals")
	mode := "unpaired"
	if rows[0].Mode == "source_literals" {
		mode = "source_literals"
	}
	for _, split := range []string{"all", a.Spec.Split} {
		key := mode + "/" + split
		p := result.Pairs[key]
		p.Count++
		if firstValid(rows[0], a) && firstValid(rows[1], b) {
			p.BothValid++
		}
		if !firstValid(rows[0], a) && !firstValid(rows[1], b) {
			p.BothInvalid++
		}
		if rows[0].Ranking.Proposed == rows[1].Ranking.Proposed {
			p.SameProposal++
		}
		if rows[0].Ranking.Logits != rows[1].Ranking.Logits {
			p.DifferentScores++
		}
		result.Pairs[key] = p
	}
}

func main() {
	need(len(os.Args) == 4, "usage: audit RESULT FROZEN_SOURCES SIGNED_BASELINE")
	result, err := audit(os.Args[1], os.Args[2], os.Args[3])
	must(err)
	must(json.NewEncoder(os.Stdout).Encode(result))
}
