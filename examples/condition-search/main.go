// condition-search runs a fresh paired search study with frozen public weights.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
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
type result struct {
	ID, Split, Mode, SourceSHA, PlanSHA string
	Search                              pathplan.SearchResult
	Progress                            []pathplan.ConditionProgress
	Feedback                            []pathplan.ConditionRanking
	SelectedGooo, SelectedGoSHA         string
	SearchNS                            int64
	Error                               string `json:",omitempty"`
}
type totals struct {
	Programs, Complete, Attempts, ConditionRejected, ModelCalls, FeedbackCalls, Reused int
	PredictionMedianNS, PredictionP95NS                                                int64
}
type report struct {
	Schema                                                     string
	DatasetSHA, ModelFileSHA, ModelFingerprint, SourceRevision string
	SourceModified                                             bool
	NativeExecutions, TrainingCalls                            int
	Scope                                                      string
	Totals                                                     map[string]totals
	Results                                                    []result
}

func digest(raw []byte) string { s := sha256.Sum256(raw); return hex.EncodeToString(s[:]) }
func save(path string, raw []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(raw)
	return errors.Join(err, f.Close())
}

func run(dataset, weights, out string) error {
	// Establish a single-use destination before any model or program execution.
	if err := os.Mkdir(out, 0700); err != nil {
		return err
	}
	if err := save(filepath.Join(out, "started.txt"), []byte("Frozen model search experiment started. Preserve partial outcomes; use a new destination for a new experiment.\n")); err != nil {
		return err
	}
	raw, err := os.ReadFile(dataset)
	if err != nil {
		return err
	}
	if len(raw) > 2<<20 {
		return errors.New("dataset bound exceeded")
	}
	modelRaw, err := os.ReadFile(weights)
	if err != nil {
		return err
	}
	m, err := conditiondecision.Decode(modelRaw)
	if err != nil {
		return err
	}
	var records []sourceRecord
	if err := json.Unmarshal(raw, &records); err != nil {
		return err
	}
	if len(records) != 24 {
		return errors.New("fixed paired study expects 24 original Gooo contracts")
	}
	r := report{Schema: "gooo/condition-search-study/v1", DatasetSHA: digest(raw), ModelFileSHA: digest(modelRaw), ModelFingerprint: m.Fingerprint(), Totals: map[string]totals{}, Scope: "Same 24 absolute-value contracts consumed in the first study. Three searches per source, up to four candidates each, source output and condition cases used during search. No retraining, independent accuracy, host CPU delta or native execution claim."}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				r.SourceRevision = s.Value
			}
			if s.Key == "vcs.modified" {
				r.SourceModified = s.Value == "true"
			}
		}
	}
	if r.SourceRevision == "" || r.SourceModified {
		return errors.New("clean committed producer required")
	}
	seen := map[string]bool{}
	for _, source := range records {
		if seen[source.ID] || source.SHA != digest([]byte(source.Source)) || len(source.Document.Plan.Decisions) != 2 {
			return errors.New("duplicate source, source hash or two-choice protocol mismatch")
		}
		seen[source.ID] = true
		p, err := source.Document.Prepare()
		if err != nil {
			return err
		}
		for _, mode := range []string{"deterministic", "initial_model", "condition_feedback"} {
			model, rounds := m, 0
			if mode == "deterministic" {
				model = nil
			}
			if mode == "condition_feedback" {
				rounds = 3
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			start := time.Now()
			search, body, progress, feedback, failure := p.SearchConditionBatches(ctx, model, source.Document.TestCases, 4, 1, "", rounds)
			elapsed := time.Since(start).Nanoseconds()
			cancel()
			row := result{ID: source.ID, Split: source.Split, Mode: mode, SourceSHA: source.SHA, PlanSHA: p.PlanSHA256(), Search: search, Progress: progress, Feedback: feedback, SearchNS: elapsed}
			if failure != nil {
				row.Error = failure.Error()
			}
			if body != nil {
				row.SelectedGooo = body.GoooSource()
				row.SelectedGoSHA = digest([]byte(body.GoSource()))
			}
			r.Results = append(r.Results, row)
			// Append the exact record immediately, including failures.
			encoded, err := json.Marshal(row)
			if err != nil {
				return err
			}
			if err := save(filepath.Join(out, fmt.Sprintf("row-%03d.json", len(r.Results))), encoded); err != nil {
				return err
			}
		}
	}
	latencies := map[string][]int64{}
	for _, row := range r.Results {
		key := row.Mode
		count := r.Totals[key]
		count.Programs++
		count.Attempts += len(row.Search.Attempts)
		count.ConditionRejected += row.Search.ConditionRejected
		count.ModelCalls += row.Search.Selection.ModelCalls
		if row.Search.Status == "TRAINING_COMPLETE" {
			count.Complete++
		}
		if len(row.Progress) > 0 && row.Progress[0].Ranking.Calls > 0 {
			latencies[key] = append(latencies[key], row.Progress[0].Ranking.PredictNS)
		}
		for _, f := range row.Feedback {
			count.FeedbackCalls += f.Calls
			if f.Reused {
				count.Reused++
			}
			if f.Calls > 0 {
				latencies[key] = append(latencies[key], f.PredictNS)
			}
		}
		r.Totals[key] = count
	}
	for mode, samples := range latencies {
		slices.Sort(samples)
		count := r.Totals[mode]
		count.PredictionMedianNS = samples[len(samples)/2]
		count.PredictionP95NS = samples[(len(samples)*95+99)/100-1]
		r.Totals[mode] = count
	}
	encoded, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if err := save(filepath.Join(out, "report.json"), encoded); err != nil {
		return err
	}
	if err := save(filepath.Join(out, "completed.txt"), []byte("72 searches completed; 0 training calls; 0 native executions. Do not repeat this run.\n")); err != nil {
		return err
	}
	summary, _ := json.MarshalIndent(r.Totals, "", "  ")
	fmt.Println(string(summary))
	return nil
}
func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: condition-search DATASET MODEL NEW_OUTPUT_DIRECTORY")
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2], os.Args[3]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
