package main

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
)

type ranker interface {
	PredictInto([][528]float32, contractdecision.CaseSource, contractdecision.ConditionSource, []uint16, *contractdecision.InteractionRequirementWorkspace, *contractdecision.Prediction) error
	Marshal() ([]byte, error)
	Fingerprint() string
}
type measurement struct {
	Complete   bool `json:"complete"`
	Outputs    int  `json:"outputs_passed"`
	Conditions int  `json:"conditions_passed"`
	Probes     int  `json:"probes_passed"`
}
type observation struct {
	ID          string      `json:"id"`
	Split       string      `json:"split"`
	Family      int         `json:"family"`
	Parameter   int         `json:"parameter"`
	Goal        int         `json:"goal"`
	Wording     int         `json:"wording"`
	Form        int         `json:"form"`
	Contract    string      `json:"contract"`
	Parity      int         `json:"parity"`
	Language    string      `json:"language"`
	FirstMask   uint16      `json:"first_mask"`
	CheckedMask uint16      `json:"checked_mask"`
	Probability [4]float32  `json:"ranking_probabilities"`
	First       measurement `json:"first"`
	Checked     measurement `json:"checked"`
	Fixed       measurement `json:"fixed_first"`
	Attempts    int         `json:"candidate_attempts"`
}
type splitStats struct {
	Documents         int `json:"documents"`
	Families          int `json:"families"`
	Contracts         int `json:"contracts"`
	FirstComplete     int `json:"first_complete"`
	FixedComplete     int `json:"fixed_first_complete"`
	CheckedComplete   int `json:"checked_complete"`
	FirstOutputs      int `json:"first_outputs_passed"`
	FirstConditions   int `json:"first_conditions_passed"`
	FirstProbes       int `json:"first_probes_passed"`
	CheckedProbes     int `json:"checked_probes_passed"`
	Attempts          int `json:"candidate_attempts"`
	LanguagePairs     int `json:"language_pairs"`
	LanguageAgreement int `json:"language_first_mask_agreement"`
}

func measured(c candidate) measurement {
	m := measurement{Complete: c.Acceptable}
	for _, r := range c.Outputs {
		if r.Passed {
			m.Outputs++
		}
	}
	for _, r := range c.Conditions {
		if r.Passed {
			m.Conditions++
		}
	}
	for _, r := range c.Probes {
		if r.Passed {
			m.Probes++
		}
	}
	return m
}
func load(root string) ([]record, string, error) {
	raw, err := os.ReadFile(filepath.Join(root, "corpus.json.gz"))
	if err != nil {
		return nil, "", err
	}
	file, err := os.Open(filepath.Join(root, "corpus.json.gz"))
	if err != nil {
		return nil, "", err
	}
	defer file.Close()
	z, err := gzip.NewReader(file)
	if err != nil {
		return nil, "", err
	}
	defer z.Close()
	d := json.NewDecoder(z)
	rows := make([]record, 0, 1536)
	for {
		var row record
		err = d.Decode(&row)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", err
		}
		rows = append(rows, row)
	}
	if len(rows) != 1536 {
		return nil, "", fmt.Errorf("corpus coverage")
	}
	return rows, fmt.Sprintf("%x", sha256.Sum256(raw)), nil
}
func modelInputs(row record, profile string) [2][528]float32 {
	if profile == "v4-branch-owned" {
		return row.V4
	}
	return row.V3
}

