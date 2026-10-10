package main

import (
	"context"
	"os"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/flowdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

type candidate struct {
	Mask            uint16                     `json:"mask"`
	Choices         map[string]string          `json:"choices"`
	TypeError       string                     `json:"type_error,omitempty"`
	GoooBody        string                     `json:"gooo_body"`
	GoSHA           string                     `json:"go_sha256"`
	Cases           []pathplan.TestResult      `json:"cases"`
	Conditions      []pathplan.ConditionResult `json:"conditions"`
	OutputPassed    int                        `json:"output_passed"`
	ConditionPassed int                        `json:"condition_passed"`
	Accepted        bool                       `json:"accepted"`
}
type judgment struct {
	ID         string                  `json:"id"`
	Split      string                  `json:"split"`
	Context    string                  `json:"context"`
	Model      string                  `json:"model"`
	SourceSHA  string                  `json:"source_sha256"`
	FeatureSHA string                  `json:"features_sha256"`
	Acceptable uint64                  `json:"acceptable_candidate_bits"`
	Prediction flowdecision.Prediction `json:"prediction"`
	PredictNS  int64                   `json:"predict_ns"`
	Selected   candidate               `json:"selected"`
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
type group struct {
	Rows            int `json:"rows"`
	Valid           int `json:"valid"`
	OutputPassed    int `json:"output_passed"`
	OutputTotal     int `json:"output_total"`
	ConditionPassed int `json:"condition_passed"`
	ConditionTotal  int `json:"condition_total"`
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

func evaluate(ctx context.Context, s preparedSource, mask uint16) candidate {
	c := candidate{Mask: mask, Choices: map[string]string{}}
	for i, choice := range s.source.Document.Plan.Decisions {
		c.Choices[choice.ID] = choice.Options[mask>>i&1].Label
	}
	body, err := s.plan.Compile(c.Choices)
	if err != nil {
		c.TypeError = err.Error()
		return c
	}
	c.GoooBody, c.GoSHA = body.GoooBody(), hash([]byte(body.GoSource()))
	c.Conditions, err = s.plan.CheckDeclaredConditions(ctx, c.Choices)
	must(err)
	for _, condition := range c.Conditions {
		if condition.Passed {
			c.ConditionPassed++
		}
	}
	for _, test := range s.source.Document.TestCases {
		value, err := body.Evaluate(test.Input)
		must(err)
		passed := value.Int == test.Expected
		c.Cases = append(c.Cases, pathplan.TestResult{Input: test.Input, Expected: test.Expected, Actual: value.Int, Passed: passed})
		if passed {
			c.OutputPassed++
		}
	}
	c.Accepted = c.OutputPassed == len(s.source.Document.TestCases) && c.ConditionPassed == len(c.Conditions)
	return c
}

func judge(ctx context.Context, name string, model *flowdecision.Model, sources []preparedSource, rows []inputRecord, f *os.File, r *report) []int64 {
	byID := map[string]preparedSource{}
	for _, s := range sources {
		byID[s.source.ID] = s
	}
	times := make([]int64, 0, len(rows))
	for _, input := range rows {
		s := byID[input.ID]
		var workspace flowdecision.Workspace
		var prediction flowdecision.Prediction
		start := time.Now()
		must(model.PredictInto(input.Features, input.Masks, &workspace, &prediction))
		ns := time.Since(start).Nanoseconds()
		// This body's construction/evaluation happens immediately after this
		// prediction, before any other model prediction is made.
		selected := evaluate(ctx, s, prediction.Selected)
		require(selected.Accepted == (input.Acceptable>>prediction.Selected&1 != 0), "new selected body agrees with frozen acceptance label")
		appendRow(f, judgment{input.ID, input.Split, input.Context, name, s.source.SHA, input.SHA, input.Acceptable, prediction, ns, selected})
		kind := "observed"
		if input.Context == "initial" {
			kind = "initial"
		}
		key := name + "/" + input.Split + "/" + kind
		g := r.Groups[key]
		g.Rows++
		if selected.Accepted {
			g.Valid++
		}
		g.OutputPassed += selected.OutputPassed
		g.OutputTotal += len(s.source.Document.TestCases)
		g.ConditionPassed += selected.ConditionPassed
		g.ConditionTotal += len(s.source.Document.Plan.ConditionCases)
		r.Groups[key] = g
		r.Judgments++
		times = append(times, ns)
	}
	return times
}

func search(ctx context.Context, models [2]*flowdecision.Model, sources []preparedSource, f *os.File, r *report) {
	for _, s := range sources {
		for version, name := range variants {
			for _, feedback := range []bool{false, true} {
				mode, rounds := name+"_initial", 0
				if feedback {
					mode, rounds = name+"_feedback", 16
				}
				start := time.Now()
				result, body, progress, rankings, err := s.plan.SearchFlowBatches(ctx, models[version], s.source.Document.TestCases, 1<<len(s.projection.Features), 1, "", rounds)
				row := searchRow{ID: s.source.ID, Split: s.source.Split, Mode: mode, SearchNS: time.Since(start).Nanoseconds(), Result: result, Progress: progress, Feedback: rankings}
				if err != nil {
					row.Error = err.Error()
				}
				if body != nil {
					row.SelectedBody = body.GoooBody()
				}
				require(len(progress) > 0, "initial runtime observation")
				for i, input := range s.projection.Features {
					require(progress[0].Ranking.FeatureSHA[i] == choiceSHA(input), "actual first runtime input matches frozen projection")
				}
				seen := map[uint16]bool{}
				for _, attempt := range result.Attempts {
					require(!seen[attempt.Mask], "no candidate repetition")
					seen[attempt.Mask] = true
				}
				appendRow(f, row)
				key := mode + "/" + s.source.Split
				g := r.SearchGroups[key]
				g.Programs++
				if result.Status == "TRAINING_COMPLETE" {
					g.Complete++
				}
				g.Attempts += len(result.Attempts)
				g.ModelCalls += result.Selection.ModelCalls
				g.SearchNS += row.SearchNS
				for _, ranking := range rankings {
					g.FeedbackCalls += ranking.Calls
					if ranking.Reused {
						g.Reused++
					}
				}
				r.SearchGroups[key] = g
				r.Searches++
				r.SearchModelCalls += result.Selection.ModelCalls
			}
		}
	}
}
