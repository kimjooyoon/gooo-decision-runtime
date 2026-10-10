// Read-only recount. No model predictions, compiler execution or training.
package main

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

func hash(raw []byte) string { s := sha256.Sum256(raw); return hex.EncodeToString(s[:]) }

type counts struct{ Programs, Complete, Attempts, Calls, FeedbackCalls int }

func audit(root string) error {
	f, err := os.Open(filepath.Join(root, "report.json.gz"))
	if err != nil {
		return err
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer z.Close()
	raw, err := io.ReadAll(io.LimitReader(z, 2<<20))
	if err != nil {
		return err
	}
	var report struct {
		Schema, DatasetSHA, ModelFileSHA, SourceRevision string
		SourceModified                                   bool
		NativeExecutions, TrainingCalls                  int
		Results                                          []struct {
			ID, Mode, SourceSHA, PlanSHA, SelectedGooo, SelectedGoSHA, Error string
			Search                                                           pathplan.SearchResult
			Progress                                                         []pathplan.ConditionProgress
			Feedback                                                         []pathplan.ConditionRanking
		}
	}
	if err = json.Unmarshal(raw, &report); err != nil {
		return err
	}
	if report.Schema != "gooo/condition-search-study/v1" || report.SourceRevision != "670f90582ec756ef8601d827fa188a474297b52b" || report.SourceModified || report.TrainingCalls != 0 || report.NativeExecutions != 0 || len(report.Results) != 72 {
		return errors.New("producer or experiment identity differs")
	}
	if report.DatasetSHA != "0b3bddcaf1ebd82f5e1153634f44e2f841e6f3944d5432a4c50744a4eb17123b" || report.ModelFileSHA != "a16696ed44c668f38cc2e7ee1dc6ff3f4716649d57e59df7c9490d1c670edc48" {
		return errors.New("frozen input differs")
	}
	totals := map[string]counts{}
	seen := map[string]bool{}
	for _, row := range report.Results {
		key := row.ID + "/" + row.Mode
		if seen[key] {
			return errors.New("duplicate row")
		}
		seen[key] = true
		if row.Error != "" || row.Search.Status != "TRAINING_COMPLETE" || row.Search.SelectedTrainingPassed != row.Search.TrainingTotal || len(row.Search.Attempts) > 4 || len(row.Progress) == 0 || row.SelectedGooo == "" || len(row.SelectedGoSHA) != 64 {
			return errors.New("incomplete search")
		}
		for _, c := range row.Search.Selection.Conditions {
			if !c.Passed {
				return errors.New("selected body failed condition")
			}
		}
		masks := map[uint16]bool{}
		for _, attempt := range row.Search.Attempts {
			if masks[attempt.Mask] {
				return errors.New("repeated committed mask")
			}
			masks[attempt.Mask] = true
		}
		calls := row.Progress[0].Ranking.Calls
		feedbackCalls := 0
		for _, r := range row.Feedback {
			calls += r.Calls
			feedbackCalls += r.Calls
		}
		if calls != row.Search.Selection.ModelCalls {
			return errors.New("call totals differ")
		}
		var previous string
		for i, p := range row.Progress {
			sha := p.SHA
			p.SHA = ""
			encoded, _ := json.Marshal(p)
			if hash(encoded) != sha {
				return errors.New("combined progress hash differs")
			}
			if p.Search.Sequence != i+1 || p.Search.PreviousSHA != previous {
				return errors.New("progress chain differs")
			}
			previous = p.Search.SHA
			p.Search.SHA = ""
			encoded, _ = json.Marshal(p.Search)
			if hash(encoded) != previous {
				return errors.New("search digest differs")
			}
			r := p.Ranking
			rankSHA := r.SHA
			r.SHA = ""
			encoded, _ = json.Marshal(r)
			if hash(encoded) != rankSHA {
				return errors.New("ranking digest differs")
			}
			if r.PlanSHA != row.PlanSHA || p.Search.Selection.PlanSHA256 != row.PlanSHA {
				return errors.New("source plan binding differs")
			}
		}
		c := totals[row.Mode]
		c.Programs++
		c.Complete++
		c.Attempts += len(masks)
		c.Calls += calls
		c.FeedbackCalls += feedbackCalls
		totals[row.Mode] = c
	}
	want := map[string]counts{"deterministic": {24, 24, 60, 0, 0}, "initial_model": {24, 24, 35, 24, 0}, "condition_feedback": {24, 24, 28, 28, 4}}
	for mode, count := range want {
		if totals[mode] != count {
			return fmt.Errorf("mode total differs: %s", mode)
		}
	}
	if len(totals) != 3 {
		return errors.New("unexpected mode")
	}
	result := struct {
		Status                                                       string
		Rows, NewPredictions, NewProgramExecutions, NewTrainingCalls int
		Totals                                                       map[string]counts
	}{"PASS", 72, 0, 0, 0, totals}
	encoded, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(encoded))
	return nil
}
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: audit STUDY_DIRECTORY")
		os.Exit(2)
	}
	if err := audit(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