func evaluate(ctx context.Context, m ranker, rows []record, profile string) ([]observation, map[string]splitStats, error) {
	results := make([]observation, 0, len(rows))
	stats := map[string]splitStats{}
	families := map[string]map[int]bool{}
	contracts := map[string]map[string]bool{}
	pairs := map[string]map[[6]int][2]int{}
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		inputs := modelInputs(row, profile)
		requirements := requirements{Outputs: row.Outputs, Conditions: row.Conditions}
		var work contractdecision.InteractionRequirementWorkspace
		var prediction contractdecision.Prediction
		if err := m.PredictInto(inputs[:], requirements, requirements, []uint16{0, 1, 2, 3}, &work, &prediction); err != nil {
			return nil, nil, err
		}
		s := row.Specification
		o := observation{ID: s.ID, Split: s.Split, Family: s.Family, Parameter: s.Parameter, Goal: s.Goal, Wording: s.Wording, Form: s.Form, Contract: s.Contract, Parity: s.Parity, Language: s.Language, FirstMask: prediction.Selected, First: measured(row.Candidates[prediction.Selected]), Fixed: measured(row.Candidates[0])}
		copy(o.Probability[:], prediction.Probabilities[:4])
		order := [4]int{0, 1, 2, 3}
		sort.SliceStable(order[:], func(i, j int) bool {
			a, b := order[i], order[j]
			if a == b {
				return false
			}
			if uint16(a) == o.FirstMask {
				return true
			}
			if uint16(b) == o.FirstMask {
				return false
			}
			return o.Probability[a] > o.Probability[b]
		})
		for _, mask := range order {
			o.Attempts++
			if row.Candidates[mask].Acceptable {
				o.CheckedMask = uint16(mask)
				o.Checked = measured(row.Candidates[mask])
				break
			}
		}
		if !o.Checked.Complete {
			return nil, nil, fmt.Errorf("retained palette has no completion")
		}
		state := stats[s.Split]
		state.Documents++
		state.FirstOutputs += o.First.Outputs
		state.FirstConditions += o.First.Conditions
		state.FirstProbes += o.First.Probes
		state.CheckedProbes += o.Checked.Probes
		state.Attempts += o.Attempts
		if o.First.Complete {
			state.FirstComplete++
		}
		if o.Fixed.Complete {
			state.FixedComplete++
		}
		if o.Checked.Complete {
			state.CheckedComplete++
		}
		stats[s.Split] = state
		if families[s.Split] == nil {
			families[s.Split] = map[int]bool{}
			contracts[s.Split] = map[string]bool{}
			pairs[s.Split] = map[[6]int][2]int{}
		}
		families[s.Split][s.Family] = true
		contracts[s.Split][s.Contract] = true
		key := [6]int{s.Family, s.Parameter, s.Goal, s.Wording, s.Parity, s.Form}
		pair, ok := pairs[s.Split][key]
		if !ok {
			pair = [2]int{-1, -1}
		}
		language := 0
		if s.Language == "en" {
			language = 1
		}
		pair[language] = int(o.FirstMask)
		pairs[s.Split][key] = pair
		results = append(results, o)
	}
	for split, state := range stats {
		state.Families = len(families[split])
		state.Contracts = len(contracts[split])
		for _, pair := range pairs[split] {
			if pair[0] < 0 || pair[1] < 0 {
				return nil, nil, fmt.Errorf("missing language sibling")
			}
			state.LanguagePairs++
			if pair[0] == pair[1] {
				state.LanguageAgreement++
			}
		}
		stats[split] = state
	}
	return results, stats, nil
}
func train(root string) error {
	if _, err := os.Stat(filepath.Join(root, "code-verified.txt")); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(root, "collection-complete.json")); err != nil {
		return err
	}
	rows, corpusSHA, err := load(root)
	if err != nil {
		return err
	}
	options := contractdecision.FitOptions{Seed: 2026101227, Epochs: 80, LearningRate: .01, L2: .00001}
	for _, version := range []string{"v3-prose-neutral", "v4-branch-owned"} {
		if err = begin(root, "fit-"+version); err != nil {
			return err
		}
		directory := filepath.Join(root, version)
		if err = os.Mkdir(directory, 0700); err != nil {
			return err
		}
		samples := make([]contractdecision.InteractionRequirementSample, 0, 768)
		for _, row := range rows {
			if row.Specification.Split != "train" {
				continue
			}
			inputs := modelInputs(row, version)
			requirements := requirements{Outputs: row.Outputs, Conditions: row.Conditions}
			samples = append(samples, contractdecision.InteractionRequirementSample{Inputs: inputs[:], Cases: requirements, Conditions: requirements, Masks: []uint16{0, 1, 2, 3}, Acceptable: row.Acceptable})
		}
		if len(samples) != 768 {
			return fmt.Errorf("training split differs")
		}
		if err = writeJSON(filepath.Join(directory, "fit-started.json"), map[string]any{"options": options, "corpus_sha256": corpusSHA, "train_documents": 768, "model": version}); err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		var model ranker
		var history []contractdecision.Epoch
		if version == "v4-branch-owned" {
			model, history, err = contractdecision.FitSourceRelativeV4(ctx, samples, options)
		} else {
			model, history, err = contractdecision.FitSourceRelativeV3ProseNeutral(ctx, samples, options)
		}
		cancel()
		if err != nil {
			return err
		}
		artifact, err := model.Marshal()
		if err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(directory, "model.json"), artifact, 0600); err != nil {
			return err
		}
		if err = writeJSON(filepath.Join(directory, "history.json"), history); err != nil {
			return err
		}
		observations, splits, err := evaluate(context.Background(), model, rows, version)
		if err != nil {
			return err
		}
		if err = writeJSON(filepath.Join(directory, "observations.json"), observations); err != nil {
			return err
		}
		groups, completeContracts := groupedOutcomes(observations)
		if err = writeJSON(filepath.Join(directory, "profile.json"), map[string]any{"profile": version, "prose_cells_cleared": true, "cleared_range": "[64:192]", "ordinary_sdk_weights_reproduce_profile": true, "branch_owned_cells": version == "v4-branch-owned"}); err != nil {
			return err
		}
		if err = writeJSON(filepath.Join(directory, "report.json"), map[string]any{"schema": "gooo/branch-owned-decision-learning/v1", "model": version, "model_fingerprint": model.Fingerprint(), "corpus_sha256": corpusSHA, "fits": 1, "predictions": 1536, "native_executions": 0, "candidate_reexecutions": 0, "splits": splits, "complete_sibling_contracts": completeContracts, "overlapping_evaluation_groups": groups, "parameter_count": contractdecision.InteractionRequirementParameterCount, "tensor_bytes": 4 * contractdecision.InteractionRequirementParameterCount, "scope": "fresh paired neutral/V4 inputs with operator-family split; first and retained finite completion separate; no general prose/native/speed claim"}); err != nil {
			return err
		}
	}
	return nil
}
