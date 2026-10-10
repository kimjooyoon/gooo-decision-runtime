// condition-fit trains one fixed CPU experiment from compiler-exported Gooo plans.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/conditiondecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

type sourceRecord struct {
	ID       string            `json:"id"`
	Split    string            `json:"split"`
	Source   string            `json:"gooo_source"`
	SHA      string            `json:"source_sha256"`
	Document pathplan.Document `json:"document"`
}
type outcome struct {
	Mask                                                       uint16
	OutputPassed, OutputTotal, ConditionPassed, ConditionTotal int
}
type row struct {
	Record   sourceRecord
	Context  string
	PlanSHA  string
	Sample   conditiondecision.Sample
	Outcomes []outcome
}
type result struct {
	ID, Split, Context, SourceSHA, PlanSHA string
	Mask                                   uint16
	Acceptable                             uint64
	Valid, BaselineValid                   bool
	Passed                                 outcome
	ValidMass                              float64
	PredictionNS                           int64
	SelectedGooo, SelectedGoSHA            string
}
type group struct{ Rows, Valid, BaselineValid, OutputsPassed, OutputsTotal, ConditionsPassed, ConditionsTotal int }

func digest(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }

func prepare(ctx context.Context, records []sourceRecord) ([]row, error) {
	var rows []row
	for _, record := range records {
		if digest([]byte(record.Source)) != record.SHA {
			return nil, fmt.Errorf("source digest differs: %s", record.ID)
		}
		doc := record.Document
		p, err := doc.Prepare()
		if err != nil {
			return nil, err
		}
		if len(doc.Plan.Decisions) > 6 {
			return nil, fmt.Errorf("study enumerates at most64 masks")
		}
		masks := make([]uint16, 1<<len(doc.Plan.Decisions))
		outcomes := make([]outcome, len(masks))
		choices := make([]map[string]string, len(masks))
		var acceptable uint64
		for i := range masks {
			mask := uint16(i)
			masks[i] = mask
			selected := make(map[string]string, len(doc.Plan.Decisions))
			for c, choice := range doc.Plan.Decisions {
				selected[choice.ID] = choice.Options[mask>>c&1].Label
			}
			choices[i] = selected
			o := outcome{Mask: mask, OutputTotal: len(doc.TestCases), ConditionTotal: len(doc.Plan.ConditionCases)}
			program, err := p.Compile(selected)
			if err == nil {
				conditions, err := p.CheckDeclaredConditions(ctx, selected)
				if err != nil {
					return nil, err
				}
				for _, condition := range conditions {
					if condition.Passed {
						o.ConditionPassed++
					}
				}
				for _, test := range doc.TestCases {
					value, err := program.Evaluate(test.Input)
					if err != nil {
						return nil, err
					}
					if value.Int == test.Expected {
						o.OutputPassed++
					}
				}
				if o.OutputPassed == o.OutputTotal && o.ConditionPassed == o.ConditionTotal {
					acceptable |= 1 << i
				}
			}
			outcomes[i] = o
		}
		if acceptable == 0 {
			return nil, fmt.Errorf("no source-valid candidate for %s", record.ID)
		}
		for observation := -1; observation < len(masks); observation++ {
			label := "initial"
			input, err := p.InitialConditionInput()
			if observation >= 0 {
				label = fmt.Sprintf("observed-%d", observation)
				input, err = p.ObserveConditionInput(ctx, choices[observation])
			}
			if err != nil {
				return nil, err
			}
			features := make([][conditiondecision.FeatureDim]float32, len(doc.Plan.Decisions))
			for c, choice := range doc.Plan.Decisions {
				if err := input.FeaturesInto(choice.ID, &features[c]); err != nil {
					return nil, err
				}
			}
			rows = append(rows, row{record, label, p.PlanSHA256(), conditiondecision.Sample{Inputs: features, Masks: masks, Acceptable: acceptable}, outcomes})
		}
	}
	return rows, nil
}

