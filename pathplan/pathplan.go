// Package pathplan lets a closed model select structural edits to typed bodies.
// Choices refer to compiler-owned identifiers and arena entries, never source text.
package pathplan

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
)

const Schema = "gooo/typed-body-path-plan/v1"

const (
	LocalReference   = "local_reference"
	AssignmentTarget = "assignment_target"
	OperandOrder     = "operand_order"
	BranchLayout     = "branch_layout"
	RootOrder        = "root_order"
)

type Plan struct {
	Schema    string        `json:"schema"`
	Base      bodyplan.Plan `json:"base"`
	Decisions []Choice      `json:"decisions"`
}

type Choice struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"`
	Target   int      `json:"target"`
	Intent   string   `json:"intent"`
	Options  []Option `json:"options"`
	Fallback string   `json:"fallback"`
}

type Option struct {
	Label   string `json:"label"`
	Name    string `json:"name,omitempty"`
	Reverse bool   `json:"reverse,omitempty"`
	Order   []int  `json:"order,omitempty"`
}

type Receipt struct {
	ID            string                       `json:"id"`
	Kind          string                       `json:"kind"`
	IntentSHA256  string                       `json:"intent_sha256"`
	Mode          string                       `json:"mode"`
	Proposed      string                       `json:"proposed,omitempty"`
	Selected      string                       `json:"selected"`
	Confidence    float32                      `json:"global_confidence,omitempty"`
	Probabilities [decision.LabelCount]float32 `json:"raw_probabilities"`
	PredictNS     int64                        `json:"predict_ns"`
}

type Selection struct {
	Schema             string            `json:"schema"`
	PlanSHA256         string            `json:"plan_sha256"`
	Choices            map[string]string `json:"choices"`
	Receipts           []Receipt         `json:"receipts"`
	ModelVariant       string            `json:"model_variant,omitempty"`
	MetadataSHA256     string            `json:"model_metadata_sha256,omitempty"`
	WeightsSHA256      string            `json:"model_weights_sha256,omitempty"`
	SeedSHA256         string            `json:"seed_sha256,omitempty"`
	ModelCalls         int               `json:"local_model_predictions"`
	ExternalCalls      int               `json:"external_provider_calls"`
	ExternalCallsKnown bool              `json:"external_provider_calls_known"`
	Joint              *JointReceipt     `json:"joint_prediction,omitempty"`
	Three              *ThreeReceipt     `json:"three_choice_prediction,omitempty"`
}

func cloneBase(base bodyplan.Plan) bodyplan.Plan {
	base.Expressions = append([]bodyplan.Expr(nil), base.Expressions...)
	for i := range base.Expressions {
		base.Expressions[i].Allowed = append([]string(nil), base.Expressions[i].Allowed...)
	}
	base.Statements = append([]bodyplan.Stmt(nil), base.Statements...)
	for i := range base.Statements {
		base.Statements[i].Then = append([]int(nil), base.Statements[i].Then...)
		base.Statements[i].Else = append([]int(nil), base.Statements[i].Else...)
	}
	base.Root = append([]int(nil), base.Root...)
	return base
}

func operationFallbacks(base bodyplan.Plan) map[string]string {
	choices := make(map[string]string)
	for _, expression := range base.Expressions {
		if expression.Kind == bodyplan.ExprHole {
			choices[expression.HoleID] = expression.Fallback
		}
	}
	return choices
}

func labelsFor(kind string) ([2]string, bool) {
	switch kind {
	case LocalReference:
		return [2]string{"reference_first", "reference_second"}, true
	case AssignmentTarget:
		return [2]string{"assign_first", "assign_second"}, true
	case OperandOrder, BranchLayout:
		return [2]string{"layout_forward", "layout_reverse"}, true
	case RootOrder:
		return [2]string{"schedule_forward", "schedule_reverse"}, true
	default:
		return [2]string{}, false
	}
}

func validID(value string) bool {
	if len(value) == 0 || len(value) > 64 {
		return false
	}
	for _, b := range []byte(value) {
		if b != '_' && b != '-' && !(b >= 'a' && b <= 'z') && !(b >= '0' && b <= '9') {
			return false
		}
	}
	return true
}

