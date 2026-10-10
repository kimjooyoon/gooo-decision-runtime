package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/flowdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

type source struct {
	ID, Group, Form, Split, Source, SourceSHA, PlanSHA string
	Document                                           pathplan.Document
	V5                                                 [][384]float32
	V5SHA                                              string
	Contexts                                           []pathplan.SemanticBranchContext
	Acceptable                                         uint64
}
type previous struct {
	ID, Model  string
	Prediction flowdecision.Prediction
}
type record struct {
	ID, Split, Model, ModelSHA, Fingerprint, FeatureVersion, SourceSHA, PlanSHA, InputSHA string
	Acceptable                                                                            uint64
	ChoiceIDs, Intents                                                                    []string
	Inputs                                                                                [][384]float32
	Prediction                                                                            flowdecision.Prediction
	Explanation                                                                           flowdecision.Explanation
	Valid                                                                                 bool
	ActiveHidden                                                                          [2]int
}
type pair struct {
	Model, MaxID, MinID                                              string
	DifferentInputs, OnlyBranchIntentDiffers, DisjointAcceptableSets bool
	SameBranchHidden, SameBranchScores, SamePrediction               bool
	MaxActive, MinActive                                             int
}
type modelSpec struct{ Name, Path, SHA, Features, PredictionSHA string }

var specs = []modelSpec{
	{"v5", "semantic-flow-learning-20261010", "8127de6776d13b9c06710d1da762bb254401c1d8ff7e26ee6bb3c2d50d5cb8db", decision.SemanticFlowFeatureVersion, "c62ad8c7920519d335255e1f3e424fdf1a6084d1856ad72ae6878d8754c6ce5c"},
	{"v6", "relational-flow-learning-20261010", "3ba2c75cd7a3dd398e445bdd31abaaa3b3337e6261beac9e88faba4ea12c9c3b", decision.RelationalFlowFeatureVersion, "f585f2514054113447931339b3d6e411382919303bd475c3588ee27983b0f333"},
}

