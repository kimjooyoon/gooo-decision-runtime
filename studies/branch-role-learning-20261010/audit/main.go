// Audit saved observations only: no Fit, Predict, Compile or Evaluate calls.
package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/conditiondecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

var versions = [2]string{decision.ConditionChannelFeatureVersion, decision.ConditionBranchFeatureVersion}

type source struct {
	ID       string            `json:"id"`
	Split    string            `json:"split"`
	Text     string            `json:"gooo_source"`
	SHA      string            `json:"source_sha256"`
	Document pathplan.Document `json:"document"`
}
type dataset struct {
	Producer string   `json:"producer_revision"`
	Compiler string   `json:"compiler_revision"`
	Records  []source `json:"records"`
}
type candidate struct {
	Mask            uint16                     `json:"mask"`
	Choices         map[string]string          `json:"choices"`
	TypeError       string                     `json:"type_error,omitempty"`
	GoooBody        string                     `json:"gooo_body"`
	GoSHA           string                     `json:"go_sha256"`
	Cases           []pathplan.TestResult      `json:"cases"`
	Conditions      []pathplan.ConditionResult `json:"conditions"`
	OutputPassed    int                        `json:"output_passed"`
	ConditionPassed int                        `json:"condition_passed"`
	Accepted        bool                       `json:"accepted"`
}
type program struct {
	ID         string      `json:"id"`
	Split      string      `json:"split"`
	PlanSHA    string      `json:"plan_sha256"`
	Acceptable uint64      `json:"acceptable_candidate_bits"`
	Candidates []candidate `json:"candidates"`
}
type input struct {
	ID         string            `json:"id"`
	Split      string            `json:"split"`
	Context    string            `json:"context"`
	Features   [2][][256]float32 `json:"features_v1_v2"`
	SHA        [2]string         `json:"features_sha256_v1_v2"`
	Masks      []uint16          `json:"candidate_masks"`
	Acceptable uint64            `json:"acceptable_candidate_bits"`
}
type group struct {
	Rows            int `json:"rows"`
	Valid           int `json:"valid"`
	FallbackValid   int `json:"fallback_valid"`
	OutputPassed    int `json:"output_passed"`
	OutputTotal     int `json:"output_total"`
	ConditionPassed int `json:"condition_passed"`
	ConditionTotal  int `json:"condition_total"`
}
type fit struct {
	Version     string  `json:"feature_version"`
	SHA         string  `json:"model_sha256"`
	Fingerprint string  `json:"model_fingerprint"`
	TrainingSHA string  `json:"training_samples_sha256"`
	Bytes       int     `json:"artifact_bytes"`
	TrainingNS  int64   `json:"training_ns"`
	First       float64 `json:"first_loss"`
	Last        float64 `json:"last_pre_update_loss"`
	Median      int64   `json:"prediction_median_ns"`
	P95         int64   `json:"prediction_p95_ns"`
}
type judgment struct {
	ID         string                       `json:"id"`
	Split      string                       `json:"split"`
	Context    string                       `json:"context"`
	Version    string                       `json:"feature_version"`
	SourceSHA  string                       `json:"source_sha256"`
	FeatureSHA string                       `json:"feature_sha256"`
	Acceptable uint64                       `json:"acceptable_candidate_bits"`
	Prediction conditiondecision.Prediction `json:"prediction"`
	NS         int64                        `json:"predict_ns"`
	Selected   candidate                    `json:"selected"`
}
type searchGroup struct {
	Programs      int   `json:"programs"`
	Complete      int   `json:"complete"`
	Attempts      int   `json:"attempts"`
	Calls         int   `json:"model_calls"`
	FeedbackCalls int   `json:"feedback_calls"`
	Reused        int   `json:"reused"`
	NS            int64 `json:"search_ns"`
}
type searchRow struct {
	ID       string                       `json:"id"`
	Split    string                       `json:"split"`
	Mode     string                       `json:"mode"`
	NS       int64                        `json:"search_ns"`
	Result   pathplan.SearchResult        `json:"result"`
	Progress []pathplan.ConditionProgress `json:"progress"`
	Feedback []pathplan.ConditionRanking  `json:"feedback"`
	Body     string                       `json:"selected_body"`
	Error    string                       `json:"error,omitempty"`
}
type report struct {
	Schema        string                       `json:"schema"`
	Producer      string                       `json:"producer_revision"`
	Compiler      string                       `json:"compiler_revision"`
	DatasetSHA    string                       `json:"dataset_sha256"`
	Programs      int                          `json:"programs"`
	Contexts      int                          `json:"contexts"`
	TrainingRows  int                          `json:"training_rows_per_fit"`
	TrainingCalls int                          `json:"training_calls"`
	Judgments     int                          `json:"judgments"`
	Searches      int                          `json:"searches"`
	SearchCalls   int                          `json:"search_model_calls"`
	Native        int                          `json:"native_executions"`
	Parameters    int                          `json:"parameters_per_model"`
	WeightBytes   int                          `json:"weight_bytes_per_model"`
	InputBytes    int                          `json:"input_bytes_per_choice"`
	Options       conditiondecision.FitOptions `json:"fit_options"`
	Fits          map[string]fit               `json:"fits"`
	Groups        map[string]group             `json:"groups"`
	SearchGroups  map[string]searchGroup       `json:"search_groups"`
	Conflicts     map[string]int               `json:"conflicting_initial_pairs"`
}

