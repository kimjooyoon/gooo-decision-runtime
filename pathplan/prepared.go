package pathplan

import (
	"encoding/json"
	"errors"

	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
)

// PreparedPlan owns an immutable snapshot after every offered option has passed
// its individual type/scope check. Combined candidates are still checked when
// compiled. It caches no models, test outcomes or mutable search state.
type PreparedPlan struct {
	plan     Plan
	defaults map[string]string
	fallback *bodyplan.Program
	sha      string
}

func ownedPlan(plan Plan) Plan {
	plan.Base = cloneBase(plan.Base)
	plan.Decisions = append([]Choice(nil), plan.Decisions...)
	for i := range plan.Decisions {
		plan.Decisions[i].Options = append([]Option(nil), plan.Decisions[i].Options...)
		for j := range plan.Decisions[i].Options {
			option := &plan.Decisions[i].Options[j]
			option.Order = append([]int(nil), option.Order...)
		}
	}
	return plan
}

// Prepare validates one bounded snapshot, retaining the already compiled
// fallback for source binding. Callers must not mutate input while preparing it.
// Once prepared, caller mutation and concurrent searches cannot change it.
func Prepare(plan Plan) (*PreparedPlan, error) {
	// Check outer limits before copying caller-owned slices.
	if plan.Schema != Schema || len(plan.Decisions) == 0 || len(plan.Decisions) > 16 ||
		len(plan.Base.Expressions) > 128 || len(plan.Base.Statements) > 128 {
		return nil, errors.New("path plan schema or arena bounds are invalid")
	}
	if len(plan.Base.Root) > 128 {
		return nil, errors.New("path root budget exceeded")
	}
	for _, expression := range plan.Base.Expressions {
		if len(expression.Allowed) > 8 {
			return nil, errors.New("operation option budget exceeded")
		}
	}
	for _, statement := range plan.Base.Statements {
		if len(statement.Then) > 128 || len(statement.Else) > 128 {
			return nil, errors.New("path branch budget exceeded")
		}
	}
	for _, choice := range plan.Decisions {
		if len(choice.Options) != 2 {
			return nil, errors.New("path option budget exceeded")
		}
		for _, option := range choice.Options {
			if len(option.Order) > 128 {
				return nil, errors.New("path order budget exceeded")
			}
		}
	}
	owned := ownedPlan(plan)
	defaults, fallback, err := validatePlan(owned)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(owned)
	if err != nil || len(raw) > 128<<10 {
		return nil, errors.New("serialized path budget exceeded")
	}
	return &PreparedPlan{plan: owned, defaults: defaults, fallback: fallback, sha: hash(raw)}, nil
}

func (prepared *PreparedPlan) PlanSHA256() string   { return prepared.sha }
func (prepared *PreparedPlan) ActivityName() string { return prepared.plan.Base.Name }

// Fallback is immutable; Program exposes no writable arena or generated body.
func (prepared *PreparedPlan) Fallback() *bodyplan.Program { return prepared.fallback }

// Defaults returns caller-owned choices, never the prepared snapshot's map.
func (prepared *PreparedPlan) Defaults() map[string]string {
	return cloneChoices(prepared.defaults)
}

// Compile checks the complete selected program, including interacting edits.
// It does not repeat the snapshot's individual-option preparation.
func (prepared *PreparedPlan) Compile(choices map[string]string) (*bodyplan.Program, error) {
	if prepared == nil || prepared.fallback == nil {
		return nil, errors.New("prepared path plan is required")
	}
	if len(choices) != len(prepared.defaults) {
		return nil, errors.New("every structural choice must be selected exactly once")
	}
	for _, choice := range prepared.plan.Decisions {
		if choices[choice.ID] != choice.Options[0].Label && choices[choice.ID] != choice.Options[1].Label {
			return nil, errors.New("missing or undeclared structural selection")
		}
	}
	return assemble(prepared.plan, choices)
}
