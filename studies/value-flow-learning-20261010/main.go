package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"slices"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/flowdecision"
)

var variants = [2]string{"flow_off", "flow_on"}

type fitRecord struct {
	ModelSHA           string  `json:"model_sha256"`
	Fingerprint        string  `json:"model_fingerprint"`
	TrainingSHA        string  `json:"training_samples_sha256"`
	ArtifactBytes      int     `json:"artifact_bytes"`
	TrainingNS         int64   `json:"training_ns"`
	FirstLoss          float64 `json:"first_loss"`
	LastLoss           float64 `json:"last_pre_update_loss"`
	PredictionMedianNS int64   `json:"prediction_median_ns"`
	PredictionP95NS    int64   `json:"prediction_p95_ns"`
}
type report struct {
	Schema           string                  `json:"schema"`
	Producer         string                  `json:"producer_revision"`
	Go               string                  `json:"go_version"`
	OS               string                  `json:"os"`
	Arch             string                  `json:"arch"`
	InputDigests     map[string]string       `json:"input_gzip_sha256"`
	Programs         int                     `json:"programs"`
	Contexts         int                     `json:"contexts"`
	TrainingRows     int                     `json:"training_rows_per_fit"`
	TrainingCalls    int                     `json:"training_calls"`
	Parameters       int                     `json:"parameters_per_model"`
	WeightBytes      int                     `json:"weight_bytes_per_model"`
	InputBytes       int                     `json:"input_bytes_per_choice"`
	Judgments        int                     `json:"judgments"`
	Searches         int                     `json:"searches"`
	SearchModelCalls int                     `json:"search_model_calls"`
	NativeExecutions int                     `json:"native_executions"`
	Options          flowdecision.FitOptions `json:"fit_options"`
	Fits             map[string]fitRecord    `json:"fits"`
	Groups           map[string]group        `json:"groups"`
	SearchGroups     map[string]searchGroup  `json:"search_groups"`
	Scope            string                  `json:"scope"`
}

func pruned(model *flowdecision.Model) *flowdecision.Model {
	weights := model.Weights()
	for hidden := range flowdecision.HiddenDim {
		clear(weights[hidden*384+320 : (hidden+1)*384])
	}
	result, err := flowdecision.New(weights)
	must(err)
	return result
}

func main() {
	require(len(os.Args) == 4, "usage: value-flow-fit ORIGINAL_RESULT_DIRECTORY PROJECTION_RESULT_DIRECTORY NEW_OUTPUT_DIRECTORY")
	revision := producer()
	original, projected, out := os.Args[1], os.Args[2], os.Args[3]
	must(os.Mkdir(out, 0700))
	save(out, "started.json", map[string]string{"producer_revision": revision, "time": time.Now().UTC().Format(time.RFC3339)})
	sources, inputs, digests := load(original, projected)
	r := report{Schema: "gooo/value-flow-learning/v1", Producer: revision, Go: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH, InputDigests: digests, Programs: len(sources), Contexts: len(inputs), TrainingRows: 160, Parameters: flowdecision.ParameterCount, WeightBytes: flowdecision.ParameterCount * 4, InputBytes: 384 * 4,
		Options: flowdecision.FitOptions{Epochs: 400, LearningRate: 0.25, L2: 0.0001, Seed: 17}, Fits: map[string]fitRecord{}, Groups: map[string]group{}, SearchGroups: map[string]searchGroup{},
		Scope: "Two fresh 384-input CPU models on 32 training sources/160 correlated contexts. Frozen source/context/static-projection inputs; no repeated old fit or old experiment. Related wording/constants/assignment/four-region variants are limited evidence. Initial runtime features contain no unseen expected outputs. New predictions immediately precede candidate construction/evaluation. No native run, quantization, retuning, host CPU utilization or general-language claim."}
	inputFile := rowFile(out, "inputs.jsonl")
	for _, input := range inputs {
		appendRow(inputFile, input)
	}
	must(inputFile.Close())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var models [2]*flowdecision.Model
	for i, name := range variants {
		training := trainingSamples(inputs, i == 1)
		require(len(training) == 160, "only 160 training contexts")
		trainingBytes := encoded(training)
		saveRaw(out, "training-"+name+".json", trainingBytes)
		start := time.Now()
		model, history, err := flowdecision.Fit(ctx, training, r.Options)
		ns := time.Since(start).Nanoseconds()
		must(err)
		r.TrainingCalls++
		require(len(history) == 400, "fixed epochs")
		if i == 0 {
			before, err := model.Marshal()
			must(err)
			saveRaw(out, "model-flow_off-before-pruning.json", before)
			model = pruned(model)
		}
		raw, err := model.Marshal()
		must(err)
		saveRaw(out, "model-"+name+".json", raw)
		save(out, "history-"+name+".json", history)
		loaded, err := flowdecision.Decode(raw)
		must(err)
		require(loaded.Weights() == model.Weights() && loaded.Fingerprint() == model.Fingerprint(), "saved model identity")
		models[i] = loaded
		r.Fits[name] = fitRecord{ModelSHA: hash(raw), Fingerprint: loaded.Fingerprint(), TrainingSHA: hash(trainingBytes), ArtifactBytes: len(raw), TrainingNS: ns, FirstLoss: history[0].Loss, LastLoss: history[len(history)-1].Loss}
	}
	judgments := rowFile(out, "judgments.jsonl")
	for i, name := range variants {
		times := judge(ctx, name, models[i], sources, inputs, judgments, &r)
		slices.Sort(times)
		fit := r.Fits[name]
		fit.PredictionMedianNS, fit.PredictionP95NS = times[len(times)/2], times[(len(times)*95-1)/100]
		r.Fits[name] = fit
	}
	must(judgments.Close())
	searches := rowFile(out, "searches.jsonl")
	search(ctx, models, sources, searches, &r)
	must(searches.Close())
	require(r.TrainingCalls == 2 && r.Judgments == 1408 && r.Searches == 512, "complete fixed protocol")
	save(out, "report.json", r)
	save(out, "completed.json", map[string]any{"time": time.Now().UTC().Format(time.RFC3339), "training_calls": r.TrainingCalls, "judgments": r.Judgments, "searches": r.Searches, "search_model_calls": r.SearchModelCalls, "native_executions": 0})
	fmt.Printf("Completed %d fresh fits, %d immediate judgments, %d finite searches; old study untouched.\n", r.TrainingCalls, r.Judgments, r.Searches)
}