func sources(root string) []source {
	raw, sha := gzipBytes(filepath.Join(root, "semantic-flow-normalization-20261010/result/records.jsonl.gz"))
	require(sha == "05fa80975f0e6ea56e0e653fc1207d5894805957cd7a4a54091b67d0e1a7ed40", "original source records")
	all := decodeLines[source](raw)
	require(len(all) == 162, "complete frozen dataset")
	var result []source
	for _, s := range all {
		if s.Form != "direct" {
			continue
		}
		require(hash([]byte(s.Source)) == s.SourceSHA && len(s.V5) == 2 && featureSHA(s.V5, 384) == s.V5SHA, "source and input identity")
		require(len(s.Document.Plan.Decisions) == 2 && s.Document.Plan.Decisions[0].ID == "comparison" && s.Document.Plan.Decisions[1].ID == "branches", "choice identity")
		result = append(result, s)
	}
	require(len(result) == 32, "all32 direct forms")
	return result
}
func input(s source, name string) [][384]float32 {
	f := slices.Clone(s.V5)
	if name == "v6" {
		require(len(s.Contexts) == 2, "source contexts")
		for i, view := range s.Contexts {
			require(view.Normalized, "eligible direct source")
			must(decision.RelationalFlowFeaturesInto(s.V5[i], view.Flow, &f[i]))
		}
	}
	return f
}
func oldPredictions(root string, spec modelSpec) map[string]flowdecision.Prediction {
	raw, digest := gzipBytes(filepath.Join(root, spec.Path, "result/judgments.jsonl.gz"))
	require(digest == spec.PredictionSHA, "immutable original prediction records")
	result := map[string]flowdecision.Prediction{}
	for _, p := range decodeLines[previous](raw) {
		if p.Model == spec.Name {
			result[p.ID] = p.Prediction
		}
	}
	require(len(result) == 162, "complete previous prediction collection")
	return result
}
func explain(s source, spec modelSpec, m *flowdecision.Model, old flowdecision.Prediction) record {
	r := record{ID: s.ID, Split: s.Split, Model: spec.Name, ModelSHA: spec.SHA, Fingerprint: m.Fingerprint(), FeatureVersion: m.FeatureVersion(), SourceSHA: s.SourceSHA, PlanSHA: s.PlanSHA, Acceptable: s.Acceptable, Inputs: input(s, spec.Name)}
	r.InputSHA = featureSHA(r.Inputs, 384)
	for _, c := range s.Document.Plan.Decisions {
		r.ChoiceIDs = append(r.ChoiceIDs, c.ID)
		r.Intents = append(r.Intents, c.Intent)
	}
	var workspace flowdecision.Workspace
	must(m.ExplainInto(r.Inputs, []uint16{0, 1, 2, 3}, &workspace, &r.Prediction, &r.Explanation))
	require(r.Prediction == old, "diagnostic pass retains original prediction")
	r.Valid = s.Acceptable>>r.Prediction.Selected&1 != 0
	for choice := range 2 {
		for _, h := range r.Explanation.Hidden[choice] {
			if h > 0 {
				r.ActiveHidden[choice]++
			}
		}
	}
	return r
}
func compare(maximum, minimum record) pair {
	p := pair{Model: maximum.Model, MaxID: maximum.ID, MinID: minimum.ID, OnlyBranchIntentDiffers: true,
		DisjointAcceptableSets: maximum.Acceptable&minimum.Acceptable == 0,
		SameBranchHidden:       maximum.Explanation.Hidden[1] == minimum.Explanation.Hidden[1],
		SameBranchScores:       maximum.Explanation.OptionScores[1] == minimum.Explanation.OptionScores[1],
		SamePrediction:         maximum.Prediction == minimum.Prediction,
		MaxActive:              maximum.ActiveHidden[1], MinActive: minimum.ActiveHidden[1]}
	for c := range 2 {
		for i, v := range maximum.Inputs[c] {
			if v == minimum.Inputs[c][i] {
				continue
			}
			p.DifferentInputs = true
			if c != 1 || i < 64 || i >= 192 {
				p.OnlyBranchIntentDiffers = false
			}
		}
	}
	return p
}
func main() {
	require(len(os.Args) == 3, "usage: intent-selection-diagnostics STUDIES_DIRECTORY NEW_OUTPUT_DIRECTORY")
	revision := producer()
	root, out := os.Args[1], os.Args[2]
	rows := sources(root)
	must(os.Mkdir(out, 0700))
	save(out, "started.json", map[string]string{"producer": revision, "time": time.Now().UTC().Format(time.RFC3339)})
	f := rowFile(out, "records.jsonl")
	pairs := []pair{}
	counts := map[string]int{}
	calls := 0
	for _, spec := range specs {
		raw, err := os.ReadFile(filepath.Join(root, spec.Path, "result/model-"+spec.Name+".json"))
		must(err)
		require(hash(raw) == spec.SHA, "immutable model bytes")
		m, err := flowdecision.Decode(raw)
		must(err)
		require(m.FeatureVersion() == spec.Features, "model feature contract")
		old := oldPredictions(root, spec)
		byID := map[string]record{}
		for _, s := range rows {
			r := explain(s, spec, m, old[s.ID])
			calls++
			byID[s.ID] = r
			appendRow(f, r)
			kind, _, _ := strings.Cut(s.ID, "-")
			counts[spec.Name+"/"+kind+"/rows"]++
			if r.Valid {
				counts[spec.Name+"/"+kind+"/valid"]++
			}
			if r.ActiveHidden[1] == 0 {
				counts[spec.Name+"/"+kind+"/all_branch_hidden_zero"]++
			}
		}
		for _, s := range rows {
			if !strings.HasPrefix(s.ID, "max-") {
				continue
			}
			other := "min-" + strings.TrimPrefix(s.ID, "max-")
			p := compare(byID[s.ID], byID[other])
			require(p.DifferentInputs && p.OnlyBranchIntentDiffers && p.DisjointAcceptableSets, "opposite source-intent pairs")
			pairs = append(pairs, p)
		}
	}
	must(f.Close())
	require(calls == 64 && len(pairs) == 32, "fixed diagnostics")
	save(out, "pairs.json", pairs)
	save(out, "report.json", map[string]any{"schema": "gooo/intent-selection-diagnostics/v1", "producer": revision, "diagnostic_forward_calls": calls, "new_fits": 0, "new_candidate_executions": 0, "direct_sources": 32, "pairs": 32, "counts": counts})
	save(out, "completed.json", map[string]any{"diagnostic_forward_calls": calls, "time": time.Now().UTC().Format(time.RFC3339)})
	must(json.NewEncoder(os.Stdout).Encode(counts))
}
