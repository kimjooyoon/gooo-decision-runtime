package pathplan

import (
	"container/heap"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/bits"
	"time"
	"unicode/utf8"

	"github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
)

type TestCase struct {
	Input    int64 `json:"input"`
	Expected int64 `json:"expected"`
}
type TestResult struct {
	Input    int64 `json:"input"`
	Expected int64 `json:"expected"`
	Actual   int64 `json:"actual"`
	Passed   bool  `json:"passed"`
}
type SearchAttempt struct {
	Mask    uint16            `json:"choice_mask"`
	Choices map[string]string `json:"choices"`
	Status  string            `json:"status"`
	Passed  int               `json:"training_cases_passed"`
	Total   int               `json:"training_cases_total"`
	Results []TestResult      `json:"case_results,omitempty"`
	GoooSHA string            `json:"gooo_source_sha256,omitempty"`
}
type SearchResult struct {
	Schema                   string            `json:"schema"`
	Status                   string            `json:"status"`
	Selection                Selection         `json:"selection"`
	DeclaredCombinations     int               `json:"declared_combinations"`
	Unattempted              int               `json:"unattempted_combinations"`
	Evaluated                int               `json:"evaluated_candidates"`
	TypeRejected             int               `json:"type_rejected_candidates"`
	SelectedTrainingPassed   int               `json:"selected_training_passed"`
	TrainingTotal            int               `json:"training_cases"`
	Attempts                 []SearchAttempt   `json:"attempts"`
	ModelAbstentionsObserved int               `json:"model_abstentions_observed"`
	EligibleProbabilities    [][2]float64      `json:"eligible_probabilities,omitempty"`
	InitialProposals         map[string]string `json:"initial_proposals"`
}
type searchNode struct {
	mask  uint16
	score float64
}
type searchHeap []searchNode

func (h searchHeap) Len() int { return len(h) }
func (h searchHeap) Less(i, j int) bool {
	if h[i].score == h[j].score {
		return h[i].mask < h[j].mask
	}
	return h[i].score > h[j].score
}
func (h searchHeap) Swap(i, j int)   { h[i], h[j] = h[j], h[i] }
func (h *searchHeap) Push(value any) { *h = append(*h, value.(searchNode)) }
func (h *searchHeap) Pop() any {
	old := *h
	last := old[len(old)-1]
	*h = old[:len(old)-1]
	return last
}

