package main

import (
	"math"
	"math/bits"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-pooling-20261011/record"
)

func validateTraining(root string, report r.Report, sources []r.Source) {
	var expected []r.TrainingRecord
	for _, source := range sources {
		if source.Spec.Split == "train" {
			expected = append(expected, r.TrainingRecord{ID: source.Spec.ID, PlanSHA: source.PlanSHA, CasesSHA: source.CasesSHA,
				CanonicalInputs: source.CanonicalInputs, Outputs: source.OutputRows, Conditions: source.ConditionRows, Acceptable: source.Acceptable})
		}
	}
	training := decode[[]r.TrainingRecord](inflated(filepath.Join(root, "training.json.gz")))
	require(len(training) == 64 && reflect.DeepEqual(training, expected), "training scope or saved arrays changed")
	require(report.Options == (contractdecision.FitOptions{Epochs: 2000, LearningRate: .3, L2: .0001, Seed: 37}), "frozen fit options")
	for _, mode := range []string{"mean", "extreme"} {
		info, ok := report.Models[mode]
		require(ok, "model information missing")
		raw := plain(filepath.Join(root, "model-"+mode+".json"))
		require(r.Hash(raw) == info.SHA && len(raw) == info.ArtifactBytes, "model file identity")
		// Decoding and fingerprinting do not execute a prediction.
		model, err := contractdecision.DecodeCanonicalInteractionRequirementConditioned(raw)
		must(err)
		pool := contractdecision.MeanPooling
		if mode == "extreme" {
			pool = contractdecision.ExtremePooling
		}
		marker := decode[struct {
			Options contractdecision.FitOptions
			Rows    int
			Pooling string
		}](plain(filepath.Join(root, mode+"-fit-started.json")))
		require(marker.Options == report.Options && marker.Rows == 64 && marker.Pooling == pool, "fit-start recipe")
		require(model.Pooling() == pool && model.Fingerprint() == info.Fingerprint, "model mode or fingerprint")
		require(info.Parameters == 19034 && info.WeightBytes == 76136 && info.FitNS > 0, "model size or fit duration")
		history := decode[[]contractdecision.Epoch](inflated(filepath.Join(root, "history-"+mode+".json.gz")))
		require(len(history) == 2000 && history[0].Loss == info.FirstLoss && history[1999].Loss == info.LastLoss, "recorded loss endpoints")
		for i, epoch := range history {
			require(epoch.Number == i+1 && !math.IsNaN(epoch.Loss) && !math.IsInf(epoch.Loss, 0), "loss history")
		}
	}
}

func summarize(report r.Report, frozen []r.Frozen, sources []r.Source, searches []r.Search) Summary {
	require(len(frozen) == 256 && len(sources) == 256 && len(searches) == 768, "source/session cardinality")
	byID := map[string]r.Source{}
	for i, source := range sources {
		require(reflect.DeepEqual(source.Frozen, frozen[i]) && r.Hash([]byte(source.Gooo)) == source.SourceSHA, "source changed from frozen corpus")
		_, exists := byID[source.Spec.ID]
		require(!exists && len(source.Candidates) == 4, "source identity or candidates")
		var declared, probes uint64
		for mask, candidate := range source.Candidates {
			require(candidate.Mask == uint16(mask), "candidate order")
			require(len(candidate.Declared.Outputs) == 8 && len(candidate.Declared.Conditions) == 3 &&
				len(candidate.Probe.Outputs) == 16 && len(candidate.Probe.Conditions) == 3, "candidate populations")
			validateOutcomes(candidate.Declared)
			validateOutcomes(candidate.Probe)
			require(candidate.Acceptable == r.Passed(candidate.Declared) && candidate.ProbeAcceptable == r.Passed(candidate.Probe), "candidate flags")
			if r.Passed(candidate.Declared) {
				declared |= 1 << mask
			}
			if r.Passed(candidate.Probe) {
				probes |= 1 << mask
			}
		}
		require(declared == source.Acceptable && probes == source.ProbeAcceptable && declared == probes && bits.OnesCount64(declared) == 1, "unique candidate label")
		byID[source.Spec.ID] = source
	}
	result := Summary{Schema: "gooo/canonical-pooling-audit/v1", Documents: len(sources), Fits: 2,
		Groups: map[string]r.Group{}, ExtremeComparedWithMean: map[string]Delta{}, EquivalentFormPairs: map[string]int{}, Prediction: map[string]Timing{}}
	byMode := map[string]map[string]r.Search{"fixed": {}, "mean": {}, "extreme": {}}
	times := map[string][]int64{}
	for _, row := range searches {
		source, found := byID[row.ID]
		require(found && byMode[row.Mode] != nil, "unknown source or policy")
		_, duplicate := byMode[row.Mode][row.ID]
		require(!duplicate && row.Split == source.Spec.Split && row.Form == source.Spec.Form && row.Family == source.Spec.Family, "session source scope")
		byMode[row.Mode][row.ID] = row
		validateSession(source, row)
		if row.Mode != "fixed" {
			ranking := row.Stages[0].Ranking
			require(ranking.ModelFingerprint == report.Models[row.Mode].Fingerprint && ranking.Applied && !ranking.Declined &&
				ranking.Error == "" && ranking.SourceFeatures == contractdecision.CanonicalOrderedSourceFeatureVersion, "ranking model identity")
		}
		addGroup(&result, source, row)
		if row.Mode != "fixed" {
			times[row.Mode] = append(times[row.Mode], row.Stages[0].Ranking.PredictNS)
		}
	}
	require(result.Sessions == 768 && result.Calls == 512 && reflect.DeepEqual(result.Groups, report.Groups), "summary changed")
	for _, source := range sources {
		mean := byMode["mean"][source.Spec.ID]
		extreme := byMode["extreme"][source.Spec.ID]
		a := firstComplete(source, mean)
		b := firstComplete(source, extreme)
		delta := result.ExtremeComparedWithMean[source.Spec.Split]
		delta.Documents++
		if !a && b {
			delta.ImprovementIDs = append(delta.ImprovementIDs, source.Spec.ID)
		}
		if a && !b {
			delta.RegressionIDs = append(delta.RegressionIDs, source.Spec.ID)
		}
		result.ExtremeComparedWithMean[source.Spec.Split] = delta
		if source.Spec.Form == "assignment" {
			directID := strings.Replace(source.Spec.ID, "-assignment-", "-direct-", 1)
			require(reflect.DeepEqual(source.CanonicalInputs, byID[directID].CanonicalInputs), "equivalent source inputs")
			for _, mode := range []string{"mean", "extreme"} {
				a, b := byMode[mode][source.Spec.ID].Stages[0].Ranking, byMode[mode][directID].Stages[0].Ranking
				require(reflect.DeepEqual(a.Logits, b.Logits) && a.Proposed == b.Proposed, "equivalent source ranking")
				result.EquivalentFormPairs[mode]++
			}
		}
	}
	for mode, values := range times {
		sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
		n := len(values)
		require(n == 256 && values[0] > 0, "prediction duration population")
		result.Prediction[mode] = Timing{n, float64(values[n/2-1]+values[n/2]) / 2, values[int(math.Ceil(.95*float64(n)))-1]}
	}
	return result
}