func validateShape(plan Plan) (map[string]string, error) {
	if plan.Schema != Schema || len(plan.Decisions) == 0 || len(plan.Decisions) > 16 || len(plan.Base.Expressions) > 128 || len(plan.Base.Statements) > 128 {
		return nil, errors.New("path plan schema or arena bounds are invalid")
	}
	choices := make(map[string]string, len(plan.Decisions))
	addresses := make(map[string]bool)
	for _, choice := range plan.Decisions {
		labels, ok := labelsFor(choice.Kind)
		address := fmt.Sprintf("%s:%d", choice.Kind, choice.Target)
		if !ok || !validID(choice.ID) || choices[choice.ID] != "" || addresses[address] || len(choice.Options) != 2 ||
			strings.TrimSpace(choice.Intent) == "" || len(choice.Intent) > decision.InputMaxBytes || !utf8.ValidString(choice.Intent) || choice.Target < 0 {
			return nil, errors.New("path choice identity, text or option bounds are invalid")
		}
		addresses[address] = true
		seen := make(map[string]bool)
		for _, option := range choice.Options {
			if seen[option.Label] || option.Label != labels[0] && option.Label != labels[1] {
				return nil, errors.New("path option is outside its closed kind vocabulary")
			}
			seen[option.Label] = true
			switch choice.Kind {
			case LocalReference:
				if choice.Target >= len(plan.Base.Expressions) || plan.Base.Expressions[choice.Target].Kind != bodyplan.ExprLocal || option.Name == "" || len(option.Name) > 128 || option.Reverse || len(option.Order) != 0 {
					return nil, errors.New("local reference must name an existing typed local expression")
				}
			case AssignmentTarget:
				if choice.Target >= len(plan.Base.Statements) || plan.Base.Statements[choice.Target].Kind != bodyplan.StmtAssign || option.Name == "" || len(option.Name) > 128 || option.Reverse || len(option.Order) != 0 {
					return nil, errors.New("assignment choice must name an existing assignment")
				}
			case OperandOrder:
				if choice.Target >= len(plan.Base.Expressions) || plan.Base.Expressions[choice.Target].Kind != bodyplan.ExprBinary || option.Name != "" || len(option.Order) != 0 || option.Reverse != (option.Label == labels[1]) {
					return nil, errors.New("operand order only swaps a fixed binary expression")
				}
			case BranchLayout:
				if choice.Target >= len(plan.Base.Statements) || plan.Base.Statements[choice.Target].Kind != bodyplan.StmtIf || option.Name != "" || len(option.Order) != 0 || option.Reverse != (option.Label == labels[1]) {
					return nil, errors.New("branch layout only swaps a fixed if statement's branches")
				}
			case RootOrder:
				if choice.Target != 0 || option.Name != "" || option.Reverse || !permutation(plan.Base.Root, option.Order) {
					return nil, errors.New("root order must preserve every declared root statement")
				}
			}
		}
		if !seen[choice.Fallback] {
			return nil, errors.New("fallback is outside the declared path options")
		}
		choices[choice.ID] = choice.Fallback
	}
	return choices, nil
}

func permutation(original, selected []int) bool {
	if len(original) == 0 || len(original) > 128 || len(original) != len(selected) {
		return false
	}
	counts := make(map[int]int, len(original))
	for _, index := range original {
		counts[index]++
	}
	for _, index := range selected {
		counts[index]--
		if counts[index] < 0 {
			return false
		}
	}
	return true
}

func assemble(plan Plan, choices map[string]string) (*bodyplan.Program, error) {
	base := cloneBase(plan.Base)
	for _, choice := range plan.Decisions {
		selected := choices[choice.ID]
		for _, option := range choice.Options {
			if selected != option.Label {
				continue
			}
			switch choice.Kind {
			case LocalReference:
				base.Expressions[choice.Target].Name = option.Name
			case AssignmentTarget:
				base.Statements[choice.Target].Name = option.Name
			case OperandOrder:
				if option.Reverse {
					base.Expressions[choice.Target].Left, base.Expressions[choice.Target].Right = base.Expressions[choice.Target].Right, base.Expressions[choice.Target].Left
				}
			case BranchLayout:
				if option.Reverse {
					base.Statements[choice.Target].Then, base.Statements[choice.Target].Else = base.Statements[choice.Target].Else, base.Statements[choice.Target].Then
				}
			case RootOrder:
				base.Root = append([]int(nil), option.Order...)
			}
		}
	}
	return bodyplan.Compile(base, operationFallbacks(base))
}

// Validate rejects every individually ill-typed option before any model call.
// Interacting choices are checked again on the complete selected program.
func Validate(plan Plan) (map[string]string, error) {
	choices, _, err := validatePlan(plan)
	return choices, err
}

func validatePlan(plan Plan) (map[string]string, *bodyplan.Program, error) {
	choices, err := validateShape(plan)
	if err != nil {
		return nil, nil, err
	}
	fallback, err := assemble(plan, choices)
	if err != nil {
		return nil, nil, fmt.Errorf("fallback path: %w", err)
	}
	for _, choice := range plan.Decisions {
		for _, option := range choice.Options {
			trial := cloneChoices(choices)
			trial[choice.ID] = option.Label
			if _, err := assemble(plan, trial); err != nil {
				return nil, nil, fmt.Errorf("path %s option %s: %w", choice.ID, option.Label, err)
			}
		}
	}
	return choices, fallback, nil
}

