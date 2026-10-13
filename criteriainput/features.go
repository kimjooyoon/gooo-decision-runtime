package criteriainput

import (
	"errors"
	"math"
)

const FeatureSchema = "gooo/scalar-criteria-features/v1"
const ChoiceFeatureDim = 256
const PredicateFeatureDim = 256

// Features keeps choice and predicate rows separate. Binding metadata is outside
// numerical rows. Unused bounded rows are zero; counts determine active rows.
// The representation is lossy and requires its own fitted model contract.
type Features struct {
	Schema              string                           `json:"schema"`
	InputSHA256         string                           `json:"input_sha256"`
	CandidatePlanSHA256 string                           `json:"candidate_plan_sha256"`
	ChoiceCount         uint8                            `json:"choice_count"`
	PredicateCount      uint8                            `json:"predicate_count"`
	Choices             [MaxChoices]ChoiceFeatures       `json:"choices"`
	Predicates          [MaxPredicates]PredicateFeatures `json:"predicates"`
}

type ChoiceFeatures struct {
	ID       string                    `json:"id"`
	Options  [2]string                 `json:"options"`
	Fallback string                    `json:"fallback"`
	Values   [ChoiceFeatureDim]float32 `json:"values"`
}

type PredicateFeatures struct {
	ID     string                       `json:"id"`
	Values [PredicateFeatureDim]float32 `json:"values"`
}

// FeaturesInto reads prepared source only. It compiles or executes no candidate,
// evaluates no requirement and makes no prediction. On error output is unchanged.
// Existing SourceFeatures may decline ambiguous or unreachable choice targets.
func (p *Prepared) FeaturesInto(output *Features) error {
	if p == nil || output == nil {
		return errors.New("prepared scalar source and feature output required")
	}
	candidate := Features{Schema: FeatureSchema, InputSHA256: p.digest,
		CandidatePlanSHA256: p.paths.PlanSHA256(), ChoiceCount: p.choiceCount, PredicateCount: p.graphCount}
	for i, choice := range p.choices[:p.choiceCount] {
		fields, err := p.paths.SourceFeatures(choice.id)
		if err != nil {
			return err
		}
		row := &candidate.Choices[i]
		row.ID, row.Options, row.Fallback = choice.id, choice.options, choice.fallback
		for j, value := range fields {
			row.Values[j] = float32(value)
		}
		featureNormalize(row.Values[:64])
		intentFeatures(choice.intent, row.Values[64:])
		featureNormalize(row.Values[64:])
	}
	for i, graph := range p.graphs[:p.graphCount] {
		row := &candidate.Predicates[i]
		row.ID = p.predicateIDs[i]
		p.graphFeatures(graph, &row.Values)
	}
	*output = candidate
	return nil
}

func featureNormalize(values []float32) {
	var squared float64
	for _, value := range values {
		squared += float64(value) * float64(value)
	}
	if squared == 0 {
		return
	}
	scale := float32(1 / math.Sqrt(squared))
	for i := range values {
		values[i] *= scale
	}
}
