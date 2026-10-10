package bodyplan

import "errors"

// ConditionObservation watches one if statement in the exact compiled arena.
// Reached=false means its branch was skipped; Value then carries no evidence.
// Statement and Expression indices are local to this plan, not stable source IDs.
type ConditionObservation struct {
	Statement  int  `json:"statement"`
	Expression int  `json:"expression"`
	Reached    bool `json:"reached"`
	Value      bool `json:"value"`
}

// ObserveCondition executes the whole program and reads an existing if condition
// at its actual execution point. Ordinary lexical slots, integer arithmetic and
// short-circuit behavior come from the unchanged evaluator. A skipped condition
// is never evaluated separately. State is local to this call; retained programs
// are safe for concurrent readers. No model or dynamic trace buffer is involved.
// On error, neither returned value nor observation is usable.
func (program Program) ObserveCondition(input int64, statement int) (Value, ConditionObservation, error) {
	if program.goSource == "" || program.goooSource == "" || statement < 0 ||
		statement >= len(program.plan.Statements) || program.plan.Statements[statement].Kind != StmtIf {
		return Value{}, ConditionObservation{}, errors.New("observation requires an if statement in a compiled program")
	}
	observation := ConditionObservation{Statement: statement, Expression: program.plan.Statements[statement].Expr}
	var slots [maxStmts]Value
	value, returned, err := observeConditionSequence(&program, program.plan.Root, 0, input, &slots, &observation)
	if err != nil {
		return Value{}, ConditionObservation{}, err
	}
	if !returned || value.Type != program.plan.ResultType {
		return Value{}, ConditionObservation{}, errors.New("observed body did not return its declared result type")
	}
	return value, observation, nil
}

func observeConditionSequence(program *Program, indices []int, scopeID int, input int64,
	slots *[maxStmts]Value, observation *ConditionObservation) (Value, bool, error) {
	for _, index := range indices {
		statement := program.plan.Statements[index]
		if statement.Kind != StmtIf {
			value, returned, err := executeSequence(program, []int{index}, scopeID, input, slots)
			if err != nil || returned {
				return value, returned, err
			}
			continue
		}
		condition, err := evalExpression(program, scopeID, statement.Expr, input, slots)
		if err != nil {
			return Value{}, false, err
		}
		if index == observation.Statement {
			observation.Reached, observation.Value = true, condition.Bool
		}
		branch, branchIndex := statement.Else, 1
		if condition.Bool {
			branch, branchIndex = statement.Then, 0
		}
		value, returned, err := observeConditionSequence(program, branch,
			int(program.branchScopes[index][branchIndex]), input, slots, observation)
		if err != nil || returned {
			return value, returned, err
		}
	}
	return Value{}, false, nil
}
