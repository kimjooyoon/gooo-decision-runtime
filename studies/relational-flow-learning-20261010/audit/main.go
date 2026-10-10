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

const producer = "3dfcb1701bba1020d4002025e2c3dd163b534359"
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
	require(r.Schema == "gooo/relational-flow-learning/v1" && r.Producer == producer && r.DatasetSHA == dataset && r.Sources == 162 && r.TrainingRows == 16 && r.Fits == 1 && r.Judgments == 162 && r.Searches == 324 && r.NativeExecutions == 0, "fixed protocol counts")
	require(r.BaselineReportSHA == "83ab9f2b87845e519e9f7d0d6bbc474d2dc87fcf568df91be972460412e9f4ac" && hash(read(root, "baseline-report.json")) == r.BaselineReportSHA, "immutable historical baseline")
	require(r.Parameters == 9290 && r.WeightBytes == 37160 && r.Options == (flowdecision.FitOptions{Epochs: 400, LearningRate: .25, L2: .0001, Seed: 17}), "fixed model options")
	require(len(r.Models) == 1, "only one fresh fit")
	require(r.Models["v6"].FeatureVersion == decision.RelationalFlowFeatureVersion, "explicit v6 artifact")
	for _, name := range []string{"v6"} {
		fit := r.Models[name]
		b := read(root, "model-"+name+".json")
		m, e := flowdecision.Decode(b)
		must(e)
		require(hash(b) == fit.ModelSHA && len(b) == fit.ArtifactBytes && m.Fingerprint() == fit.Fingerprint && m.FeatureVersion() == fit.FeatureVersion && fit.TrainingNS > 0, "model artifact")
		b = read(root, "training-"+name+".json.gz")
		require(hash(b) == fit.TrainingSHA, "training bytes")
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
		require(!seen[key] && j.Model == "v6", "unique model/source")
		seen[key] = true
		want := featureSHA(s.V6)
		require(j.InputSHA == want && j.NS > 0, "initial model input")
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
	for _, name := range []string{"v6"} {
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
		require(slices.Contains([]string{"v6_initial", "v6_feedback"}, row.Mode), "search mode")
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
			name := "v6"
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
	return map[string]any{"status": "PASS", "producer": producer, "sources": 162, "training_rows_per_fit": 16, "judgments": len(judgments), "searches": len(searches), "search_model_calls": calls, "new_audit_model_calls": 0, "new_audit_candidate_executions": 0, "intent_diagnostics": intentDiagnostics(sources, judgments)}
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
	for _, s := range sources {
		if !strings.HasPrefix(s.ID, "max-") {
			continue
		}
		otherID := "min-" + strings.TrimPrefix(s.ID, "max-")
		other, ok := byID[otherID]
		require(ok, "paired intent record")
		paired++
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
	return map[string]any{"rows": counts, "valid": valid, "paired_opposite_intents": paired, "disjoint_acceptable_sets": conflicting, "different_encoded_inputs": different, "different_inputs_same_prediction": same}
}
func main() {
	require(len(os.Args) == 3, "usage: audit RESULT_DIRECTORY FROZEN_SOURCE_DIRECTORY")
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	must(e.Encode(audit(os.Args[1], os.Args[2])))
}
