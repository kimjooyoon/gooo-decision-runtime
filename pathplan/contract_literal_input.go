package pathplan

import (
	"errors"
	"slices"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

// InitialContractInputFor binds an explicit case ABI to the complete source and
// authored cases. The literal profile reads every Int expression in the bounded
// base arena, including unexecuted branches, without evaluating any candidate.
func (p *PreparedPlan) InitialContractInputFor(cases []TestCase, version string) (*ContractInput, error) {
	if version != decision.DeclaredCaseFeatureVersion && version != decision.SourceLiteralCaseFeatureVersion {
		return nil, errors.New("unsupported declared case feature version")
	}
	input, err := p.InitialContractInput(cases)
	if err != nil || version == decision.DeclaredCaseFeatureVersion {
		return input, err
	}
	input.literals = new([decision.MaxSourceCaseLiterals]int64)
	for _, expression := range p.plan.Base.Expressions {
		if expression.Kind != "int" || slices.Contains(input.literals[:input.literalCount], expression.Int) {
			continue
		}
		if input.literalCount == len(input.literals) {
			return nil, errors.New("source literal budget exceeded")
		}
		input.literals[input.literalCount] = expression.Int
		input.literalCount++
	}
	slices.Sort(input.literals[:input.literalCount])
	return input, nil
}

func (input *ContractInput) CaseFeatureVersion() string {
	if input != nil && input.literals != nil {
		return decision.SourceLiteralCaseFeatureVersion
	}
	return decision.DeclaredCaseFeatureVersion
}

// SourceLiteralsInto copies the complete canonical source-literal set, allowing
// callers to inspect the projection without sharing mutable input storage.
func (input *ContractInput) SourceLiteralsInto(output *[decision.MaxSourceCaseLiterals]int64) (int, error) {
	if input == nil || input.literals == nil || output == nil {
		return 0, errors.New("source literal contract and destination required")
	}
	*output = *input.literals
	return input.literalCount, nil
}
