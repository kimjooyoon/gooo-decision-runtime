package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"

	"github.com/kimjooyoon/gooo-decision-runtime/flowdecision"
)

const producer = "1e0c6776af92f03cb0c6e63124b664f2ef7a8a4f"
const dataset = "05fa80975f0e6ea56e0e653fc1207d5894805957cd7a4a54091b67d0e1a7ed40"

func initialRecords(root string, recorded modelRecord) map[string]initial {
	absolute, err := filepath.Abs(root)
	must(err)
	directory, file, modelSHA, initialSHA := "leaky-flow-learning-20261010", "model-leaky_v6.json", "d5f9c4ffb204852f4e1b3b1b1c491c7d1fb0753d0df2c1e0f0ffead80d9ac47e", "4ad76a785a122b478128b88edce4a056b90e9f867c7e7d631d5a3cf05de61acf"
	if recorded.Name == "observation_trained" {
		directory, file, modelSHA, initialSHA = "feedback-flow-learning-20261010", "model-feedback_v6.json", "b81317ce44943549c58237eb9b87e797eca56c181a24086780f7f08a13d4ad59", "4aba2c54b83efd939fb3b2054ab03b87ed7bc6a44a640c743fbc2242a66242c8"
	} else {
		require(recorded.Name == "initial_trained", "known fixed model")
	}
	base := filepath.Join(filepath.Dir(filepath.Dir(absolute)), directory, "result")
	require(recorded.SHA == modelSHA && hash(read(base, file)) == modelSHA && recorded.InitialSHA == initialSHA, "fixed model and initial identities")
	m, err := flowdecision.Decode(read(base, file))
	must(err)
	require(m.Fingerprint() == recorded.Fingerprint && m.Activation() == flowdecision.LeakyReLUActivation, "frozen computation identity")
	z, err := os.ReadFile(filepath.Join(base, "judgments.jsonl.gz"))
	must(err)
	require(hash(z) == initialSHA, "fixed saved judgments")
	rows := map[string]initial{}
	for _, row := range lines[initial](read(base, "judgments.jsonl.gz")) {
		rows[row.ID] = row
	}
	require(len(rows) == 162, "all baseline sources")
	return rows
}

func auditObservation(s source, o observation) {
	require(s.Split != "future_train" && o.SourceSHA == s.SourceSHA && o.PlanSHA == s.PlanSHA && o.CaseSHA == hash(jsonBytes(s.Document.TestCases)) && o.Mask < 4 && s.Acceptable>>o.Mask&1 == 0 && o.NS > 0, "heldout rejected source identity")
	c, hc, v, hv := expectedFailures(s, o.Mask)
	require((hc || hv) && o.ConditionPresent == hc && o.OutputPresent == hv && reflect.DeepEqual(o.Condition, c) && o.Output == v, "exact first actual failures")
	f := observedFeatures(s, o)
	require(reflect.DeepEqual(o.Inputs, f) && o.InputSHA == featureSHA(f), "exact observed input and unchanged source channels")
}