func require(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
func hash(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func raw(dir, name string) []byte {
	b, err := os.ReadFile(filepath.Join(dir, name))
	must(err)
	if strings.HasSuffix(name, ".gz") {
		z, err := gzip.NewReader(bytes.NewReader(b))
		must(err)
		b, err = io.ReadAll(z)
		must(err)
		must(z.Close())
	}
	return b
}
func decode[T any](b []byte) T { var v T; must(json.Unmarshal(b, &v)); return v }
func lines[T any](b []byte) []T {
	var rows []T
	s := bufio.NewScanner(bytes.NewReader(b))
	s.Buffer(make([]byte, 64<<10), 2<<20)
	for s.Scan() {
		rows = append(rows, decode[T](s.Bytes()))
	}
	must(s.Err())
	return rows
}
func featureSHA(features [][256]float32) string {
	d := sha256.New()
	_, _ = d.Write([]byte{byte(len(features))})
	var b [1024]byte
	for _, row := range features {
		for i, x := range row {
			require(!math.IsNaN(float64(x)) && !math.IsInf(float64(x), 0), "nonfinite input")
			binary.LittleEndian.PutUint32(b[4*i:], math.Float32bits(x))
		}
		_, _ = d.Write(b[:])
	}
	return fmt.Sprintf("%x", d.Sum(nil))
}
func jsonHash(value any) string { b, err := json.Marshal(value); must(err); return hash(b) }

func run(dir string) any {
	datasetRaw := raw(dir, "dataset.json.gz")
	d := decode[dataset](datasetRaw)
	r := decode[report](raw(dir, "report.json"))
	require(r.Schema == "gooo/branch-role-learning-study/v1" && r.Producer == "b3360fb08c3f72bef003821fe4543de484c70664" && r.Producer == d.Producer && r.Compiler == d.Compiler && r.Compiler == "1ceb96b99fa6fabe12648f5ec6deac47a0d546e1" && r.DatasetSHA == hash(datasetRaw), "producer binding")
	require(r.Programs == 60 && len(d.Records) == 60 && r.Contexts == 292 && r.TrainingRows == 40 && r.TrainingCalls == 2 && r.Judgments == 584 && r.Searches == 300 && r.Native == 0, "protocol counts")
	require(r.Parameters == 6218 && r.WeightBytes == 24872 && r.InputBytes == 1024 && r.Options == (conditiondecision.FitOptions{Epochs: 400, LearningRate: 0.25, L2: 0.0001, Seed: 17}), "fixed architecture/options")
	sources := map[string]source{}
	splitCounts := map[string]int{}
	for _, s := range d.Records {
		_, seen := sources[s.ID]
		require(!seen && s.SHA == hash([]byte(s.Text)), "source identity")
		sources[s.ID] = s
		splitCounts[s.Split]++
	}
	require(reflect.DeepEqual(splitCounts, map[string]int{"train": 8, "wording": 16, "structure": 8, "both": 16, "nested": 8, "literal_collision": 4}), "source splits")
	programs := map[string]program{}
	candidateCount := 0
	big := map[int64]bool{}
	for _, p := range lines[program](raw(dir, "candidates.jsonl.gz")) {
		s, exists := sources[p.ID]
		_, seen := programs[p.ID]
		require(exists && !seen && p.Split == s.Split && len(p.Candidates) == 1<<len(s.Document.Plan.Decisions), "program binding")
		var acceptable uint64
		for i, c := range p.Candidates {
			require(int(c.Mask) == i && c.TypeError == "" && c.GoooBody != "" && len(c.GoSHA) == 64 && len(c.Cases) == len(s.Document.TestCases) && len(c.Conditions) == len(s.Document.Plan.ConditionCases), "candidate shape")
			for n, choice := range s.Document.Plan.Decisions {
				require(c.Choices[choice.ID] == choice.Options[c.Mask>>n&1].Label, "candidate choice labels")
			}
			outputs, conditions := 0, 0
			for j, test := range c.Cases {
				declared := s.Document.TestCases[j]
				require(test.Input == declared.Input && test.Expected == declared.Expected && test.Passed == (test.Actual == test.Expected), "exact output case")
				big[test.Input] = true
				if test.Passed {
					outputs++
				}
			}
			for j, test := range c.Conditions {
				want := test.Observation.Reached && test.Observation.Value == test.Case.Expected
				require(test.Case == s.Document.Plan.ConditionCases[j] && test.Passed == want, "declared condition")
				status := "NOT_REACHED"
				if test.Observation.Reached {
					status = "MISMATCH"
					if want {
						status = "MATCH"
					}
				}
				require(test.Status == status, "condition status")
				if test.Passed {
					conditions++
				}
			}
			require(outputs == c.OutputPassed && conditions == c.ConditionPassed && c.Accepted == (outputs == len(c.Cases) && conditions == len(c.Conditions)), "candidate completeness")
			if c.Accepted {
				acceptable |= 1 << i
			}
			candidateCount++
		}
		require(p.Acceptable == acceptable && acceptable != 0, "acceptable complete mask set")
		programs[p.ID] = p
	}
	require(len(programs) == 60 && candidateCount == 232, "candidate enumeration")
	for _, x := range []int64{9007199254740993, -9007199254740995, 9007199254740995, 18014398509481990} {
		require(big[x], "large integer absent")
	}
	inputs := lines[input](raw(dir, "contexts.jsonl.gz"))
	require(len(inputs) == 292, "context count")
	byInput := map[string]input{}
	initial := map[string]input{}
	var training [2][]conditiondecision.Sample
	for _, in := range inputs {
		s, ok := sources[in.ID]
		p := programs[in.ID]
		_, seen := byInput[in.ID+"/"+in.Context]
		require(ok && !seen && in.Split == s.Split && in.Acceptable == p.Acceptable && len(in.Masks) == len(p.Candidates), "context binding")
		for i, m := range in.Masks {
			require(int(m) == i, "mask order")
		}
		for v := range 2 {
			require(len(in.Features[v]) == len(s.Document.Plan.Decisions) && featureSHA(in.Features[v]) == in.SHA[v], "actual feature hash")
			if in.Split == "train" {
				training[v] = append(training[v], conditiondecision.Sample{Inputs: in.Features[v], Masks: in.Masks, Acceptable: in.Acceptable})
			}
		}
		for i := range in.Features[0] {
			require(slices.Equal(in.Features[0][i][:236], in.Features[1][i][:236]), "v1 populated cells changed")
		}
		if in.Context == "initial" {
			initial[in.ID] = in
			for v := range 2 {
				for _, f := range in.Features[v] {
					for _, x := range f[192:236] {
						require(x == 0, "initial invented condition")
					}
				}
			}
		}
		byInput[in.ID+"/"+in.Context] = in
	}
	for _, in := range inputs {
		start := initial[in.ID]
		for v := range 2 {
			for i, f := range in.Features[v] {
				require(slices.Equal(f[:192], start.Features[v][i][:192]) && slices.Equal(f[236:], start.Features[v][i][236:]), "feedback changed static input")
			}
		}
	}
	conflicts := map[string]int{}
	for v, version := range versions {
		count := 0
		for i, a := range inputs {
			if a.Context != "initial" {
				continue
			}
			for _, b := range inputs[:i] {
				if b.Context == "initial" && a.ID != b.ID && a.SHA[v] == b.SHA[v] && a.Acceptable&b.Acceptable == 0 {
					count++
				}
			}
		}
		conflicts[version] = count
		f := r.Fits[version]
		artifact := raw(dir, fmt.Sprintf("model-v%d.json", v+1))
		m, err := conditiondecision.Decode(artifact)
		must(err)
		require(len(training[v]) == 40 && jsonHash(training[v]) == f.TrainingSHA && len(artifact) == f.Bytes && hash(artifact) == f.SHA && m.Fingerprint() == f.Fingerprint && m.FeatureVersion() == version && f.Version == version && f.TrainingNS > 0, "fit/artifact binding")
		history := decode[[]conditiondecision.Epoch](raw(dir, fmt.Sprintf("history-v%d.json.gz", v+1)))
		require(len(history) == 400 && history[0].Loss == f.First && history[399].Loss == f.Last, "fit history")
		for i, h := range history {
			require(h.Number == i+1 && !math.IsNaN(h.Loss) && !math.IsInf(h.Loss, 0), "history order")
		}
	}
	require(reflect.DeepEqual(conflicts, r.Conflicts) && len(initial) == 60, "collision recount")
	groups := map[string]group{}
	times := map[string][]int64{}
	seenJudgments := map[string]bool{}
	checkedOutputFailures := 0
	judgments := lines[judgment](raw(dir, "judgments.jsonl.gz"))
	require(len(judgments) == 584, "judgment count")
	for _, j := range judgments {
		v := slices.Index(versions[:], j.Version)
		in, ok := byInput[j.ID+"/"+j.Context]
		s := sources[j.ID]
		p := programs[j.ID]
		key := j.Version + "/" + j.ID + "/" + j.Context
		require(v >= 0 && ok && !seenJudgments[key] && j.Split == s.Split && j.SourceSHA == s.SHA && j.FeatureSHA == in.SHA[v] && j.Acceptable == p.Acceptable && j.NS > 0, "judgment binding")
		seenJudgments[key] = true
		require(int(j.Prediction.Selected) < len(p.Candidates) && j.Prediction.Count == len(p.Candidates) && reflect.DeepEqual(j.Selected, p.Candidates[j.Prediction.Selected]), "selected body/cases differ from candidate")
		kind := "observed"
		if j.Context == "initial" {
			kind = "initial"
		}
		groupKey := j.Version + "/" + j.Split + "/" + kind
		g := groups[groupKey]
		g.Rows++
		if j.Selected.Accepted {
			g.Valid++
		}
		if p.Candidates[0].Accepted {
			g.FallbackValid++
		}
		g.OutputPassed += j.Selected.OutputPassed
		g.OutputTotal += len(j.Selected.Cases)
		g.ConditionPassed += j.Selected.ConditionPassed
		g.ConditionTotal += len(j.Selected.Conditions)
		groups[groupKey] = g
		times[j.Version] = append(times[j.Version], j.NS)
		if !j.Selected.Accepted && len(j.Selected.Conditions) > 0 && j.Selected.ConditionPassed == len(j.Selected.Conditions) {
			checkedOutputFailures++
		}
	}
	require(reflect.DeepEqual(groups, r.Groups), "judgment totals")
	for version, times := range times {
		slices.Sort(times)
		require(times[len(times)/2] == r.Fits[version].Median && times[(len(times)*95-1)/100] == r.Fits[version].P95, "timing recount")
	}
	searchGroups := map[string]searchGroup{}
	seenSearch := map[string]bool{}
	searches := lines[searchRow](raw(dir, "searches.jsonl.gz"))
	require(len(searches) == 300, "search count")
	searchCalls := 0
	feedbackCalls := 0
	for _, row := range searches {
		p, ok := programs[row.ID]
		key := row.Mode + "/" + row.ID
		require(ok && !seenSearch[key] && row.Split == p.Split && row.NS > 0 && row.Error == "" && row.Result.Status == "TRAINING_COMPLETE" && len(row.Progress) > 0, "search binding/completion")
		seenSearch[key] = true
		modes := []string{"deterministic", "v1_initial", "v1_feedback", "v2_initial", "v2_feedback"}
		require(slices.Contains(modes, row.Mode), "search mode")
		seenMasks := map[uint16]bool{}
		for _, a := range row.Result.Attempts {
			require(int(a.Mask) < len(p.Candidates) && !seenMasks[a.Mask], "repeated candidate")
			seenMasks[a.Mask] = true
			c := p.Candidates[a.Mask]
			require(a.Passed == c.OutputPassed && a.Total == len(c.Cases) && reflect.DeepEqual(a.Results, c.Cases) && reflect.DeepEqual(a.Conditions, c.Conditions), "attempt differs from saved candidate")
		}
		last := row.Result.Attempts[len(row.Result.Attempts)-1]
		require(p.Candidates[last.Mask].Accepted && row.Body == p.Candidates[last.Mask].GoooBody, "final accepted body")
		calls := row.Progress[0].Ranking.Calls
		g := searchGroups[row.Mode+"/"+row.Split]
		g.Programs++
		g.Complete++
		g.Attempts += len(row.Result.Attempts)
		g.NS += row.NS
		for _, feedback := range row.Feedback {
			calls += feedback.Calls
			g.FeedbackCalls += feedback.Calls
			feedbackCalls += feedback.Calls
			if feedback.Reused {
				g.Reused++
			}
			require(!feedback.HasFailure && feedback.Calls == 0, "new condition-driven inference in this observation")
		}
		for _, progress := range row.Progress {
			digest := progress.SHA
			progress.SHA = ""
			require(jsonHash(progress) == digest, "progress digest")
			ranking := progress.Ranking
			digest = ranking.SHA
			ranking.SHA = ""
			require(jsonHash(ranking) == digest, "ranking digest")
		}
		require(calls == row.Result.Selection.ModelCalls, "actual search calls")
		if row.Mode == "deterministic" {
			require(calls == 0, "deterministic model call")
		}
		g.Calls += calls
		searchGroups[row.Mode+"/"+row.Split] = g
		searchCalls += calls
	}
	require(reflect.DeepEqual(searchGroups, r.SearchGroups) && searchCalls == r.SearchCalls && searchCalls == 240, "search totals")
	completed := decode[map[string]int](raw(dir, "completed.json"))
	require(completed["training_calls"] == 2 && completed["judgments"] == 584 && completed["searches"] == 300 && completed["search_model_calls"] == 240 && completed["native_executions"] == 0, "completion marker")
	return map[string]any{"status": "PASS", "programs": 60, "complete_candidates": candidateCount, "contexts": 292, "judgments": 584, "searches": 300, "recorded_search_model_calls": searchCalls, "recorded_feedback_model_calls": feedbackCalls, "judgments_with_wrong_outputs_and_all_declared_conditions_passed": checkedOutputFailures, "conflicting_initial_pairs": conflicts, "groups": groups, "search_groups": searchGroups, "new_training_calls": 0, "new_model_predictions": 0, "new_program_executions": 0}
}

func main() {
	require(len(os.Args) == 2, "result directory required")
	out, err := json.Marshal(run(os.Args[1]))
	must(err)
	fmt.Println(string(out))
}