func firstComplete(source r.Source, row r.Search) bool {
	mask := row.Stages[0].Progress[1].NewAttempts[0].Mask
	return source.Acceptable&(1<<mask) != 0
}

func validateSession(source r.Source, row r.Search) {
	require(len(row.Stages) == 1, "unexpected feedback stage")
	stage := row.Stages[0]
	require(stage.Gooo == source.Gooo && stage.SourceSHA == source.SourceSHA && stage.PlanSHA == source.PlanSHA &&
		stage.CasesSHA == source.CasesSHA && reflect.DeepEqual(stage.Document, source.Document), "session changed source")
	require(stage.NextOutput == nil && stage.NextCondition == nil && len(stage.Progress) >= 2 && len(stage.Progress) <= 5, "bounded initial construction")
	calls := 1
	if row.Mode == "fixed" {
		calls = 0
	}
	require(stage.Ranking.Calls == calls && stage.Progress[0].Attempted == 0 && len(stage.Progress[0].NewAttempts) == 0, "prediction order")
	require(stage.Ranking.PlanSHA == source.PlanSHA && stage.Ranking.CaseSHA == source.CasesSHA && stage.Ranking.ChoiceCount == 2 && stage.Ranking.CaseCount == 8, "ranking inputs")
	if row.Mode == "fixed" {
		require(!stage.Ranking.Applied && stage.Ranking.ModelFingerprint == "" && stage.Ranking.PredictNS == 0, "fixed policy invoked a model")
	}
	seen := map[uint16]bool{}
	for i, step := range stage.Progress {
		require(step.PredictionsThisAdvance == 0, "unexpected late prediction")
		if i == 0 {
			continue
		}
		require(len(step.NewAttempts) == 1 && step.Attempted == i, "one candidate per step")
		attempt := step.NewAttempts[0]
		require(attempt.Mask < 4 && !seen[attempt.Mask], "duplicate or out-of-range candidate")
		seen[attempt.Mask] = true
		if i == 1 {
			require(attempt.Mask == stage.Ranking.Proposed, "first proposed mask")
		}
		expected := source.Candidates[attempt.Mask].Declared
		require(reflect.DeepEqual(attempt.Results, expected.Outputs) && reflect.DeepEqual(attempt.Conditions, expected.Conditions), "attempt differs from collected observations")
		passed := 0
		for _, result := range expected.Outputs {
			if result.Passed {
				passed++
			}
		}
		require(attempt.Passed == passed && attempt.Total == len(expected.Outputs), "attempt counts")
	}
	last := stage.Progress[len(stage.Progress)-1]
	require(last.Status == "TRAINING_COMPLETE" && r.Passed(stage.Probe) && stage.NS > 0, "selected body completion")
	validateOutcomes(stage.Probe)
}

func validateOutcomes(observation r.Observations) {
	for _, result := range observation.Outputs {
		require(result.Passed == (result.Expected == result.Actual), "output pass flag")
	}
	for _, result := range observation.Conditions {
		expected := result.Observation.Reached && result.Case.Expected == result.Observation.Value
		require(result.Passed == expected, "condition pass flag")
	}
}

func addGroup(result *Summary, source r.Source, row r.Search) {
	stage := row.Stages[0]
	first := stage.Progress[1].NewAttempts[0]
	last := stage.Progress[len(stage.Progress)-1]
	for _, key := range []string{row.Mode + "/all", row.Mode + "/" + row.Split, row.Mode + "/" + row.Split + "/" + row.Form} {
		g := result.Groups[key]
		g.Programs++
		if first.Passed == first.Total && pathplan.ConditionsPassed(first.Conditions) {
			g.FirstDeclaredComplete++
		}
		if r.Passed(source.Candidates[first.Mask].Probe) {
			g.FirstProbeComplete++
		}
		g.InitialDeclaredComplete++
		g.InitialProbeComplete++
		g.InitialAttempts += last.Attempted
		g.TotalAttempts += last.Attempted
		g.ModelCalls += stage.Ranking.Calls
		g.SearchNS += stage.NS
		g.PredictNS += stage.Ranking.PredictNS
		result.Groups[key] = g
	}
	result.Sessions++
	result.Calls += stage.Ranking.Calls
}
