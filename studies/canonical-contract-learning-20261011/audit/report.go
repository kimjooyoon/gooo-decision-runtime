package main

import (
	"context"
	"reflect"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-contract-learning-20261011/record"
)

type trainingRecord struct {
	ID, PlanSHA, CasesSHA         string
	LegacyInputs, CanonicalInputs [][528]float32
	Outputs, Conditions           [][32]float32
	Acceptable                    uint64
}

func checkTrainingAndModels(root string, sources []r.Source, report r.Report) {
	training := decode[[]trainingRecord](load(root, "training.json"))
	var expected []trainingRecord
	for _, x := range sources {
		if x.Spec.Split == "train" {
			expected = append(expected, trainingRecord{x.Spec.ID, x.PlanSHA, x.CasesSHA, x.LegacyInputs, x.CanonicalInputs, x.OutputRows, x.ConditionRows, x.Acceptable})
		}
	}
	need(len(training) == 64 && reflect.DeepEqual(training, expected), "training contains only authored training rows")
	need(report.Options == (contractdecision.FitOptions{Epochs: 2000, LearningRate: .3, L2: .0001, Seed: 31}) && len(report.Models) == 2, "fixed fit settings")
	for _, name := range []string{"legacy", "canonical"} {
		raw := load(root, "model-"+name+".json")
		m := report.Models[name]
		need(m.SHA == r.Hash(raw) && m.ArtifactBytes == len(raw) && m.Parameters == 19034 && m.WeightBytes == 76136 && m.FitNS > 0, "saved model identity")
		if name == "legacy" {
			model, err := contractdecision.DecodeInteractionRequirementConditioned(raw)
			must(err)
			need(model.Fingerprint() == m.Fingerprint && model.ArtifactSchema() == m.Schema && model.Pooling() == contractdecision.MeanPooling, "legacy artifact ABI")
		} else {
			model, err := contractdecision.DecodeCanonicalInteractionRequirementConditioned(raw)
			must(err)
			need(model.Fingerprint() == m.Fingerprint && model.ArtifactSchema() == m.Schema && model.Pooling() == contractdecision.MeanPooling, "canonical artifact ABI")
		}
		history := decode[[]contractdecision.Epoch](load(root, "history-"+name+".json"))
		need(len(history) == 2000 && history[0].Loss == m.FirstLoss && history[1999].Loss == m.LastLoss, "full loss history")
		for i, h := range history {
			need(h.Number == i+1 && h.Loss >= 0, "epoch sequence")
		}
	}
	started := decode[map[string]string](load(root, "started.json"))
	completed := decode[map[string]string](load(root, "completed.json"))
	need(started["producer"] == report.Producer && completed["producer"] == report.Producer && started["compiler"] == r.Compiler && started["corpus"] == r.CorpusSHA, "original execution markers")
}

func checkAudits(sources []r.Source, report r.Report) {
	for _, format := range []string{"legacy", "canonical"} {
		for _, scope := range []string{"declared", "sparse_probes"} {
			var samples []contractdecision.OrderedRequirementSample
			for _, x := range sources {
				if scope == "sparse_probes" && x.Spec.Split != "sparse" {
					continue
				}
				inputs, acceptable := x.LegacyInputs, x.Acceptable
				if format == "canonical" {
					inputs = x.CanonicalInputs
				}
				if scope == "sparse_probes" {
					acceptable = x.ProbeAcceptable
				}
				samples = append(samples, contractdecision.OrderedRequirementSample{Inputs: inputs, Cases: r.Rows(x.OutputRows), Conditions: r.Rows(x.ConditionRows), Masks: []uint16{0, 1, 2, 3}, Acceptable: acceptable})
			}
			var got *contractdecision.InputAudit
			var err error
			if format == "canonical" {
				got, err = contractdecision.AuditCanonicalRequirements(context.Background(), samples)
			} else {
				got, err = contractdecision.AuditOrderedRequirements(context.Background(), samples)
			}
			must(err)
			need(reflect.DeepEqual(got, report.InputAudits[format+"_"+scope]), "stored input-information bound")
			if scope == "sparse_probes" {
				need(got.BestFirstPasses == 16 && got.UnavoidableMisses == 48, "sparse hidden goal information bound")
			}
		}
	}
}

func recount(groups map[string]r.Group, x r.Source, row r.Search) {
	initial, final := row.Stages[0], row.Stages[len(row.Stages)-1]
	first := initial.Progress[1].NewAttempts[0]
	initialLast, finalLast := initial.Progress[len(initial.Progress)-1], final.Progress[len(final.Progress)-1]
	probe := x.Candidates[first.Mask].Probe
	for _, key := range []string{row.Mode + "/all", row.Mode + "/" + row.Split, row.Mode + "/" + row.Split + "/" + row.Form, row.Mode + "/family-" + row.Family} {
		g := groups[key]
		g.Programs++
		if x.Candidates[first.Mask].Acceptable {
			g.FirstDeclaredComplete++
		}
		if x.Candidates[first.Mask].ProbeAcceptable {
			g.FirstProbeComplete++
		}
		if initialLast.Status == "TRAINING_COMPLETE" {
			g.InitialDeclaredComplete++
		}
		if r.Passed(initial.Probe) {
			g.InitialProbeComplete++
		}
		if finalLast.Status == "TRAINING_COMPLETE" {
			g.FinalDeclaredComplete++
		}
		if r.Passed(final.Probe) {
			g.FinalProbeComplete++
		}
		g.FirstOutputTotal += len(first.Results)
		for _, c := range first.Results {
			if c.Passed {
				g.FirstOutputPassed++
			}
		}
		g.FirstConditionTotal += len(first.Conditions)
		for _, c := range first.Conditions {
			if c.Passed {
				g.FirstConditionPassed++
			}
		}
		g.FirstProbeOutputTotal += len(probe.Outputs)
		for _, c := range probe.Outputs {
			if c.Passed {
				g.FirstProbeOutputPassed++
			}
		}
		g.FirstProbeConditionTotal += len(probe.Conditions)
		for _, c := range probe.Conditions {
			if c.Passed {
				g.FirstProbeConditionPassed++
			}
		}
		g.InitialAttempts += initialLast.Attempted
		g.FeedbackStages += len(row.Stages) - 1
		for _, stage := range row.Stages {
			g.TotalAttempts += stage.Progress[len(stage.Progress)-1].Attempted
			g.ModelCalls += stage.Ranking.Calls
			g.SearchNS += stage.NS
			g.PredictNS += stage.Ranking.PredictNS
		}
		groups[key] = g
	}
}
