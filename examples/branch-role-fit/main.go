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
	Parameters              int                          `json:"parameters_per_model"`
	WeightBytes             int                          `json:"weight_bytes_per_model"`
	InputBytes              int                          `json:"input_bytes_per_choice"`
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

func rank(ctx context.Context, model *conditiondecision.Model, version int, programs []preparedRecord, rows []inputRecord, f *os.File, r *report) ([]int64, error) {
	byID := map[string]preparedRecord{}
	for _, p := range programs {
		byID[p.source.ID] = p
	}
	var times []int64
	var work conditiondecision.Workspace
	for _, input := range rows {
		p := byID[input.ID]
		var prediction conditiondecision.Prediction
		start := time.Now()
		err := model.PredictInto(input.Features[version], input.Masks, &work, &prediction)
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
		j := judgment{input.ID, input.Split, input.Context, versions[version], p.source.SHA, input.FeatureSHA[version], input.Acceptable, prediction, elapsed, selected}
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

func search(ctx context.Context, models [2]*conditiondecision.Model, programs []preparedRecord, f *os.File, r *report) error {
	for _, p := range programs {
		for mode := range 5 {
			name := "deterministic"
			var model *conditiondecision.Model
			rounds := 0
			if mode > 0 {
				v := (mode - 1) / 2
				model = models[v]
				name = fmt.Sprintf("v%d_initial", v+1)
				if mode%2 == 0 {
					name = fmt.Sprintf("v%d_feedback", v+1)
					rounds = len(p.candidates) - 1
				}
			}
			start := time.Now()
			result, body, progress, feedback, failure := p.plan.SearchConditionBatches(ctx, model, p.source.Document.TestCases, len(p.candidates), 1, "", rounds)
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
	if len(rows) != 292 || len(samples(rows, 0)) != 40 {
		return fmt.Errorf("fixed context counts differ")
	}
	r := report{Schema: "gooo/branch-role-learning-study/v1", Producer: producer, Compiler: d.Compiler, Go: runtime.Version(), DatasetSHA: hash(raw), Programs: 60, Contexts: len(rows), TrainingRows: 40,
		Parameters: conditiondecision.ParameterCount, WeightBytes: conditiondecision.ParameterCount * 4, InputBytes: conditiondecision.FeatureDim * 4,
		Options: conditiondecision.FitOptions{Epochs: 400, LearningRate: 0.25, L2: 0.0001, Seed: 17}, Fits: map[string]fitRecord{}, Groups: map[string]group{}, SearchGroups: map[string]searchGroup{}, ConflictingInitialPairs: map[string]int{},
		Scope: "60 authored variants and correlated contexts; max/min-only training; new wording, local returns, nested clamp and literal-only challenges are separate. Finite source cases are visible to search. Two fixed paired CPU fits, no retuning, no independent language accuracy, calibrated probability, native execution or host CPU utilization claim."}
	var models [2]*conditiondecision.Model
	for v, version := range versions {
		training := samples(rows, v)
		trainingRaw, err := jsonBytes(training)
		if err != nil {
			return err
		}
		start := time.Now()
		model, history, fitErr := conditiondecision.FitForFeatures(ctx, training, r.Options, version)
		elapsed := time.Since(start).Nanoseconds()
		r.TrainingCalls++
		if err := save(out, fmt.Sprintf("history-v%d.json", v+1), history); err != nil {
			return err
		}
		if fitErr != nil {
			return fitErr
		}
		artifact, err := model.Marshal()
		if err != nil {
			return err
		}
		loaded, err := conditiondecision.Decode(artifact)
		if err != nil || loaded.Fingerprint() != model.Fingerprint() {
			return fmt.Errorf("model artifact round trip differs: %v", err)
		}
		if err := saveRaw(out, fmt.Sprintf("model-v%d.json", v+1), artifact); err != nil {
			return err
		}
		models[v] = loaded
		r.Fits[version] = fitRecord{Version: version, ModelSHA: hash(artifact), Fingerprint: loaded.Fingerprint(), TrainingSHA: hash(trainingRaw), ArtifactBytes: len(artifact), TrainingNS: elapsed, FirstLoss: history[0].Loss, LastLoss: history[len(history)-1].Loss}
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
		fmt.Fprintln(os.Stderr, "usage: branch-role-fit DATASET FRESH_OUTPUT_DIRECTORY")
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