func audit(root, frozen string) map[string]any {
	z, err := os.ReadFile(filepath.Join(frozen, "records.jsonl.gz"))
	must(err)
	require(hash(z) == dataset, "frozen dataset")
	sources := lines[source](read(frozen, "records.jsonl.gz"))
	byID := map[string]source{}
	var order []string
	for _, s := range sources {
		if s.Split == "future_train" {
			continue
		}
		require(hash([]byte(s.Source)) == s.SourceSHA, "source bytes")
		s.V6 = relational(s)
		byID[s.ID] = s
		order = append(order, s.ID)
	}
	require(len(byID) == 146, "heldout source count")
	r := decode[report](read(root, "report.json"))
	require(r.Schema == "gooo/heldout-repair/v1" && r.Producer == producer && r.DatasetSHA == dataset && r.Sources == 146 && r.Observations == 438 && r.ModelCalls == 876 && r.SelectedEvaluations == 876 && r.Fits == 0 && r.NativeExecutions == 0, "fixed protocol identity/counts")
	require(len(r.Models) == 2 && r.Models[0].Name == "initial_trained" && r.Models[1].Name == "observation_trained", "fixed comparison order")
	initials := map[string]map[string]initial{}
	for _, model := range r.Models {
		initials[model.Name] = initialRecords(root, model)
	}
	observations := lines[observation](read(root, "observations.jsonl.gz"))
	require(len(observations) == 438, "all rejected starts")
	byObservation := map[string]observation{}
	n := 0
	var ns int64
	for _, id := range order {
		s := byID[id]
		for mask := range uint16(4) {
			if s.Acceptable>>mask&1 != 0 {
				continue
			}
			o := observations[n]
			require(o.ID == id && o.Mask == mask, "fixed observed order")
			auditObservation(s, o)
			byObservation[fmt.Sprintf("%s/%d", id, mask)] = o
			n++
			ns += o.NS
		}
	}
	require(n == 438 && ns == r.ObservationNS, "observation timing total")
	judgments := lines[judgment](read(root, "judgments.jsonl.gz"))
	require(len(judgments) == 876, "every next judgment")
	groups := map[string]group{}
	times := map[string][]int64{}
	for i, j := range judgments {
		o := observations[i/2]
		s := byID[j.ID]
		require(j.ID == o.ID && j.ObservedMask == o.Mask && j.Model == r.Models[i%2].Name && j.Split == s.Split && j.Form == s.Form && j.InputSHA == o.InputSHA && j.NS > 0, "paired judgment identity/order")
		auditTrace(j)
		var mass float64
		for k, p := range j.Prediction.Probabilities {
			require(!math.IsNaN(float64(p)) && p >= 0 && p <= 1 && (k < 4 || p == 0), "bounded saved probabilities")
			mass += float64(p)
		}
		require(math.Abs(mass-1) < .00001, "probability mass")
		require(j.Prediction.Count == 4 && j.Prediction.Selected < 4 && j.Selected.Mask == next(j.Explanation.CandidateScores, o.Mask) && j.Selected.Mask != o.Mask, "raw and untried selection")
		require(j.RawValid == (s.Acceptable>>j.Prediction.Selected&1 != 0) && j.RawRepeats == (j.Prediction.Selected == o.Mask), "raw repeats and validity")
		require(len(j.Selected.Outputs) == 8 && len(j.Selected.Conditions) == 3 && j.Selected.Body != "" && len(j.Selected.GoSHA) == 64, "whole actual selected program")
		outputs := auditOutputs(s, j.Selected.Mask, j.Selected.Outputs)
		conditions := auditConditions(s, j.Selected.Mask, j.Selected.Conditions)
		require(j.Selected.Valid == (outputs == 8 && conditions == 3) && j.Selected.Valid == (s.Acceptable>>j.Selected.Mask&1 != 0), "actual candidate acceptance")
		prior := initials[j.Model][j.ID]
		require(prior.SourceSHA == s.SourceSHA && prior.InputSHA == featureSHA(s.V6) && prior.Acceptable == s.Acceptable, "same saved source/input")
		previous := next(prior.Explanation.CandidateScores, o.Mask)
		require(j.InitialRemaining == previous && j.InitialRemainingValid == (s.Acceptable>>previous&1 != 0), "read-only no-feedback counterfactual")
		for _, key := range []string{j.Model + "/all", j.Model + "/" + s.Split, j.Model + "/form/" + s.Form} {
			groups[key] = addGroup(groups[key], j)
		}
		times[j.Model] = append(times[j.Model], j.NS)
	}
	require(reflect.DeepEqual(groups, r.Groups), "recounted groups")
	for name, values := range times {
		slices.Sort(values)
		require(len(values) == 438 && r.MedianNS[name] == values[219] && r.P95NS[name] == values[(len(values)*95-1)/100], "recounted latencies")
	}
	return map[string]any{"status": "PASS", "producer": producer, "sources": 146, "observations": 438, "judgments": 876, "new_audit_model_calls": 0, "new_audit_candidate_executions": 0, "new_fits": 0, "groups": groups}
}

func main() {
	require(len(os.Args) == 3, "usage: audit RESULT_DIRECTORY FROZEN_SOURCE_DIRECTORY")
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	must(e.Encode(audit(os.Args[1], os.Args[2])))
}
