// Read-only audit of retained arrays, scores, bodies and observation ledgers.
// This command never fits, predicts, compiles or evaluates a candidate body.
package main

import (
	"encoding/json"
	"math"
	"math/bits"
	"os"
	"reflect"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	relative "github.com/kimjooyoon/gooo-decision-runtime/studies/source-relative-learning-20261011/model"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/source-relative-learning-20261011/record"
)

func main() {
	require(len(os.Args) == 2, "need original result directory")
	root := os.Args[1]
	report := read[r.Report](root, "report.json")
	require(report.Schema == "gooo/source-relative-learning-study/v1" && report.Compiler == r.Compiler && report.SDKBase == r.SDKBase && report.CorpusSHA == r.CorpusSHA, "identities differ")
	require(report.Sources == 288 && report.TrainingRows == 64 && report.Fits == 2 && report.Searches == 1152 && report.ModelCalls == 576, "populations differ")
	require(report.Options == (contractdecision.FitOptions{Epochs: 2000, LearningRate: .3, L2: .0001, Seed: 37}), "fit recipe differs")
	start := read[map[string]string](root, "started.json")
	end := read[map[string]string](root, "completed.json")
	require(start["producer"] == report.Producer && end["producer"] == report.Producer && start["compiler"] == r.Compiler && start["sdk_base"] == r.SDKBase && start["corpus"] == r.CorpusSHA, "start/completion identities differ")
	for _, name := range []string{"baseline", "relative"} {
		raw := payload(root, "model-"+name+".json")
		var model interface {
			Fingerprint() string
			ArtifactSchema() string
		}
		var err error
		if name == "baseline" {
			model, err = contractdecision.DecodeCanonicalInteractionRequirementConditioned(raw)
		} else {
			model, err = relative.Decode(raw)
		}
		must(err)
		info := read[r.Model](root, "model-info-"+name+".json")
		require(info == report.Models[name] && info.SHA == r.Hash(raw) && info.Fingerprint == model.Fingerprint() && info.Schema == model.ArtifactSchema(), "model identity differs")
		require(info.Parameters == 19034 && info.WeightBytes == 76136 && info.ArtifactBytes == len(raw), "model dimensions differ")
		history := read[[]contractdecision.Epoch](root, "history-"+name+".json")
		require(len(history) == 2000 && history[0].Loss == info.FirstLoss && history[1999].Loss == info.LastLoss, "loss history differs")
		for _, epoch := range history {
			require(!math.IsNaN(epoch.Loss) && !math.IsInf(epoch.Loss, 0), "nonfinite loss")
		}
	}
	sources := lines[r.Source](root, "sources.jsonl")
	searches := lines[r.Search](root, "searches.jsonl")
	training := read[[]r.TrainingRecord](root, "training.json")
	require(len(sources) == 288 && len(searches) == 1152 && len(training) == 64, "retained populations differ")
	byID := map[string]r.Source{}
	bySpec := map[r.Spec]r.Source{}
	records := map[string]r.Search{}
	trainingSeen := map[string]bool{}
	specs := r.Specs()
	for i, s := range sources {
		_, exists := byID[s.Spec.ID]
		require(!exists && s.Spec == specs[i] && reflect.DeepEqual(s.Frozen, r.Freeze(specs[i])), "source identity differs")
		auditSource(s)
		byID[s.Spec.ID] = s
		key := s.Spec
		key.ID = ""
		bySpec[key] = s
	}
	for _, tr := range training {
		s, ok := byID[tr.ID]
		require(ok && s.Spec.Split == "train" && !trainingSeen[tr.ID], "training scope differs")
		trainingSeen[tr.ID] = true
		require(tr.PlanSHA == s.PlanSHA && tr.CasesSHA == s.CasesSHA && tr.Acceptable == s.Acceptable && bits.OnesCount64(tr.Acceptable) == 1, "training target differs")
		require(reflect.DeepEqual(tr.CanonicalInputs, s.CanonicalInputs) && reflect.DeepEqual(tr.RelativeInputs, s.RelativeInputs) && reflect.DeepEqual(tr.Outputs, s.OutputRows) && reflect.DeepEqual(tr.Conditions, s.ConditionRows), "training arrays differ")
	}
	groups := map[string]r.Group{}
	timings := map[string][]int64{}
	for _, row := range searches {
		s, ok := byID[row.ID]
		require(ok && row.Split == s.Spec.Split && row.Family == s.Spec.Family && row.Form == s.Spec.Form, "search identity differs")
		key := row.ID + "/" + row.Mode
		_, exists := records[key]
		require(!exists, "duplicate search")
		records[key] = row
		auditSearch(s, row, report)
		for _, p := range row.Predictions {
			timings[p.Model] = append(timings[p.Model], p.NS)
		}
		for _, field := range []struct {
			name string
			ns   int64
		}{{"preparation", row.PreparationNS}, {"construction", row.NS}, {"probe", row.ProbeNS}, {"rule", row.RuleNS}} {
			timings[row.Mode+"/"+field.name] = append(timings[row.Mode+"/"+field.name], field.ns)
		}
		aggregate(groups, s, row)
	}
	require(reflect.DeepEqual(groups, report.Groups), "group summary differs")
	changes, paired, equivalent := pairings(sources, records, bySpec)
	eventCount := checkEvents(lines[event](root, "events.jsonl"), records)
	names := []string{"sources.jsonl", "training.json", "searches.jsonl", "events.jsonl", "report.json", "started.json", "completed.json"}
	for _, name := range []string{"baseline", "relative"} {
		names = append(names, "model-"+name+".json", "model-info-"+name+".json", "history-"+name+".json", name+"-fit-started.json")
	}
	identities := map[string]string{}
	for _, name := range names {
		identities[name] = hash(root, name)
	}
	latencies := map[string]latency{}
	for name, ns := range timings {
		latencies[name] = distribution(ns)
	}
	out := map[string]any{"schema": "gooo/source-relative-learning-audit/v1", "source_count": len(sources), "search_count": len(searches), "fits": 2, "event_count": eventCount, "new_fits": 0, "new_predictions": 0, "new_candidate_executions": 0, "groups": groups, "compared_with_baseline": changes, "goal_pairs": paired, "equivalent_form_pairs": equivalent, "latencies": latencies, "files": identities, "scope": "retained-record validation; paired groups overlap; no independent oracle execution or causal attribution"}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	must(enc.Encode(out))
}
