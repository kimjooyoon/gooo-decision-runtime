// Read-only recount of the original fit artifacts and execution records.
// No Fit, Predict, Prepare, Compile, Evaluate or native process is invoked.
package main

import (
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
	"github.com/kimjooyoon/gooo-decision-runtime/flowdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

const producer = "91d3f58e4f80f2a71b7812ec489125ee17d8d78c"
const dataset = "05fa80975f0e6ea56e0e653fc1207d5894805957cd7a4a54091b67d0e1a7ed40"

type output struct {
	Input, Expected, Actual int64
	Passed                  bool
}
type candidate struct {
	Mask       uint16
	Outputs    []output
	Conditions []pathplan.ConditionResult
	Acceptable bool
}
type source struct {
	ID, Group, Form, Split, Source, SourceSHA, PlanSHA string
	Document                                           pathplan.Document
	V4, V5                                             [][384]float32
	V4SHA, V5SHA                                       string
	Acceptable                                         uint64
	Candidates                                         []candidate
	Contexts                                           []pathplan.SemanticBranchContext
	V6                                                 [][384]float32
}
type fitRecord struct {
	FeatureVersion, ModelSHA, Fingerprint, TrainingSHA string
	Activation                                         string
	ArtifactBytes                                      int
	TrainingNS, MedianNS, P95NS                        int64
	FirstLoss, LastPreUpdateLoss                       float64
}
type group struct{ Rows, Valid, OutputPassed, OutputTotal, ConditionPassed, ConditionTotal int }
type searchGroup struct {
	Programs, Complete, Attempts, ModelCalls, FeedbackCalls int
	NS                                                      int64
}
type report struct {
	Schema, Producer, DatasetSHA                                                                                  string
	Sources, TrainingRows, Fits, Judgments, Searches, SearchModelCalls, NativeExecutions, Parameters, WeightBytes int
	Options                                                                                                       flowdecision.FitOptions
	Models                                                                                                        map[string]fitRecord
	Groups                                                                                                        map[string]group
	SearchGroups                                                                                                  map[string]searchGroup
	PairedDistributionMatches                                                                                     map[string]int
	BaselineReportSHA                                                                                             string
}
type selected struct {
	Mask                   uint16
	GoooBody, GoSHA, Error string
	Outputs                []pathplan.TestResult
	Conditions             []pathplan.ConditionResult
	Acceptable             bool
}
type judgment struct {
	ID, Group, Form, Split, Model, SourceSHA, InputSHA string
	Acceptable                                         uint64
	Prediction                                         flowdecision.Prediction
	NS                                                 int64
	Selected                                           selected
	Explanation                                        flowdecision.Explanation
}
type searchRow struct {
	ID, Form, Split, Mode string
	NS                    int64
	Result                pathplan.SearchResult
	Progress              []pathplan.ConditionProgress
	Feedback              []pathplan.ConditionRanking
	GoooBody, Error       string
}

func must(e error) {
	if e != nil {
		panic(e)
	}
}
func require(ok bool, s string) {
	if !ok {
		panic(s)
	}
}
func hash(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }
func read(root, name string) []byte {
	b, e := os.ReadFile(filepath.Join(root, name))
	must(e)
	if strings.HasSuffix(name, ".gz") {
		z, e := gzip.NewReader(bytes.NewReader(b))
		must(e)
		b, e = io.ReadAll(z)
		must(e)
		must(z.Close())
	}
	return b
}
func decode[T any](b []byte) T { var x T; must(json.Unmarshal(b, &x)); return x }
func lines[T any](b []byte) []T {
	var result []T
	for line := range bytes.SplitSeq(b, []byte{'\n'}) {
		if len(line) > 0 {
			result = append(result, decode[T](line))
		}
	}
	return result
}
func choiceSHA(f [384]float32) string {
	var b [1536]byte
	for i, x := range f {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(x))
	}
	return hash(b[:])
}
func relational(s source) [][384]float32 {
	require(len(s.Contexts) == len(s.V5), "source context count")
	f := slices.Clone(s.V5)
	for i, view := range s.Contexts {
		if view.Normalized {
			must(decision.RelationalFlowFeaturesInto(s.V5[i], view.Flow, &f[i]))
		}
	}
	return f
}
func featureSHA(inputs [][384]float32) string {
	h := sha256.New()
	_, _ = h.Write([]byte{byte(len(inputs))})
	for _, f := range inputs {
		var b [1536]byte
		for i, x := range f {
			binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(x))
		}
		_, _ = h.Write(b[:])
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
func auditOutputs(s source, mask uint16, outs []pathplan.TestResult) int {
	require(mask < 4, "finite mask")
	good := 0
	for i, o := range outs {
		require(i < len(s.Document.TestCases), "finite output index")
		c := s.Document.TestCases[i]
		expected := s.Candidates[mask].Outputs[i]
		require(o.Input == c.Input && o.Expected == c.Expected && o.Actual == expected.Actual && o.Passed == (o.Actual == o.Expected), "exact recorded output")
		if o.Passed {
			good++
		}
	}
	return good
}
func auditConditions(s source, mask uint16, conditions []pathplan.ConditionResult) int {
	good := 0
	for i, c := range conditions {
		require(i < len(s.Document.Plan.ConditionCases) && c.Case == s.Document.Plan.ConditionCases[i], "authored condition")
		require(reflect.DeepEqual(c, s.Candidates[mask].Conditions[i]), "frozen condition outcome")
		if c.Passed {
			good++
		}
	}
	return good
}
func audit(root, frozen string) map[string]any {
	compressed, e := os.ReadFile(filepath.Join(frozen, "records.jsonl.gz"))
	must(e)
	require(hash(compressed) == dataset, "immutable dataset")
	sources := lines[source](read(frozen, "records.jsonl.gz"))
	require(len(sources) == 162, "all sources")
	byID := map[string]source{}
	for i := range sources {
		sources[i].V6 = relational(sources[i])
		s := sources[i]
		require(hash([]byte(s.Source)) == s.SourceSHA, "source bytes")
		byID[s.ID] = s
	}
	r := decode[report](read(root, "report.json"))
	require(r.Schema == "gooo/leaky-flow-learning/v1" && r.Producer == producer && r.DatasetSHA == dataset && r.Sources == 162 && r.TrainingRows == 16 && r.Fits == 1 && r.Judgments == 162 && r.Searches == 324 && r.NativeExecutions == 0, "fixed protocol counts")
	require(r.BaselineReportSHA == "2d14177055225af66adc66f28fbf25331f98e013e46c631c9ab5695afadff61f" && hash(read(root, "baseline-report.json")) == r.BaselineReportSHA, "immutable historical baseline")
	require(r.Parameters == 9290 && r.WeightBytes == 37160 && r.Options == (flowdecision.FitOptions{Epochs: 400, LearningRate: .25, L2: .0001, Seed: 17}), "fixed model options")
	require(len(r.Models) == 1, "only one fresh fit")
	require(r.Models["leaky_v6"].FeatureVersion == decision.RelationalFlowFeatureVersion, "explicit v6 artifact")
	for _, name := range []string{"leaky_v6"} {
		fit := r.Models[name]
		b := read(root, "model-"+name+".json")
		m, e := flowdecision.Decode(b)
		must(e)
		require(fit.Activation == flowdecision.LeakyReLUActivation && m.Activation() == fit.Activation && m.ArtifactSchema() == flowdecision.ActivationSchema, "explicit computation artifact")
		require(hash(b) == fit.ModelSHA && len(b) == fit.ArtifactBytes && m.Fingerprint() == fit.Fingerprint && m.FeatureVersion() == fit.FeatureVersion && fit.TrainingNS > 0, "model artifact")
		b = read(root, "training-"+name+".json.gz")
		require(hash(b) == fit.TrainingSHA && fit.TrainingSHA == "e62fd490c2224aef24c25906e0768e666390d8e4f1b6381258bcb24af64251c4", "same exact ReLU baseline training bytes")
		training := decode[[]flowdecision.Sample](b)
		var want []flowdecision.Sample
		for _, s := range sources {
			if s.Split == "future_train" {
				require(s.Form == "direct", "training form")
				f := s.V6
				want = append(want, flowdecision.Sample{Inputs: f, Masks: []uint16{0, 1, 2, 3}, Acceptable: s.Acceptable})
			}
		}
		require(len(training) == 16 && reflect.DeepEqual(training, want), "only16 original direct training inputs")
		history := decode[[]flowdecision.Epoch](read(root, "history-"+name+".json.gz"))
		require(len(history) == 400 && history[0].Loss == fit.FirstLoss && history[399].Loss == fit.LastPreUpdateLoss, "fit history")
		for i, h := range history {
			require(h.Number == i+1 && !math.IsNaN(h.Loss) && !math.IsInf(h.Loss, 0), "epoch sequence")
		}
	}
	judgments := lines[judgment](read(root, "judgments.jsonl.gz"))
	require(len(judgments) == 162, "all judgments")
	groups := map[string]group{}
	seen := map[string]bool{}
	predictions := map[string]flowdecision.Prediction{}
	direct := map[string]flowdecision.Prediction{}
	times := map[string][]int64{}
	for _, j := range judgments {
		s, ok := byID[j.ID]
		require(ok && j.Split == s.Split && j.Form == s.Form && j.Group == s.Group && j.SourceSHA == s.SourceSHA && j.Acceptable == s.Acceptable, "judgment source")
		key := j.Model + "/" + j.ID
		require(!seen[key] && j.Model == "leaky_v6", "unique model/source")
		seen[key] = true
		want := featureSHA(s.V6)
		require(j.InputSHA == want && j.NS > 0, "initial model input")
		auditTrace(j)
		require(j.Prediction.Count == 4 && j.Selected.Mask == j.Prediction.Selected && j.Selected.Error == "" && j.Selected.GoooBody != "" && len(j.Selected.GoSHA) == 64, "selected body")
		var sum float32
		for _, p := range j.Prediction.Probabilities[:4] {
			require(p >= 0 && p <= 1, "finite probability")
			sum += p
		}
		require(math.Abs(float64(sum-1)) < .00001, "probability mass")
		require(len(j.Selected.Outputs) == 8 && len(j.Selected.Conditions) == 3, "selected cases")
		outputs := auditOutputs(s, j.Selected.Mask, j.Selected.Outputs)
		conditions := auditConditions(s, j.Selected.Mask, j.Selected.Conditions)
		valid := outputs == 8 && conditions == 3
		require(j.Selected.Acceptable == valid && valid == (s.Acceptable>>j.Selected.Mask&1 != 0), "selected acceptance")
		for _, groupKey := range []string{j.Model + "/" + s.Split, j.Model + "/form/" + s.Form} {
			g := groups[groupKey]
			g.Rows++
			if valid {
				g.Valid++
			}
			g.OutputPassed += outputs
			g.OutputTotal += 8
			g.ConditionPassed += conditions
			g.ConditionTotal += 3
			groups[groupKey] = g
		}
		predictions[key] = j.Prediction
		if s.Form == "direct" {
			direct[j.Model+"/"+s.Group] = j.Prediction
		}
		times[j.Model] = append(times[j.Model], j.NS)
	}
	require(reflect.DeepEqual(groups, r.Groups), "recounted judgment groups")
	for _, name := range []string{"leaky_v6"} {
		matches := 0
		for _, s := range sources {
			if s.Form != "direct" && s.Split != "control" && predictions[name+"/"+s.ID] == direct[name+"/"+s.Group] {
				matches++
			}
		}
		require(matches == r.PairedDistributionMatches[name], "paired distributions")
		t := times[name]
		slices.Sort(t)
		require(len(t) == 162 && t[81] == r.Models[name].MedianNS && t[(len(t)*95-1)/100] == r.Models[name].P95NS, "latencies")
	}
	searches := lines[searchRow](read(root, "searches.jsonl.gz"))
	require(len(searches) == 324, "all searches")
	searchGroups := map[string]searchGroup{}
	seen = map[string]bool{}
	calls := 0
	for _, row := range searches {
		s, ok := byID[row.ID]
		require(ok && row.Split == s.Split && row.Form == s.Form, "search source")
		key := row.Mode + "/" + row.ID
		require(!seen[key], "unique search")
		seen[key] = true
		require(slices.Contains([]string{"leaky_v6_initial", "leaky_v6_feedback"}, row.Mode), "search mode")
		x := row.Result
		require(row.Error == "" && row.NS > 0 && x.Selection.PlanSHA256 == s.PlanSHA && len(x.Attempts) > 0 && len(x.Attempts) <= 4, "search bounds")
		masks := map[uint16]bool{}
		for _, a := range x.Attempts {
			require(a.Mask < 4 && !masks[a.Mask], "unique candidate")
			masks[a.Mask] = true
			require(a.Passed == auditOutputs(s, a.Mask, a.Results), "attempt passed count")
			auditConditions(s, a.Mask, a.Conditions)
		}
		if x.Status == "TRAINING_COMPLETE" {
			last := x.Attempts[len(x.Attempts)-1]
			require(s.Acceptable>>last.Mask&1 != 0 && last.Passed == 8 && row.GoooBody != "", "completed source cases")
		}
		feedbackCalls := 0
		for _, fb := range row.Feedback {
			feedbackCalls += fb.Calls
		}
		{
			name := "leaky_v6"
			f := s.V6
			require(len(row.Progress) > 0 && x.Selection.ModelCalls == 1+feedbackCalls, "model call accounting")
			initial := row.Progress[0].Ranking
			require(initial.ModelFingerprint == r.Models[name].Fingerprint && initial.FeatureVersion == r.Models[name].FeatureVersion && initial.Calls == 1, "exact model route")
			for i, input := range f {
				require(initial.FeatureSHA[i] == choiceSHA(input), "runtime initial features")
			}
			if strings.HasSuffix(row.Mode, "_initial") {
				require(feedbackCalls == 0, "initial-only mode")
			}
		}
		g := searchGroups[row.Mode+"/"+s.Split]
		g.Programs++
		if x.Status == "TRAINING_COMPLETE" {
			g.Complete++
		}
		g.Attempts += len(x.Attempts)
		g.ModelCalls += x.Selection.ModelCalls
		g.FeedbackCalls += feedbackCalls
		g.NS += row.NS
		searchGroups[row.Mode+"/"+s.Split] = g
		calls += x.Selection.ModelCalls
	}
	require(calls == r.SearchModelCalls && reflect.DeepEqual(searchGroups, r.SearchGroups), "search aggregates")
	return map[string]any{"status": "PASS", "producer": producer, "sources": 162, "training_rows_per_fit": 16, "judgments": len(judgments), "searches": len(searches), "search_model_calls": calls, "new_audit_model_calls": 0, "new_audit_candidate_executions": 0, "intent_diagnostics": intentDiagnostics(sources, judgments), "case_changes": baselineChanges(frozen, judgments)}
}

func auditTrace(j judgment) {
	e := j.Explanation
	require(e.ChoiceCount == 2 && e.CandidateCount == 4, "first-call diagnostic trace")
	for c := range 16 {
		for _, h := range e.Hidden[c] {
			require(!math.IsNaN(float64(h)) && !math.IsInf(float64(h), 0), "finite signed activation")
		}
		if c >= 2 {
			require(e.Hidden[c] == [24]float32{} && e.OptionScores[c] == [2]float32{}, "unused choice trace")
		}
	}
	for mask := range 64 {
		if mask >= 4 {
			require(e.CandidateScores[mask] == 0, "unused candidate trace")
			continue
		}
		want := float64(e.OptionScores[0][mask&1]) + float64(e.OptionScores[1][mask>>1&1])
		require(e.CandidateScores[mask] == want, "actual additive candidate score")
	}
	best := 0
	for i := 1; i < 4; i++ {
		if e.CandidateScores[i] > e.CandidateScores[best] {
			best = i
		}
	}
	require(j.Prediction.Selected == uint16(best), "trace scores select the recorded candidate")
}

func baselineChanges(frozen string, rows []judgment) map[string]any {
	base := filepath.Join(filepath.Dir(filepath.Dir(frozen)), "relational-flow-learning-20261010/result")
	b, e := os.ReadFile(filepath.Join(base, "judgments.jsonl.gz"))
	must(e)
	require(hash(b) == "f585f2514054113447931339b3d6e411382919303bd475c3588ee27983b0f333", "original ReLU predictions")
	old := map[string]judgment{}
	for _, j := range lines[judgment](read(base, "judgments.jsonl.gz")) {
		old[j.ID] = j
	}
	var improved, regressed []string
	unchanged := 0
	for _, j := range rows {
		previous, ok := old[j.ID]
		require(ok && previous.SourceSHA == j.SourceSHA && previous.InputSHA == j.InputSHA && previous.Acceptable == j.Acceptable, "same source and input in activation comparison")
		switch {
		case !previous.Selected.Acceptable && j.Selected.Acceptable:
			improved = append(improved, j.ID)
		case previous.Selected.Acceptable && !j.Selected.Acceptable:
			regressed = append(regressed, j.ID)
		default:
			unchanged++
		}
	}
	return map[string]any{"improved": len(improved), "regressed": len(regressed), "unchanged": unchanged, "improved_ids": improved, "regressed_ids": regressed}
}

// Recount existing records only. Different natural-language intentions can
// receive identical model distributions despite different encoded inputs.
func intentDiagnostics(sources []source, judgments []judgment) map[string]any {
	byID := map[string]source{}
	byJudgment := map[string]judgment{}
	valid := map[string]int{"max": 0, "min": 0}
	counts := map[string]int{"max": 0, "min": 0}
	for _, s := range sources {
		byID[s.ID] = s
	}
	for _, j := range judgments {
		byJudgment[j.ID] = j
		for _, intent := range []string{"max", "min"} {
			if strings.HasPrefix(j.ID, intent+"-") {
				counts[intent]++
				if j.Selected.Acceptable {
					valid[intent]++
				}
			}
		}
	}
	paired, different, same, conflicting := 0, 0, 0, 0
	directPairs, directZero, directSameHidden, directSameScores, negativeDirect := 0, 0, 0, 0, 0
	for _, j := range judgments {
		if j.Form == "direct" {
			for _, h := range j.Explanation.Hidden[1] {
				if h < 0 {
					negativeDirect++
					break
				}
			}
		}
	}
	for _, s := range sources {
		if !strings.HasPrefix(s.ID, "max-") {
			continue
		}
		otherID := "min-" + strings.TrimPrefix(s.ID, "max-")
		other, ok := byID[otherID]
		require(ok, "paired intent record")
		paired++
		if s.Form == "direct" {
			directPairs++
			a, b := byJudgment[s.ID].Explanation, byJudgment[otherID].Explanation
			if a.Hidden[1] == [24]float32{} && b.Hidden[1] == [24]float32{} {
				directZero++
			}
			if a.Hidden[1] == b.Hidden[1] {
				directSameHidden++
			}
			if a.OptionScores[1] == b.OptionScores[1] {
				directSameScores++
			}
		}
		if s.Acceptable&other.Acceptable == 0 {
			conflicting++
		}
		if !reflect.DeepEqual(s.V6, other.V6) {
			different++
			if byJudgment[s.ID].Prediction == byJudgment[otherID].Prediction {
				same++
			}
		}
	}
	return map[string]any{"rows": counts, "valid": valid, "paired_opposite_intents": paired, "disjoint_acceptable_sets": conflicting, "different_encoded_inputs": different, "different_inputs_same_prediction": same, "direct_pairs": directPairs, "direct_all_zero_branch_pairs": directZero, "direct_equal_branch_hidden_pairs": directSameHidden, "direct_equal_branch_score_pairs": directSameScores, "direct_rows_with_negative_branch_activations": negativeDirect}
}
func main() {
	require(len(os.Args) == 3, "usage: audit RESULT_DIRECTORY FROZEN_SOURCE_DIRECTORY")
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	must(e.Encode(audit(os.Args[1], os.Args[2])))
}