// Search ranks declared typed paths before evaluating any finite cases. Model
// confidence/abstention is an observation, not a veto on a compiler-owned TDD
// experiment. Every offered/combined path remains subject to type checking.
// Holdout cases are deliberately absent from this API.
func Search(ctx context.Context, plan Plan, model *decision.Model, cases []TestCase, maxAttempts int, seed string) (SearchResult, *bodyplan.Program, error) {
	if ctx == nil {
		return SearchResult{}, nil, errors.New("search context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return SearchResult{}, nil, errors.New("search context must have a deadline")
	}
	if len(cases) == 0 || len(cases) > 128 || maxAttempts < 1 || maxAttempts > 64 {
		return SearchResult{}, nil, errors.New("finite search budget is invalid")
	}
	defaults, err := Validate(plan)
	if err != nil {
		return SearchResult{}, nil, err
	}
	if plan.Base.ResultType != decision.TypeInt {
		return SearchResult{}, nil, errors.New("integer test cases require an integer result")
	}
	if model != nil && model.Schema() != decision.PathMetadataSchema {
		return SearchResult{}, nil, errors.New("search requires the structural model ABI")
	}
	if seed != "" && (model == nil || len(seed) > 512 || !utf8.ValidString(seed)) {
		return SearchResult{}, nil, errors.New("seeded search requires a model and a bounded seed")
	}
	raw, err := json.Marshal(plan)
	if err != nil || len(raw) > 128<<10 {
		return SearchResult{}, nil, errors.New("serialized path budget exceeded")
	}
	result := SearchResult{Schema: "gooo/typed-path-tdd-search/v1", Status: "PARTIAL", DeclaredCombinations: 1 << len(plan.Decisions), TrainingTotal: len(cases), Selection: Selection{Schema: "gooo/typed-body-path-selection/v1", PlanSHA256: hash(raw), Choices: defaults, ExternalCallsKnown: true}}
	result.Unattempted = result.DeclaredCombinations
	result.InitialProposals = cloneChoices(defaults)
	if model != nil {
		result.Selection.ModelVariant, result.Selection.MetadataSHA256, result.Selection.WeightsSHA256 = model.Variant(), model.MetadataSHA256(), model.WeightsSHA256()
	}
	if seed != "" {
		result.Selection.SeedSHA256 = hash([]byte(seed))
	}
	var logWeights [16][2]float64
	var initialMask, fallbackMask uint16
	var workspace decision.Workspace
	for i, choice := range plan.Decisions {
		if err := ctx.Err(); err != nil {
			return result, nil, err
		}
		if choice.Fallback == choice.Options[1].Label {
			fallbackMask |= 1 << i
		}
		receipt := Receipt{ID: choice.ID, Kind: choice.Kind, IntentSHA256: hash([]byte(choice.Intent)), Selected: choice.Fallback, Mode: "declared_fallback"}
		if model != nil {
			var prediction decision.Prediction
			started := time.Now()
			err := model.PredictInto(choice.Intent, &workspace, &prediction)
			receipt.PredictNS = time.Since(started).Nanoseconds()
			result.Selection.ModelCalls++
			if err != nil {
				return result, nil, errors.New("structural ranking prediction failed")
			}
			receipt.Proposed, receipt.Confidence, receipt.Probabilities = model.PredictLabel(&prediction), prediction.Confidence, prediction.Probabilities
			if prediction.Abstained {
				result.ModelAbstentionsObserved++
			}
			weights := eligibleProbabilities(choice, prediction, model.Temperature())
			result.EligibleProbabilities = append(result.EligibleProbabilities, weights)
			for j, weight := range weights {
				logWeights[i][j] = math.Log(math.Max(weight, 1e-12))
			}
			selected := 0
			if weights[1] > weights[0] {
				selected = 1
			}
			if seed != "" {
				label, err := sample(result.Selection.PlanSHA256, choice, result.Selection.MetadataSHA256, result.Selection.WeightsSHA256, seed, weights)
				if err != nil {
					return result, nil, err
				}
				if label == choice.Options[1].Label {
					selected = 1
				} else {
					selected = 0
				}
			}
			if selected == 1 {
				initialMask |= 1 << i
			}
			receipt.Selected, receipt.Mode = choice.Options[selected].Label, "typed_probability_rank_before_finite_validation"
			result.InitialProposals[choice.ID] = receipt.Selected
		}
		result.Selection.Receipts = append(result.Selection.Receipts, receipt)
	}
	if model == nil {
		initialMask = fallbackMask
	}
	score := func(mask uint16) float64 {
		if model == nil {
			return -float64(bits.OnesCount16(mask ^ fallbackMask))
		}
		sum := 0.0
		for i := range plan.Decisions {
			sum += logWeights[i][int(mask>>i&1)]
		}
		return sum
	}
	queue := searchHeap{{initialMask, score(initialMask)}}
	heap.Init(&queue)
	seen := map[uint16]bool{initialMask: true}
	bestPassed := -1
	var best *bodyplan.Program
	var bestChoices map[string]string
	for queue.Len() > 0 && len(result.Attempts) < maxAttempts {
		if err := ctx.Err(); err != nil {
			return result, best, err
		}
		node := heap.Pop(&queue).(searchNode)
		choices := make(map[string]string, len(plan.Decisions))
		for i, choice := range plan.Decisions {
			choices[choice.ID] = choice.Options[int(node.mask>>i&1)].Label
		}
		attempt := SearchAttempt{Mask: node.mask, Choices: choices, Total: len(cases), Status: "TYPE_REJECTED"}
		program, err := assemble(plan, choices)
		if err != nil {
			result.TypeRejected++
		} else {
			attempt.Status = "EVALUATED"
			attempt.GoooSHA = hash([]byte(program.GoooSource()))
			result.Evaluated++
			for _, test := range cases {
				if err := ctx.Err(); err != nil {
					return result, best, err
				}
				value, err := program.Evaluate(test.Input)
				if err != nil {
					return result, best, errors.New("finite path interpreter failed")
				}
				passed := value.Int == test.Expected
				if passed {
					attempt.Passed++
				}
				attempt.Results = append(attempt.Results, TestResult{test.Input, test.Expected, value.Int, passed})
			}
			if attempt.Passed > bestPassed {
				bestPassed, best, bestChoices = attempt.Passed, program, cloneChoices(choices)
			}
		}
		result.Attempts = append(result.Attempts, attempt)
		result.Unattempted = result.DeclaredCombinations - len(result.Attempts)
		if attempt.Status == "EVALUATED" && attempt.Passed == len(cases) {
			result.Status = "TRAINING_COMPLETE"
			break
		}
		for i := range plan.Decisions {
			neighbor := node.mask ^ (1 << i)
			if !seen[neighbor] {
				seen[neighbor] = true
				heap.Push(&queue, searchNode{neighbor, score(neighbor)})
			}
		}
	}
	result.Unattempted = result.DeclaredCombinations - len(result.Attempts)
	if best == nil {
		return result, nil, errors.New("no typed candidate was evaluated within the search budget")
	}
	result.SelectedTrainingPassed = bestPassed
	result.Selection.Choices = bestChoices
	for i := range result.Selection.Receipts {
		result.Selection.Receipts[i].Selected = bestChoices[result.Selection.Receipts[i].ID]
		result.Selection.Receipts[i].Mode = "finite_tdd_selection"
	}
	return result, best, nil
}

// Normalize the two eligible raw logits, independent of unrelated output heads.
// Global softmax can underflow both eligible probabilities to zero. These
// conditional probabilities rank paths; they are not calibrated acceptance odds.
func eligibleProbabilities(choice Choice, prediction decision.Prediction, temperature float32) [2]float64 {
	labels := decision.PathLabels()
	var logits [2]float64
	for i, option := range choice.Options {
		for j, label := range labels {
			if option.Label == label {
				logits[i] = float64(prediction.Logits[j]) / float64(temperature)
			}
		}
	}
	maximum := math.Max(logits[0], logits[1])
	first, second := math.Exp(logits[0]-maximum), math.Exp(logits[1]-maximum)
	return [2]float64{first / (first + second), second / (first + second)}
}