func Compile(plan Plan, choices map[string]string) (*bodyplan.Program, error) {
	defaults, err := Validate(plan)
	if err != nil {
		return nil, err
	}
	if len(choices) != len(defaults) {
		return nil, errors.New("every structural choice must be selected exactly once")
	}
	for _, choice := range plan.Decisions {
		found := false
		for _, option := range choice.Options {
			found = found || option.Label == choices[choice.ID]
		}
		if !found {
			return nil, errors.New("missing or undeclared structural selection")
		}
	}
	return assemble(plan, choices)
}

func cloneChoices(choices map[string]string) map[string]string {
	clone := make(map[string]string, len(choices))
	for id, value := range choices {
		clone[id] = value
	}
	return clone
}

func hash(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }

// Choose reads only validated plans and intent. Gold choices and test cases are
// absent from its API. An omitted model selects deterministic declared fallbacks.
func Choose(plan Plan, model *decision.Model, seed string) (Selection, *bodyplan.Program, error) {
	choices, err := Validate(plan)
	if err != nil {
		return Selection{}, nil, err
	}
	if seed != "" && (model == nil || len(seed) > 512 || !utf8.ValidString(seed)) {
		return Selection{}, nil, errors.New("sampling needs a path model and a bounded UTF-8 seed")
	}
	if model != nil && model.Schema() != decision.PathMetadataSchema {
		return Selection{}, nil, errors.New("structural selection requires the path model ABI")
	}
	raw, err := json.Marshal(plan)
	if err != nil || len(raw) > 128<<10 {
		return Selection{}, nil, errors.New("path plan exceeds its serialized byte budget")
	}
	result := Selection{Schema: "gooo/typed-body-path-selection/v1", PlanSHA256: hash(raw), Choices: choices, ExternalCallsKnown: true}
	if model != nil {
		result.ModelVariant, result.MetadataSHA256, result.WeightsSHA256 = model.Variant(), model.MetadataSHA256(), model.WeightsSHA256()
	}
	if seed != "" {
		result.SeedSHA256 = hash([]byte(seed))
	}
	var workspace decision.Workspace
	labels := decision.PathLabels()
	for _, choice := range plan.Decisions {
		receipt := Receipt{ID: choice.ID, Kind: choice.Kind, IntentSHA256: hash([]byte(choice.Intent)), Mode: "declared_fallback", Selected: choice.Fallback}
		if model != nil {
			var prediction decision.Prediction
			started := time.Now()
			err := model.PredictInto(choice.Intent, &workspace, &prediction)
			receipt.PredictNS = time.Since(started).Nanoseconds()
			result.ModelCalls++
			if err != nil {
				return result, nil, errors.New("path model prediction failed")
			}
			receipt.Proposed, receipt.Confidence, receipt.Probabilities = model.PredictLabel(&prediction), prediction.Confidence, prediction.Probabilities
			allowed := receipt.Proposed == choice.Options[0].Label || receipt.Proposed == choice.Options[1].Label
			switch {
			case prediction.Abstained:
				receipt.Mode = "fallback_low_global_confidence"
			case seed != "":
				var weights [2]float64
				for i, option := range choice.Options {
					for j, label := range labels {
						if option.Label == label {
							weights[i] = float64(prediction.Probabilities[j])
						}
					}
				}
				label, err := sample(result.PlanSHA256, choice, result.MetadataSHA256, result.WeightsSHA256, seed, weights)
				if err != nil {
					return result, nil, err
				}
				receipt.Selected, receipt.Mode = label, "seeded_allowed_probability_sample"
			case allowed:
				receipt.Selected, receipt.Mode = receipt.Proposed, "model_global_argmax"
			default:
				receipt.Mode = "fallback_global_label_outside_allowed"
			}
		}
		result.Choices[choice.ID] = receipt.Selected
		result.Receipts = append(result.Receipts, receipt)
	}
	program, err := assemble(plan, result.Choices)
	if err != nil {
		return result, nil, fmt.Errorf("combined selected path: %w", err)
	}
	return result, program, nil
}

func sample(planSHA string, choice Choice, metadataSHA, weightsSHA, seed string, weights [2]float64) (string, error) {
	var total float64
	for _, weight := range weights {
		if weight < 0 || math.IsNaN(weight) || math.IsInf(weight, 0) {
			return "", errors.New("invalid path probability")
		}
		total += weight
	}
	if total <= 0 || math.IsInf(total, 0) {
		return "", errors.New("empty path probability mass")
	}
	bound := []byte(planSHA + "\x00" + choice.ID + "\x00" + metadataSHA + "\x00" + weightsSHA + "\x00" + seed)
	for i, option := range choice.Options {
		bound = append(bound, 0)
		bound = append(bound, option.Label...)
		var encoded [8]byte
		binary.BigEndian.PutUint64(encoded[:], math.Float64bits(weights[i]/total))
		bound = append(bound, encoded[:]...)
	}
	sum := sha256.Sum256(bound)
	draw := float64(binary.BigEndian.Uint64(sum[:8])>>11) / (1 << 53)
	if draw < weights[0]/total {
		return choice.Options[0].Label, nil
	}
	return choice.Options[1].Label, nil
}
