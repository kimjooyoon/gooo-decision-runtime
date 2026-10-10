package pathplan

import (
	"errors"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

// ContractInput owns the declared finite requirements alongside initial source
// features. Cases are goals supplied by the caller, not observed execution.
// The compiler must bind that caller data to authoritative Gooo source.
type ContractInput struct {
	initial ConditionInput
	cases   [128]TestCase
	count   int
}

// InitialContractInput preserves every supplied case, including order and
// duplicates. It performs no candidate evaluation or model prediction. The
// fixed integer storage avoids retaining the caller's backing array or a
// precomputed floating-point tensor for the whole suite.
func (p *PreparedPlan) InitialContractInput(cases []TestCase) (*ContractInput, error) {
	initial, err := p.InitialExecutionInput(cases)
	if err != nil {
		return nil, err
	}
	input := &ContractInput{initial: *initial, count: len(cases)}
	copy(input.cases[:], cases)
	return input, nil
}

func (input *ContractInput) PlanSHA256() string {
	if input == nil {
		return ""
	}
	return input.initial.PlanSHA256()
}

func (input *ContractInput) CaseSHA256() string {
	if input == nil {
		return ""
	}
	return input.initial.CaseSHA256()
}

func (input *ContractInput) CaseCount() int {
	if input == nil {
		return 0
	}
	return input.count
}

// RelationalSourceFeaturesInto returns the same initial v6 source array used
// by existing flow models. Declared cases live in their separate explicit ABI.
func (input *ContractInput) RelationalSourceFeaturesInto(id string, output *[decision.ExecutionFlowFeatureDim]float32) error {
	if input == nil || input.count == 0 {
		return errors.New("source-bound declared contract required")
	}
	return input.initial.ExecutionRelationalFlowFeaturesInto(id, output)
}

// CaseFeaturesInto writes exactly one declared case into caller-owned scratch.
// A consumer can reuse one 128-byte destination across all 1..128 cases. Reads
// do not mutate the input; concurrent readers supply separate destinations.
func (input *ContractInput) CaseFeaturesInto(index int, output *[decision.DeclaredCaseFeatureDim]float32) error {
	if input == nil || index < 0 || index >= input.count {
		return errors.New("declared case index outside bound contract")
	}
	c := input.cases[index]
	return decision.DeclaredCaseFeaturesInto(c.Input, c.Expected, output)
}

// CaseFeatures returns a fixed value for streaming model readers. Returning by
// value keeps caller scratch from escaping through an interface method call.
func (input *ContractInput) CaseFeatures(index int) ([decision.DeclaredCaseFeatureDim]float32, error) {
	var row [decision.DeclaredCaseFeatureDim]float32
	err := input.CaseFeaturesInto(index, &row)
	return row, err
}
