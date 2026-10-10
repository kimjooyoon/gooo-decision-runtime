package main

import (
	"context"
	"math/bits"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/flowdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

const initialTrainingSHA = "e62fd490c2224aef24c25906e0768e666390d8e4f1b6381258bcb24af64251c4"

type trainingObservation struct {
	ID, SourceSHA, PlanSHA, CaseSHA, InputSHA string
	SampleIndex                               int
	NS                                        int64
	Mask                                      uint16
	ConditionPresent, OutputPresent           bool
	Condition                                 pathplan.ConditionFailure
	Output                                    pathplan.OutputFailure
}

func rejectedMasks(row prepared) []uint16 {
	if row.Split != "future_train" {
		return nil
	}
	require(row.Form == "direct" && bits.OnesCount64(row.Acceptable) == 1 && row.Acceptable < 16, "one accepted training mask")
	var masks []uint16
	for mask := range uint16(4) {
		if row.Acceptable>>mask&1 == 0 {
			masks = append(masks, mask)
		}
	}
	return masks
}

// Only these 48 declared training candidates are executed. Labels come from
// the frozen complete candidate acceptance table, never from a model proposal.
func collectTraining(ctx context.Context, rows []prepared, out string, r *report) []flowdecision.Sample {
	samples := training(rows)
	require(len(samples) == 16 && hash(encoded(samples)) == initialTrainingSHA, "unchanged initial training bytes")
	f := rowFile(out, "training-observations.jsonl")
	for _, row := range rows {
		for _, mask := range rejectedMasks(row) {
			choices := map[string]string{}
			for i, c := range row.Document.Plan.Decisions {
				choices[c.ID] = c.Options[mask>>i&1].Label
			}
			start := time.Now()
			input, err := row.plan.ObserveExecutionInput(ctx, choices, row.Document.TestCases)
			ns := time.Since(start).Nanoseconds()
			must(err)
			record := trainingObservation{ID: row.ID, SourceSHA: row.SourceSHA, PlanSHA: input.PlanSHA256(), CaseSHA: input.CaseSHA256(), SampleIndex: len(samples), Mask: mask}
			record.Condition, record.ConditionPresent = input.Failure()
			record.NS = ns
			record.Output, record.OutputPresent = input.OutputFailure()
			require(record.ConditionPresent || record.OutputPresent, "rejected candidate has actual failure")
			features := make([][384]float32, len(row.Document.Plan.Decisions))
			for i, c := range row.Document.Plan.Decisions {
				must(input.ExecutionRelationalFlowFeaturesInto(c.ID, &features[i]))
				for j, value := range features[i] {
					if !observationChannel(j) {
						require(value == row.V6[i][j], "observation preserves all source channels")
					}
				}
			}
			record.InputSHA = featureSHA(features, 384)
			appendRow(f, record)
			samples = append(samples, flowdecision.Sample{Inputs: features, Masks: []uint16{0, 1, 2, 3}, Acceptable: row.Acceptable})
			r.TrainingObservations++
		}
	}
	must(f.Close())
	return samples
}

func observationChannel(i int) bool {
	return (i >= 192 && i < 236) || (i >= 256 && i < 320)
}
