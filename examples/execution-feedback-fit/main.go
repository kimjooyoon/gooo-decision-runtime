package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"slices"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/conditiondecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

type group struct {
	Rows            int `json:"rows"`
	Valid           int `json:"valid"`
	FallbackValid   int `json:"fallback_valid"`
	OutputPassed    int `json:"output_passed"`
	OutputTotal     int `json:"output_total"`
	ConditionPassed int `json:"condition_passed"`
	ConditionTotal  int `json:"condition_total"`
}
type fitRecord struct {
	Version            string  `json:"feature_version"`
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
type judgment struct {
	ID         string                       `json:"id"`
	Split      string                       `json:"split"`
	Context    string                       `json:"context"`
	Model      string                       `json:"model"`
	Version    string                       `json:"feature_version"`
	SourceSHA  string                       `json:"source_sha256"`
	FeatureSHA string                       `json:"feature_sha256"`
	Acceptable uint64                       `json:"acceptable_candidate_bits"`
	Prediction conditiondecision.Prediction `json:"prediction"`
	PredictNS  int64                        `json:"predict_ns"`
	Selected   candidate                    `json:"selected"`
}
type searchRow struct {
	ID           string                       `json:"id"`
	Split        string                       `json:"split"`
	Mode         string                       `json:"mode"`
	SearchNS     int64                        `json:"search_ns"`
	Result       pathplan.SearchResult        `json:"result"`
	Progress     []pathplan.ConditionProgress `json:"progress"`
	Feedback     []pathplan.ConditionRanking  `json:"feedback"`
	SelectedBody string                       `json:"selected_body"`
	Error        string                       `json:"error,omitempty"`
}
type searchGroup struct {
	Programs      int   `json:"programs"`
	Complete      int   `json:"complete"`
	Attempts      int   `json:"attempts"`
	ModelCalls    int   `json:"model_calls"`
	FeedbackCalls int   `json:"feedback_calls"`
	Reused        int   `json:"reused"`
	SearchNS      int64 `json:"search_ns"`
}
type report struct {
	Schema                  string                       `json:"schema"`
	Producer                string                       `json:"producer_revision"`
	Compiler                string                       `json:"compiler_revision"`
	Go                      string                       `json:"go_version"`
	DatasetSHA              string                       `json:"dataset_sha256"`
	Programs                int                          `json:"programs"`
	Contexts                int                          `json:"contexts"`
	TrainingRows            int                          `json:"training_rows_per_fit"`
	TrainingCalls           int                          `json:"training_calls"`
	Parameters              map[string]int               `json:"parameters_per_model"`
	WeightBytes             map[string]int               `json:"weight_bytes_per_model"`
	InputBytes              map[string]int               `json:"input_bytes_per_choice"`
	Judgments               int                          `json:"judgments"`
	Searches                int                          `json:"searches"`
	SearchModelCalls        int                          `json:"search_model_calls"`
	NativeExecutions        int                          `json:"native_executions"`
	Options                 conditiondecision.FitOptions `json:"fit_options"`
	Fits                    map[string]fitRecord         `json:"fits"`
	Groups                  map[string]group             `json:"groups"`
	SearchGroups            map[string]searchGroup       `json:"search_groups"`
	ConflictingInitialPairs map[string]int               `json:"conflicting_initial_pairs"`
	Scope                   string                       `json:"scope"`
}

func rank(ctx context.Context, model fittedModel, version int, programs []preparedRecord, rows []inputRecord, f *os.File, r *report) ([]int64, error) {
	byID := map[string]preparedRecord{}
	for _, p := range programs {
		byID[p.source.ID] = p
	}
	var times []int64
	for _, input := range rows {
		p := byID[input.ID]
		var prediction conditiondecision.Prediction
		start := time.Now()
		err := model.predict(input.Features[version], input.Masks, &prediction)
		elapsed := time.Since(start).Nanoseconds()
		if err != nil {
			return times, err
		}
		// Assemble and execute the selected body immediately after this prediction.
		selected, err := evaluate(ctx, p.plan, p.source.Document, prediction.Selected)
		if err != nil {
			return times, err
		}
		expected := p.candidates[prediction.Selected]
		if selected.GoSHA != expected.GoSHA || selected.Accepted != expected.Accepted || selected.OutputPassed != expected.OutputPassed || selected.ConditionPassed != expected.ConditionPassed {
			return times, fmt.Errorf("selected candidate replay differs")
		}
		j := judgment{ID: input.ID, Split: input.Split, Context: input.Context, Model: versions[version], Version: featureVersions[version], SourceSHA: p.source.SHA, FeatureSHA: input.FeatureSHA[version], Acceptable: input.Acceptable, Prediction: prediction, PredictNS: elapsed, Selected: selected}
		if err := appendRow(f, j); err != nil {
			return times, err
		}
		kind := "observed"
		if input.Context == "initial" {
			kind = "initial"
		}
		key := versions[version] + "/" + input.Split + "/" + kind
		g := r.Groups[key]
		g.Rows++
		if selected.Accepted {
			g.Valid++
		}
		if input.Acceptable&1 != 0 {
			g.FallbackValid++
		}
		g.OutputPassed += selected.OutputPassed
		g.OutputTotal += len(p.source.Document.TestCases)
		g.ConditionPassed += selected.ConditionPassed
		g.ConditionTotal += len(p.source.Document.Plan.ConditionCases)
		r.Groups[key] = g
		r.Judgments++
		times = append(times, elapsed)
	}
	return times, nil
}

func search(ctx context.Context, models [3]fittedModel, programs []preparedRecord, f *os.File, r *report) error {
	for _, p := range programs {
		for mode := range 7 {
			name := "deterministic"
			var model fittedModel
			rounds := 0
			if mode > 0 {
				v := (mode - 1) / 2
				model = models[v]
				name = versions[v] + "_initial"
				if mode%2 == 0 {
					name = versions[v] + "_feedback"
					rounds = len(p.candidates) - 1
				}
			}
			start := time.Now()
			result, body, progress, feedback, failure := model.search(ctx, p.plan, p.source.Document.TestCases, len(p.candidates), rounds)
			row := searchRow{ID: p.source.ID, Split: p.source.Split, Mode: name, SearchNS: time.Since(start).Nanoseconds(), Result: result, Progress: progress, Feedback: feedback}
			if failure != nil {
				row.Error = failure.Error()
			}
			if body != nil {
				row.SelectedBody = body.GoooBody()
			}
			if err := appendRow(f, row); err != nil {
				return err
			}
			key := name + "/" + p.source.Split
			g := r.SearchGroups[key]
			g.Programs++
			if result.Status == "TRAINING_COMPLETE" {
				g.Complete++
			}
			g.Attempts += len(result.Attempts)
			g.ModelCalls += result.Selection.ModelCalls
			g.SearchNS += row.SearchNS
			for _, feedback := range feedback {
				g.FeedbackCalls += feedback.Calls
				if feedback.Reused {
					g.Reused++
				}
			}
			r.SearchGroups[key] = g
			r.Searches++
			r.SearchModelCalls += result.Selection.ModelCalls
		}
	}
	return nil
}

func conflicts(rows []inputRecord, version int) int {
	count := 0
	for i, a := range rows {
		if a.Context != "initial" {
			continue
		}
		for _, b := range rows[:i] {
			if b.Context == "initial" && a.ID != b.ID && a.FeatureSHA[version] == b.FeatureSHA[version] && a.Acceptable&b.Acceptable == 0 {
				count++
			}
		}
	}
	return count
}

func run(path, out string) error {
	producer, err := identity()
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	d, err := validateDataset(raw, producer)
	if err != nil {
		return err
	}
	if err := os.Mkdir(out, 0700); err != nil {
		return err
	}
	if err := save(out, "started.json", map[string]string{"producer_revision": producer, "dataset_sha256": hash(raw)}); err != nil {
		return err
	}
	sourceFile, err := rowFile(out, "candidates.jsonl")
	if err != nil {
		return err
	}
	defer sourceFile.Close()
	contextFile, err := rowFile(out, "contexts.jsonl")
	if err != nil {
		return err
	}
	defer contextFile.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	programs, rows, err := prepare(ctx, d, sourceFile, contextFile)
	if err != nil {
		return err
	}
	if err := errors.Join(sourceFile.Close(), contextFile.Close()); err != nil {
		return err
	}
	if len(rows) != 704 || len(trainingRows(rows)) != 160 {
		return fmt.Errorf("fixed context counts differ")
	}
	r := report{Schema: "gooo/execution-feedback-learning-study/v1", Producer: producer, Compiler: d.Compiler, Go: runtime.Version(), DatasetSHA: hash(raw), Programs: 128, Contexts: len(rows), TrainingRows: 160,
		Parameters: map[string]int{"v2": 6218, "v3_output_off": 7754, "v3_output_on": 7754}, WeightBytes: map[string]int{"v2": 24872, "v3_output_off": 31016, "v3_output_on": 31016}, InputBytes: map[string]int{"v2": 1024, "v3_output_off": 1280, "v3_output_on": 1280},
		Options: conditiondecision.FitOptions{Epochs: 400, LearningRate: 0.25, L2: 0.0001, Seed: 17}, Fits: map[string]fitRecord{}, Groups: map[string]group{}, SearchGroups: map[string]searchGroup{}, ConflictingInitialPairs: map[string]int{},
		Scope: "128 source variants; 32 training sources/160 correlated training contexts only. Held wording, constants, assignment structure and four-region classification. Finite source cases visible to searches; no broad language accuracy or host utilization claim. Three fixed fresh CPU fits; output-off uses zero training tail and fixed removal of unused tail weights after fitting. No post-evaluation retuning. Native executions zero."}
	var models [3]fittedModel
	for v, version := range versions {
		training := modelSamples(rows, v)
		trainingRaw, err := jsonBytes(training)
		if err != nil {
			return err
		}
		start := time.Now()
		model, history, fitErr := fitModel(ctx, rows, v, r.Options)
		elapsed := time.Since(start).Nanoseconds()
		r.TrainingCalls++
		if err := save(out, "history-"+version+".json", history); err != nil {
			return err
		}
		if fitErr != nil {
			return fitErr
		}
		artifact, err := model.marshal()
		if err != nil {
			return err
		}
		loaded, err := decodeModel(artifact, v)
		if err != nil || loaded.fingerprint() != model.fingerprint() {
			return fmt.Errorf("model round trip differs: %v", err)
		}
		if err := saveRaw(out, "model-"+version+".json", artifact); err != nil {
			return err
		}
		if len(model.beforeAblation) > 0 {
			if err := saveRaw(out, "model-v3_output_off-before-pruning.json", model.beforeAblation); err != nil {
				return err
			}
		}
		models[v] = loaded
		r.Fits[version] = fitRecord{Version: featureVersions[v], ModelSHA: hash(artifact), Fingerprint: loaded.fingerprint(), TrainingSHA: hash(trainingRaw), ArtifactBytes: len(artifact), TrainingNS: elapsed, FirstLoss: history[0].Loss, LastLoss: history[len(history)-1].Loss}
		r.ConflictingInitialPairs[version] = conflicts(rows, v)
	}
	judgments, err := rowFile(out, "judgments.jsonl")
	if err != nil {
		return err
	}
	defer judgments.Close()
	for v, model := range models {
		times, err := rank(ctx, model, v, programs, rows, judgments, &r)
		if err != nil {
			return err
		}
		slices.Sort(times)
		fit := r.Fits[versions[v]]
		fit.PredictionMedianNS = times[len(times)/2]
		fit.PredictionP95NS = times[(len(times)*95-1)/100]
		r.Fits[versions[v]] = fit
	}
	if err := judgments.Close(); err != nil {
		return err
	}
	searches, err := rowFile(out, "searches.jsonl")
	if err != nil {
		return err
	}
	defer searches.Close()
	if err := search(ctx, models, programs, searches, &r); err != nil {
		return err
	}
	if err := searches.Close(); err != nil {
		return err
	}
	if err := save(out, "report.json", r); err != nil {
		return err
	}
	if err := save(out, "completed.json", map[string]int{"training_calls": r.TrainingCalls, "judgments": r.Judgments, "searches": r.Searches, "search_model_calls": r.SearchModelCalls, "native_executions": 0}); err != nil {
		return err
	}
	fmt.Printf("Completed %d fixed fits, %d judgments and %d finite searches.\n", r.TrainingCalls, r.Judgments, r.Searches)
	return nil
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: execution-feedback-fit DATASET FRESH_OUTPUT_DIRECTORY")
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