func writeExclusive(path string, raw []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(raw)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func replay(ctx context.Context, r row, mask uint16) (outcome, string, string, error) {
	doc := r.Record.Document
	p, err := doc.Prepare()
	if err != nil {
		return outcome{}, "", "", err
	}
	choices := make(map[string]string, len(doc.Plan.Decisions))
	for i, choice := range doc.Plan.Decisions {
		choices[choice.ID] = choice.Options[mask>>i&1].Label
	}
	program, err := p.Compile(choices)
	if err != nil {
		return outcome{}, "", "", err
	}
	conditions, err := p.CheckDeclaredConditions(ctx, choices)
	if err != nil {
		return outcome{}, "", "", err
	}
	o := outcome{Mask: mask, OutputTotal: len(doc.TestCases), ConditionTotal: len(conditions)}
	for _, c := range conditions {
		if c.Passed {
			o.ConditionPassed++
		}
	}
	for _, c := range doc.TestCases {
		v, err := program.Evaluate(c.Input)
		if err != nil {
			return outcome{}, "", "", err
		}
		if v.Int == c.Expected {
			o.OutputPassed++
		}
	}
	return o, program.GoooSource(), digest([]byte(program.GoSource())), nil
}

func run(dataset, out string) error {
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		return fmt.Errorf("fresh output directory required")
	}
	raw, err := os.ReadFile(dataset)
	if err != nil {
		return err
	}
	if len(raw) > 2<<20 {
		return fmt.Errorf("study dataset exceeds2MiB")
	}
	var records []sourceRecord
	if err := json.Unmarshal(raw, &records); err != nil {
		return err
	}
	if len(records) != 24 {
		return fmt.Errorf("protocol requires24 source programs")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	rows, err := prepare(ctx, records)
	if err != nil {
		return err
	}
	var training []conditiondecision.Sample
	for _, r := range rows {
		if r.Record.Split == "train" {
			training = append(training, r.Sample)
		}
	}
	if len(training) != 40 || len(rows) != 120 {
		return fmt.Errorf("protocol split/context counts differ")
	}
	trainingRaw, err := json.Marshal(training)
	if err != nil {
		return err
	}
	options := conditiondecision.FitOptions{Epochs: 400, LearningRate: 0.25, L2: 0.0001, Seed: 17}
	start := time.Now()
	model, history, err := conditiondecision.Fit(ctx, training, options)
	trainingNS := time.Since(start).Nanoseconds()
	if err != nil {
		return err
	}
	artifact, err := model.Marshal()
	if err != nil {
		return err
	}
	loaded, err := conditiondecision.Decode(artifact)
	if err != nil {
		return err
	}
	if loaded.Weights() != model.Weights() {
		return fmt.Errorf("exported weights differ")
	}
	results := make([]result, 0, len(rows))
	groups := map[string]group{}
	times := make([]int64, 0, len(rows))
	var workspace conditiondecision.Workspace
	for _, r := range rows {
		var prediction conditiondecision.Prediction
		start := time.Now()
		err := loaded.PredictInto(r.Sample.Inputs, r.Sample.Masks, &workspace, &prediction)
		elapsed := time.Since(start).Nanoseconds()
		if err != nil {
			return err
		}
		valid := r.Sample.Acceptable>>prediction.Selected&1 != 0
		mass := 0.0
		for i, p := range prediction.Probabilities[:prediction.Count] {
			if r.Sample.Acceptable>>i&1 != 0 {
				mass += float64(p)
			}
		}
		observed, gooo, goSHA, err := replay(ctx, r, prediction.Selected)
		if err != nil {
			return err
		}
		if observed != r.Outcomes[prediction.Selected] {
			return fmt.Errorf("selected program replay differs")
		}
		results = append(results, result{r.Record.ID, r.Record.Split, r.Context, r.Record.SHA, r.PlanSHA, prediction.Selected, r.Sample.Acceptable, valid, r.Sample.Acceptable&1 != 0, observed, mass, elapsed, gooo, goSHA})
		times = append(times, elapsed)
		kind := "observed"
		if r.Context == "initial" {
			kind = "initial"
		}
		key := r.Record.Split + "/" + kind
		g := groups[key]
		g.Rows++
		if valid {
			g.Valid++
		}
		if r.Sample.Acceptable&1 != 0 {
			g.BaselineValid++
		}
		g.OutputsPassed += observed.OutputPassed
		g.OutputsTotal += observed.OutputTotal
		g.ConditionsPassed += observed.ConditionPassed
		g.ConditionsTotal += observed.ConditionTotal
		groups[key] = g
	}
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	build, _ := debug.ReadBuildInfo()
	report := map[string]any{"schema": "gooo/condition-candidate-study/v1", "build": build, "dataset_sha256": digest(raw), "training_samples_sha256": digest(trainingRaw), "model_sha256": digest(artifact), "model_artifact_bytes": len(artifact), "parameter_count": conditiondecision.ParameterCount, "resident_weight_bytes": conditiondecision.ParameterCount * 4, "training_rows": len(training), "options": options, "training_ns": trainingNS, "first_loss": history[0].Loss, "last_pre_update_loss": history[len(history)-1].Loss, "model_predictions": len(results), "native_executions": 0, "selected_candidate_replays": len(results), "prediction_p50_ns": times[len(times)/2], "prediction_p95_ns": times[(len(times)*95-1)/100], "groups": groups, "results": results, "scope": "Finite source output and condition cases; initial and observed contexts are correlated. New wording and local-variable structures are separate small splits; probabilities are uncalibrated. FP32 only. No compiler search integration or general language accuracy claim."}
	reportRaw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	historyRaw, err := json.Marshal(history)
	if err != nil {
		return err
	}
	if err := os.Mkdir(out, 0700); err != nil {
		return err
	}
	for name, data := range map[string][]byte{"model.json": artifact, "report.json": reportRaw, "history.json": historyRaw} {
		if err := writeExclusive(filepath.Join(out, name), data); err != nil {
			return err
		}
	}
	summary, _ := json.MarshalIndent(groups, "", "  ")
	fmt.Println(string(summary))
	return nil
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: condition-fit dataset.json fresh-output-directory")
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
