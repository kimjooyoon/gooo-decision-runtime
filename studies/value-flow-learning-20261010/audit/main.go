// Audit frozen files only. There are no training, prediction, preparation,
// compilation, candidate evaluation or native execution calls in this command.
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
	"strconv"
	"strings"

	"github.com/kimjooyoon/gooo-decision-runtime/flowdecision"
)

var variants = [2]string{"flow_off", "flow_on"}

func require(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
func must(err error)         { require(err == nil, fmt.Sprint(err)) }
func hash(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func encoded(v any) []byte   { b, e := json.Marshal(v); must(e); return b }
func read(dir, name string) []byte {
	b, e := os.ReadFile(filepath.Join(dir, name))
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
func decode[T any](b []byte) T { var v T; must(json.Unmarshal(b, &v)); return v }
func lines[T any](b []byte) []T {
	var rows []T
	for line := range bytes.SplitSeq(b, []byte{'\n'}) {
		if len(line) > 0 {
			rows = append(rows, decode[T](line))
		}
	}
	return rows
}
func featureBytes(f [384]float32) []byte {
	b := make([]byte, 1536)
	for i, x := range f {
		require(!math.IsNaN(float64(x)) && !math.IsInf(float64(x), 0), "finite features")
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(x))
	}
	return b
}
func featureSHA(inputs [][384]float32) string {
	b := []byte{byte(len(inputs))}
	for _, input := range inputs {
		b = append(b, featureBytes(input)...)
	}
	return hash(b)
}

func inputChecks(dir, original, projected string, r report) ([]inputRecord, map[string]sourceRecord, map[string][]candidate) {
	for name, want := range r.InputDigests {
		root, file := original, name
		if name == "projection-report.json.gz" {
			root, file = projected, "report.json.gz"
		}
		b, e := os.ReadFile(filepath.Join(root, file))
		must(e)
		require(hash(b) == want, "frozen input digest")
	}
	require(len(r.InputDigests) == 3, "three frozen inputs")
	dataset := decode[struct {
		Records []sourceRecord `json:"records"`
	}](read(original, "dataset.json.gz"))
	projection := decode[struct {
		Rows []projectionRow `json:"rows"`
	}](read(projected, "report.json.gz"))
	byID := map[string]sourceRecord{}
	for _, s := range dataset.Records {
		_, exists := byID[s.ID]
		require(!exists && hash([]byte(s.Source)) == s.SHA, "unique source identity")
		byID[s.ID] = s
	}
	byProjection := map[string]projectionRow{}
	for _, p := range projection.Rows {
		s, ok := byID[p.ID]
		require(ok && s.SHA == p.SourceSHA && s.Split == p.Split && hash(encoded(s.Document.Plan)) == p.PlanSHA, "serialized plan identity")
		byProjection[p.ID] = p
	}
	originalContexts := lines[frozenContext](read(original, "contexts.jsonl.gz"))
	inputs := lines[inputRecord](read(dir, "inputs.jsonl.gz"))
	require(len(inputs) == 704 && len(originalContexts) == 704 && len(byID) == 128 && len(byProjection) == 128, "full original collection")
	seen := map[string]bool{}
	for i, input := range inputs {
		f := originalContexts[i]
		p := byProjection[input.ID]
		key := input.ID + "/" + input.Context
		require(!seen[key] && input.ID == f.ID && input.Split == f.Split && input.Context == f.Context && input.Acceptable == f.Acceptable && reflect.DeepEqual(input.Masks, f.Masks), "original ordered context/labels")
		seen[key] = true
		require(len(input.Features) == len(f.Features[2]) && len(input.Features) == len(p.Features), "choice count")
		for j, features := range input.Features {
			require(slices.Equal(features[:320], f.Features[2][j][:]) && slices.Equal(features[320:], p.Features[j][320:]), "unchanged original prefix and static suffix")
			if input.Context == "initial" {
				require(features == p.Features[j], "original initial features")
			}
		}
		require(input.SHA == featureSHA(input.Features), "complete input digest")
	}
	programs := lines[struct {
		ID         string      `json:"id"`
		Candidates []candidate `json:"candidates"`
	}](read(original, "candidates.jsonl.gz"))
	candidates := map[string][]candidate{}
	for _, p := range programs {
		candidates[p.ID] = p.Candidates
	}
	require(len(candidates) == 128, "all frozen candidate observations")
	return inputs, byID, candidates
}

func modelChecks(dir string, r report, inputs []inputRecord) {
	for _, name := range variants {
		f := r.Fits[name]
		raw := read(dir, "model-"+name+".json")
		m, e := flowdecision.Decode(raw)
		must(e)
		require(hash(raw) == f.ModelSHA && m.Fingerprint() == f.Fingerprint && len(raw) == f.ArtifactBytes, "artifact identity")
		trainingRaw := read(dir, "training-"+name+".json.gz")
		training := decode[[]flowdecision.Sample](trainingRaw)
		require(hash(trainingRaw) == f.TrainingSHA && len(training) == 160, "original training bytes")
		i := 0
		for _, input := range inputs {
			if input.Split != "train" {
				continue
			}
			want := flowdecision.Sample{Inputs: slices.Clone(input.Features), Masks: input.Masks, Acceptable: input.Acceptable}
			if name == "flow_off" {
				for j := range want.Inputs {
					clear(want.Inputs[j][320:])
				}
			}
			require(reflect.DeepEqual(want, training[i]), "exact train-only sample projection")
			i++
		}
		require(i == 160, "complete fixed training set")
		history := decode[[]flowdecision.Epoch](read(dir, "history-"+name+".json.gz"))
		require(len(history) == 400 && history[0].Loss == f.FirstLoss && history[399].Loss == f.LastLoss && f.TrainingNS > 0, "fit history and duration")
		for i, h := range history {
			require(h.Number == i+1 && !math.IsNaN(h.Loss) && !math.IsInf(h.Loss, 0), "finite numbered epochs")
		}
		if name == "flow_off" {
			before, e := flowdecision.Decode(read(dir, "model-flow_off-before-pruning.json"))
			must(e)
			old, after := before.Weights(), m.Weights()
			for j, w := range after {
				want := old[j]
				if j < 384*24 && j%384 >= 320 {
					want = 0
				}
				require(w == want, "only unused static weights pruned")
			}
		}
	}
}

func judgmentChecks(dir string, r report, inputs []inputRecord, sources map[string]sourceRecord, candidates map[string][]candidate) {
	byContext := map[string]inputRecord{}
	for _, input := range inputs {
		byContext[input.ID+"/"+input.Context] = input
	}
	rows := lines[judgment](read(dir, "judgments.jsonl.gz"))
	require(len(rows) == 1408, "all immediate judgments")
	groups := map[string]group{}
	seen := map[string]bool{}
	times := map[string][]int64{}
	for _, row := range rows {
		input, ok := byContext[row.ID+"/"+row.Context]
		s := sources[row.ID]
		require(ok && row.Split == input.Split && row.SourceSHA == s.SHA && row.FeatureSHA == input.SHA && row.Acceptable == input.Acceptable, "judgment binding")
		key := row.Model + "/" + row.ID + "/" + row.Context
		require(!seen[key] && slices.Contains(variants[:], row.Model), "unique known model judgment")
		seen[key] = true
		p := row.Prediction
		require(p.Count == len(input.Masks) && int(p.Selected) < p.Count && row.PredictNS >= 0, "bounded recorded prediction")
		var sum float64
		for i, value := range p.Probabilities {
			require(!math.IsNaN(float64(value)) && value >= 0 && value <= 1, "valid probability")
			if i < p.Count {
				sum += float64(value)
			} else {
				require(value == 0, "unused probabilities zero")
			}
		}
		require(math.Abs(sum-1) < 1e-5, "probability mass")
		require(reflect.DeepEqual(row.Selected, candidates[row.ID][p.Selected]), "new actual candidate equals original typed case observations")
		require(row.Selected.Accepted == (row.Acceptable>>p.Selected&1 != 0), "complete acceptable mask")
		kind := "observed"
		if row.Context == "initial" {
			kind = "initial"
		}
		k := row.Model + "/" + row.Split + "/" + kind
		g := groups[k]
		g.Rows++
		if row.Selected.Accepted {
			g.Valid++
		}
		g.OutputPassed += row.Selected.OutputPassed
		g.OutputTotal += len(s.Document.TestCases)
		g.ConditionPassed += row.Selected.ConditionPassed
		g.ConditionTotal += len(s.Document.Plan.ConditionCases)
		groups[k] = g
		times[row.Model] = append(times[row.Model], row.PredictNS)
	}
	require(reflect.DeepEqual(groups, r.Groups), "recount judgment groups")
	for name, values := range times {
		slices.Sort(values)
		f := r.Fits[name]
		require(f.PredictionMedianNS == values[len(values)/2] && f.PredictionP95NS == values[(len(values)*95-1)/100], "prediction time quantiles")
	}
}

// The search hashes the complete fixed declaration wrapper, while the frozen
// candidate record stores its body. Rebuild only that serialization for hashing.
func sourceSHA(name, body string) string {
	return hash([]byte("package bodyplan\nnamespace bodyplan\n\n" +
		"entity Integer id \"bodyplan://entity/integer\"\n" +
		"entity Boolean id \"bodyplan://entity/boolean\"\n\n" +
		"activity " + name + "(Integer) -> Integer computes " + strconv.Quote(strings.TrimSuffix(body, "\n")) + "\n"))
}

func searchChecks(dir string, r report, inputs []inputRecord, sources map[string]sourceRecord, candidates map[string][]candidate) map[string]int {
	initial := map[string]inputRecord{}
	for _, input := range inputs {
		if input.Context == "initial" {
			initial[input.ID] = input
		}
	}
	rows := lines[searchRow](read(dir, "searches.jsonl.gz"))
	require(len(rows) == 512, "all new searches")
	groups := map[string]searchGroup{}
	seen := map[string]bool{}
	attemptsBySource := map[string]int{}
	totalCalls := 0
	for _, row := range rows {
		key := row.Mode + "/" + row.ID
		require(!seen[key], "unique search")
		seen[key] = true
		name := strings.TrimSuffix(strings.TrimSuffix(row.Mode, "_initial"), "_feedback")
		require(slices.Contains(variants[:], name) && (row.Mode == name+"_initial" || row.Mode == name+"_feedback"), "known search mode")
		in := initial[row.ID]
		require(row.Split == in.Split && len(row.Progress) > 0, "source-bound search")
		first := row.Progress[0].Ranking
		require(first.ModelFingerprint == r.Fits[name].Fingerprint && first.Calls == 1 && first.Attempted == 0, "initial model receipt")
		for i, input := range in.Features {
			require(first.FeatureSHA[i] == hash(featureBytes(input)), "actual first input bytes")
		}
		for _, p := range row.Progress {
			sha := p.SHA
			p.SHA = ""
			require(hash(encoded(p)) == sha, "progress receipt digest")
			q := p.Ranking
			sha = q.SHA
			q.SHA = ""
			require(hash(encoded(q)) == sha, "ranking receipt digest")
		}
		masks := map[uint16]bool{}
		for _, a := range row.Result.Attempts {
			require(!masks[a.Mask] && int(a.Mask) < len(in.Masks), "finite unique mask")
			masks[a.Mask] = true
			want := candidates[row.ID][a.Mask]
			require(a.Passed == want.OutputPassed && a.Total == len(want.Cases) && reflect.DeepEqual(a.Choices, want.Choices) && reflect.DeepEqual(a.Results, want.Cases) && reflect.DeepEqual(a.Conditions, want.Conditions) && a.GoooSHA == sourceSHA(sources[row.ID].Document.Plan.Base.Name, want.GoooBody), "actual search candidate evidence")
		}
		calls, feedbackCalls, reused := first.Calls, 0, 0
		for _, ranking := range row.Feedback {
			calls += ranking.Calls
			feedbackCalls += ranking.Calls
			if ranking.Reused {
				reused++
			}
		}
		require(calls == row.Result.Selection.ModelCalls, "actual call accounting")
		if row.Mode == name+"_initial" {
			require(len(row.Feedback) == 0, "initial-only mode")
		}
		if row.Result.Status == "TRAINING_COMPLETE" {
			found := false
			for _, c := range candidates[row.ID] {
				if reflect.DeepEqual(c.Choices, row.Result.Selection.Choices) {
					require(c.Accepted && row.SelectedBody == c.GoooBody, "selected complete body")
					found = true
				}
			}
			require(found && row.Error == "", "saved complete source")
		}
		gkey := row.Mode + "/" + row.Split
		g := groups[gkey]
		g.Programs++
		if row.Result.Status == "TRAINING_COMPLETE" {
			g.Complete++
		}
		g.Attempts += len(row.Result.Attempts)
		g.ModelCalls += calls
		g.FeedbackCalls += feedbackCalls
		g.Reused += reused
		g.SearchNS += row.SearchNS
		groups[gkey] = g
		totalCalls += calls
		attemptsBySource[key] = len(row.Result.Attempts)
	}
	require(reflect.DeepEqual(groups, r.SearchGroups) && totalCalls == r.SearchModelCalls, "recount all search groups")
	comparison := map[string]int{}
	for id := range initial {
		a, b := attemptsBySource["flow_off_initial/"+id], attemptsBySource["flow_on_initial/"+id]
		kind := "same"
		if b < a {
			kind = "improved"
		} else if b > a {
			kind = "worse"
		}
		comparison["on_vs_off_initial_"+kind]++
		for _, name := range variants {
			a, b := attemptsBySource[name+"_initial/"+id], attemptsBySource[name+"_feedback/"+id]
			kind := "same"
			if b < a {
				kind = "improved"
			} else if b > a {
				kind = "worse"
			}
			comparison[name+"_feedback_"+kind]++
		}
	}
	return comparison
}

func main() {
	require(len(os.Args) == 4, "usage: audit RESULT_DIRECTORY ORIGINAL_RESULT_DIRECTORY PROJECTION_RESULT_DIRECTORY")
	dir, original, projected := os.Args[1], os.Args[2], os.Args[3]
	r := decode[report](read(dir, "report.json"))
	require(r.Schema == "gooo/value-flow-learning/v1" && r.Producer == "2db75199ef23c76ab1a368290a403523e2dfc3a4" && r.Go == "go1.27.2", "producer identity")
	require(r.Options == (flowdecision.FitOptions{Epochs: 400, LearningRate: 0.25, L2: 0.0001, Seed: 17}) && r.TrainingCalls == 2 && r.TrainingRows == 160 && r.Programs == 128 && r.Contexts == 704 && r.Judgments == 1408 && r.Searches == 512 && r.NativeExecutions == 0, "fixed study bounds")
	require(r.Parameters == 9290 && r.WeightBytes == 37160 && r.InputBytes == 1536, "model dimensions")
	inputs, sources, candidates := inputChecks(dir, original, projected, r)
	modelChecks(dir, r, inputs)
	judgmentChecks(dir, r, inputs, sources, candidates)
	comparison := searchChecks(dir, r, inputs, sources, candidates)
	fmt.Println(string(encoded(map[string]any{"sources": 128, "contexts": 704, "judgments": 1408, "searches": 512, "search_model_calls": r.SearchModelCalls, "paired_attempt_comparison": comparison, "new_model_calls": 0, "new_candidate_evaluations": 0, "new_fits": 0})))
}
