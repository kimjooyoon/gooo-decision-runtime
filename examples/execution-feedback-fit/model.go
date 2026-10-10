package main

import (
	"context"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/conditiondecision"
	"github.com/kimjooyoon/gooo-decision-runtime/executiondecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

type fittedModel struct {
	v2             *conditiondecision.Model
	v3             *executiondecision.Model
	beforeAblation []byte
}

func modelSamples(rows []inputRecord, version int) any {
	if version == 0 {
		var samples []conditiondecision.Sample
		for _, r := range trainingRows(rows) {
			inputs := make([][256]float32, len(r.Features[0]))
			for i := range inputs {
				copy(inputs[i][:], r.Features[0][i][:256])
			}
			samples = append(samples, conditiondecision.Sample{Inputs: inputs, Masks: r.Masks, Acceptable: r.Acceptable})
		}
		return samples
	}
	var samples []executiondecision.Sample
	for _, r := range trainingRows(rows) {
		samples = append(samples, executiondecision.Sample{Inputs: r.Features[version], Masks: r.Masks, Acceptable: r.Acceptable})
	}
	return samples
}

func fitModel(ctx context.Context, rows []inputRecord, version int, options conditiondecision.FitOptions) (fittedModel, []conditiondecision.Epoch, error) {
	if version == 0 {
		m, h, err := conditiondecision.FitForFeatures(ctx, modelSamples(rows, version).([]conditiondecision.Sample), options, decision.ConditionBranchFeatureVersion)
		return fittedModel{v2: m}, h, err
	}
	m, h, err := executiondecision.Fit(ctx, modelSamples(rows, version).([]executiondecision.Sample), executiondecision.FitOptions(options))
	history := make([]conditiondecision.Epoch, len(h))
	for i, row := range h {
		history[i] = conditiondecision.Epoch(row)
	}
	if err != nil {
		return fittedModel{}, history, err
	}
	var beforeAblation []byte
	if version == 1 {
		beforeAblation, err = m.Marshal()
		if err != nil {
			return fittedModel{}, history, err
		}
		// Fixed ablation: these weights never saw nonzero input during fitting.
		// Removing them keeps every training prediction unchanged and lets the
		// normal search supply real observations without enabling the channel.
		weights := m.Weights()
		for hidden := range executiondecision.HiddenDim {
			clear(weights[hidden*320+256 : (hidden+1)*320])
		}
		m, err = executiondecision.New(weights)
		if err != nil {
			return fittedModel{}, history, err
		}
	}
	return fittedModel{v3: m, beforeAblation: beforeAblation}, history, nil
}

func (m fittedModel) predict(inputs [][320]float32, masks []uint16, output *conditiondecision.Prediction) error {
	if m.v2 != nil {
		var prefix [16][256]float32
		for i := range inputs {
			copy(prefix[i][:], inputs[i][:256])
		}
		var workspace conditiondecision.Workspace
		return m.v2.PredictInto(prefix[:len(inputs)], masks, &workspace, output)
	}
	var workspace executiondecision.Workspace
	var prediction executiondecision.Prediction
	if err := m.v3.PredictInto(inputs, masks, &workspace, &prediction); err != nil {
		return err
	}
	*output = conditiondecision.Prediction(prediction)
	return nil
}

func (m fittedModel) marshal() ([]byte, error) {
	if m.v2 != nil {
		return m.v2.Marshal()
	}
	return m.v3.Marshal()
}
func (m fittedModel) fingerprint() string {
	if m.v2 != nil {
		return m.v2.Fingerprint()
	}
	return m.v3.Fingerprint()
}
func decodeModel(raw []byte, version int) (fittedModel, error) {
	if version == 0 {
		m, err := conditiondecision.Decode(raw)
		return fittedModel{v2: m}, err
	}
	m, err := executiondecision.Decode(raw)
	return fittedModel{v3: m}, err
}
func (m fittedModel) search(ctx context.Context, p *pathplan.PreparedPlan, cases []pathplan.TestCase, budget, rounds int) (pathplan.SearchResult, *bodyplan.Program, []pathplan.ConditionProgress, []pathplan.ConditionRanking, error) {
	if m.v3 != nil {
		return p.SearchExecutionBatches(ctx, m.v3, cases, budget, 1, "", rounds)
	}
	return p.SearchConditionBatches(ctx, m.v2, cases, budget, 1, "", rounds)
}
