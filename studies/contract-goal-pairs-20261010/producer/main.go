package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"slices"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/contract-goals-20261010/record"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func require(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
func write(root, name string, data []byte) {
	f, err := os.OpenFile(filepath.Join(root, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	must(err)
	_, err = f.Write(data)
	must(err)
	must(f.Close())
}
func save(root, name string, value any) { write(root, name, r.Encode(value)) }
func identity() string {
	b, ok := debug.ReadBuildInfo()
	require(ok && b.GoVersion == "go1.27.2", "Go1.27.2 required")
	rev, clean := "", false
	for _, setting := range b.Settings {
		if setting.Key == "vcs.revision" {
			rev = setting.Value
		}
		if setting.Key == "vcs.modified" {
			clean = setting.Value == "false"
		}
	}
	require(clean && len(rev) == 40, "clean committed producer required")
	return rev
}
func readGzip(path, hash string) []byte {
	raw, err := os.ReadFile(path)
	must(err)
	require(r.Hash(raw) == hash, "frozen compressed input changed")
	f, err := os.Open(path)
	must(err)
	defer f.Close()
	z, err := gzip.NewReader(f)
	must(err)
	defer z.Close()
	data, err := io.ReadAll(z)
	must(err)
	return data
}

type cases [][32]float32

func (c cases) CaseCount() int                          { return len(c) }
func (c cases) CaseFeatures(i int) ([32]float32, error) { return c[i], nil }

func prepare(source r.Source) *pathplan.PreparedPlan {
	p, err := source.Document.Prepare()
	must(err)
	input, err := p.InitialContractInput(source.Document.TestCases)
	must(err)
	require(input.CaseSHA256() == source.CaseSHA && p.PlanSHA256() == source.PlanSHA, "source/case identity")
	for i, choice := range source.Document.Plan.Decisions {
		var row [384]float32
		must(input.RelationalSourceFeaturesInto(choice.ID, &row))
		require(row == source.Inputs[i], "initial source arrays changed")
	}
	for i := range input.CaseCount() {
		row, err := input.CaseFeatures(i)
		must(err)
		require(row == source.CaseRows[i], "initial case arrays changed")
	}
	return p
}

func search(ctx context.Context, source r.Source, p *pathplan.PreparedPlan, model *contractdecision.Model) r.Search {
	start := time.Now()
	s, err := p.NewContractSession(ctx, model, source.Document.TestCases)
	must(err)
	row := r.Search{ID: source.Spec.ID, Split: source.Spec.Split, Family: source.Spec.Family, Mode: "goal_pairs", Ranking: s.Ranking()}
	initial, err := s.Observe()
	must(err)
	row.Progress = append(row.Progress, initial)
	for range 4 {
		progress, body, err := s.Advance(ctx, 1)
		if err != nil && !errors.Is(err, pathplan.ErrNoConditionCandidate) && !errors.Is(err, pathplan.ErrNoTypedCandidate) {
			panic(err)
		}
		require(progress.Schema != "" && len(progress.NewAttempts) == 1, "one immediate candidate")
		row.Progress = append(row.Progress, progress)
		if body != nil {
			row.GoooBody, row.GoSource = body.GoooBody(), body.GoSource()
		}
		if progress.Status == "TRAINING_COMPLETE" || progress.Exhausted {
			break
		}
	}
	row.NS = time.Since(start).Nanoseconds()
	return row
}

func main() {
	require(len(os.Args) == 3, "usage: producer FROZEN_RESULT FRESH_OUTPUT")
	producer := identity()
	root, out := os.Args[1], os.Args[2]
	must(os.Mkdir(out, 0700))
	save(out, "started.json", map[string]string{"producer": producer, "time": time.Now().UTC().Format(time.RFC3339Nano)})
	sourceRaw := readGzip(filepath.Join(root, "sources.jsonl.gz"), "773db4e279b497465a92ad2d7df683f0a2898d72365f6f48ac351e5f7036b413")
	trainingRaw := readGzip(filepath.Join(root, "training.json.gz"), "e8e07ad2cbc8ff5adac35b81999000717baef4e0143c93c204dc483cbdcf7d1d")
	rows, plans := sources(sourceRaw)
	var training []r.Training
	must(json.Unmarshal(trainingRaw, &training))
	require(len(training) == 24 && len(rows) == 196, "fixed collection size")
	samples := trainingSamples(rows, training)
	var pairs []contractdecision.GoalPair
	for i := 0; i < len(training); i += 2 {
		require(training[i].ID[:len(training[i].ID)-1] == training[i+1].ID[:len(training[i+1].ID)-1], "consecutive original goal pair")
		require(training[i].ID[len(training[i].ID)-1] == '0' && training[i+1].ID[len(training[i+1].ID)-1] == '1', "original goal order")
		pairs = append(pairs, contractdecision.GoalPair{First: i, Second: i + 1})
	}
	require(len(pairs) == 12, "twelve original training goal pairs")
	pairOptions := contractdecision.GoalPairOptions{Weight: .5, Margin: 2}
	save(out, "pairs.json", pairs)
	write(out, "training.json", trainingRaw)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	options := contractdecision.FitOptions{Epochs: 600, LearningRate: .3, L2: .0001, Seed: 17}
	start := time.Now()
	model, history, err := contractdecision.FitWithGoalPairs(ctx, samples, options, contractdecision.ExtremePooling, pairs, pairOptions)
	must(err)
	trainingNS := time.Since(start).Nanoseconds()
	raw, err := model.Marshal()
	must(err)
	write(out, "model.json", raw)
	save(out, "history.json", history)
	f, err := os.OpenFile(filepath.Join(out, "searches.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	must(err)
	var times []int64
	for i, source := range rows {
		row := search(ctx, source, plans[i], model)
		require(row.Ranking.Calls == 1, "one forward per source")
		must(json.NewEncoder(f).Encode(row))
		times = append(times, row.Ranking.PredictNS)
	}
	must(f.Close())
	slices.Sort(times)
	save(out, "report.json", map[string]any{"schema": "gooo/contract-goal-pairs-study/v1", "producer": producer,
		"goal_pairs": len(pairs), "pair_options": pairOptions, "fit_forward_passes": 600 * (24 + 2*12),
		"pooling": model.Pooling(), "model_schema": model.ArtifactSchema(), "model_sha256": r.Hash(raw), "fingerprint": model.Fingerprint(),
		"source_raw_sha256": r.Hash(sourceRaw), "training_sha256": r.Hash(trainingRaw), "fit_options": options,
		"fits": 1, "training_rows": 24, "parameters": contractdecision.ParameterCount, "weight_bytes": contractdecision.ParameterCount * 4,
		"artifact_bytes": len(raw), "training_ns": trainingNS, "first_loss": history[0].Loss, "last_loss": history[599].Loss,
		"sources": len(rows), "searches": len(rows), "model_calls": len(times), "post_failure_calls": 0, "native_processes": 0,
		"prediction_median_ns": times[len(times)/2], "prediction_p95_ns": times[(len(times)*95+99)/100-1]})
	save(out, "completed.json", map[string]any{"time": time.Now().UTC().Format(time.RFC3339Nano), "sources": len(rows), "model_calls": len(times)})
	fmt.Println("one fresh paired-goal fit, 196 new-model searches and calls; original unpaired observations retained")
}

func sources(raw []byte) ([]r.Source, []*pathplan.PreparedPlan) {
	d := json.NewDecoder(bytes.NewReader(raw))
	var rows []r.Source
	var plans []*pathplan.PreparedPlan
	for {
		var row r.Source
		err := d.Decode(&row)
		if err == io.EOF {
			break
		}
		must(err)
		rows = append(rows, row)
		plans = append(plans, prepare(row))
	}
	return rows, plans
}

func trainingSamples(rows []r.Source, training []r.Training) []contractdecision.Sample {
	byID := make(map[string]r.Source, len(rows))
	for _, row := range rows {
		byID[row.Spec.ID] = row
	}
	var samples []contractdecision.Sample
	for _, row := range training {
		source, ok := byID[row.ID]
		require(ok && source.Spec.Split == "train" && source.Acceptable == row.Acceptable && source.PlanSHA == row.PlanSHA && source.CaseSHA == row.CaseSHA, "training membership")
		require(reflect.DeepEqual(row.Inputs, source.Inputs) && reflect.DeepEqual(row.Cases, source.CaseRows), "frozen training arrays")
		samples = append(samples, contractdecision.Sample{Inputs: row.Inputs, Cases: cases(row.Cases), Masks: row.Masks, Acceptable: row.Acceptable})
	}
	return samples
}
