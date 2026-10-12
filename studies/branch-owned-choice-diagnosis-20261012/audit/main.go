// Read the original, immutable study observations; no model or compiler is loaded.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/bits"
	"os"
	"path/filepath"
)

var retainedSHA = map[string]string{
	"corpus.json.gz":                     "8daf73f5cdd96411fe5ef8d14b852a5b8b2732267df31c8bad400445bc89847c",
	"v3-prose-neutral/observations.json": "eccbd50ab41a2dd668dd9bf4c6953e7d487c5921e80554b79f71ea1327f9f01b",
	"v4-branch-owned/observations.json":  "466f49569d01fa79c0fb6749ad99781cf937549b0258bd8820c38f8077721671",
}

type sourceRecord struct {
	Specification struct {
		ID    string `json:"id"`
		Split string `json:"split"`
	} `json:"specification"`
	Document struct {
		Plan struct {
			Choices []struct {
				Kind string `json:"kind"`
			} `json:"decisions"`
		} `json:"path_plan"`
	} `json:"document"`
	Acceptable uint16 `json:"acceptable"`
	Candidates []struct {
		Mask       uint16 `json:"mask"`
		Acceptable bool   `json:"acceptable"`
	} `json:"candidates"`
}

type observation struct {
	ID            string    `json:"id"`
	Split         string    `json:"split"`
	FirstMask     uint16    `json:"first_mask"`
	CheckedMask   uint16    `json:"checked_mask"`
	Probabilities []float64 `json:"ranking_probabilities"`
	First         struct {
		Complete bool `json:"complete"`
	} `json:"first"`
}

type totals struct {
	Rows          int    `json:"rows"`
	Complete      int    `json:"first_complete"`
	ChoiceCorrect [2]int `json:"choice_correct"`
	// The last two dimensions are authored acceptable bit, then selected bit.
	ChoiceConfusion                     [2][2][2]int `json:"choice_confusion"`
	ErrorBits                           [4]int       `json:"error_bits_histogram"`
	SelectedMasks                       [4]int       `json:"selected_masks"`
	AcceptableMasks                     [4]int       `json:"acceptable_masks"`
	MeanMaxProbability                  float64      `json:"mean_max_probability"`
	MeanEntropyBits                     float64      `json:"mean_entropy_bits"`
	MaxProbabilityFactorizationResidual float64      `json:"max_probability_factorization_residual"`
}

func (s *totals) add(o observation, acceptable uint16) error {
	if len(o.Probabilities) != 4 || o.FirstMask > 3 || o.CheckedMask > 3 ||
		bits.OnesCount16(acceptable) != 1 || acceptable > 8 || acceptable != 1<<o.CheckedMask ||
		o.First.Complete != (acceptable&(1<<o.FirstMask) != 0) {
		return fmt.Errorf("invalid saved outcome: %s", o.ID)
	}
	sum, maximum, entropy := 0.0, 0.0, 0.0
	for _, p := range o.Probabilities {
		if math.IsNaN(p) || math.IsInf(p, 0) || p < 0 || p > 1 {
			return fmt.Errorf("invalid saved probability: %s", o.ID)
		}
		sum += p
		maximum = math.Max(maximum, p)
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}
	if math.Abs(sum-1) > 1e-6 || maximum-o.Probabilities[o.FirstMask] > 1e-7 {
		return fmt.Errorf("inconsistent saved distribution: %s", o.ID)
	}
	s.Rows++
	if o.First.Complete {
		s.Complete++
	}
	errorBits := o.FirstMask ^ o.CheckedMask
	s.ErrorBits[errorBits]++
	s.SelectedMasks[o.FirstMask]++
	s.AcceptableMasks[o.CheckedMask]++
	for choice := range 2 {
		want, got := o.CheckedMask>>choice&1, o.FirstMask>>choice&1
		s.ChoiceConfusion[choice][want][got]++
		if want == got {
			s.ChoiceCorrect[choice]++
		}
	}
	s.MeanMaxProbability += maximum
	s.MeanEntropyBits += entropy
	p := o.Probabilities
	s.MaxProbabilityFactorizationResidual = math.Max(s.MaxProbabilityFactorizationResidual, math.Abs(p[0]*p[3]-p[1]*p[2]))
	return nil
}

func readRetained(root, name string) ([]byte, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		return nil, err
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != retainedSHA[name] {
		return nil, fmt.Errorf("retained file identity changed: %s", name)
	}
	return raw, nil
}

func corpus(root string) (map[string]sourceRecord, error) {
	raw, err := readRetained(root, "corpus.json.gz")
	if err != nil {
		return nil, err
	}
	z, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer z.Close()
	d := json.NewDecoder(io.LimitReader(z, 256<<20))
	rows := make(map[string]sourceRecord, 1536)
	for {
		var row sourceRecord
		if err := d.Decode(&row); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		id := row.Specification.ID
		if _, exists := rows[id]; exists || id == "" || len(rows) == 1536 {
			return nil, fmt.Errorf("duplicate, empty or excess corpus ID: %s", id)
		}
		choices := row.Document.Plan.Choices
		if len(choices) != 2 || choices[0].Kind != "operand_order" || choices[1].Kind != "branch_layout" || len(row.Candidates) != 4 {
			return nil, fmt.Errorf("source choice order changed: %s", id)
		}
		var acceptable uint16
		for mask, candidate := range row.Candidates {
			if candidate.Mask != uint16(mask) {
				return nil, fmt.Errorf("candidate identity: %s", id)
			}
			if candidate.Acceptable {
				acceptable |= 1 << mask
			}
		}
		if acceptable != row.Acceptable || bits.OnesCount16(acceptable) != 1 {
			return nil, fmt.Errorf("ambiguous saved acceptance: %s", id)
		}
		rows[id] = row
	}
	if len(rows) != 1536 {
		return nil, fmt.Errorf("corpus count: %d", len(rows))
	}
	return rows, nil
}

func auditProfile(root, profile string, rows map[string]sourceRecord) (map[string]totals, error) {
	raw, err := readRetained(root, profile+"/observations.json")
	if err != nil {
		return nil, err
	}
	var observations []observation
	if err := json.Unmarshal(raw, &observations); err != nil {
		return nil, err
	}
	if len(observations) != len(rows) {
		return nil, fmt.Errorf("observation count")
	}
	seen := make(map[string]bool, len(rows))
	result := make(map[string]totals, 3)
	for _, o := range observations {
		row, ok := rows[o.ID]
		if !ok || seen[o.ID] || row.Specification.Split != o.Split {
			return nil, fmt.Errorf("observation/source join: %s", o.ID)
		}
		seen[o.ID] = true
		state := result[o.Split]
		if err := state.add(o, row.Acceptable); err != nil {
			return nil, err
		}
		result[o.Split] = state
	}
	for split, state := range result {
		want := map[string]int{"train": 768, "validation": 384, "evaluation": 384}[split]
		if state.Rows != want {
			return nil, fmt.Errorf("split count: %s", split)
		}
		state.MeanMaxProbability /= float64(state.Rows)
		state.MeanEntropyBits /= float64(state.Rows)
		// Reports use descriptive six-decimal values; no exact cross-libm claim.
		state.MeanMaxProbability = math.Round(state.MeanMaxProbability*1e6) / 1e6
		state.MeanEntropyBits = math.Round(state.MeanEntropyBits*1e6) / 1e6
		state.MaxProbabilityFactorizationResidual = math.Round(state.MaxProbabilityFactorizationResidual*1e12) / 1e12
		result[split] = state
	}
	return result, nil
}

func run(root string) error {
	rows, err := corpus(root)
	if err != nil {
		return err
	}
	profiles := make(map[string]map[string]totals, 2)
	for _, profile := range []string{"v3-prose-neutral", "v4-branch-owned"} {
		profiles[profile], err = auditProfile(root, profile, rows)
		if err != nil {
			return err
		}
	}
	report := struct {
		Schema              string                       `json:"schema"`
		Fits                int                          `json:"fits"`
		ModelCalls          int                          `json:"model_calls"`
		CandidateExecutions int                          `json:"candidate_executions"`
		Choices             [2]string                    `json:"choice_order"`
		SHA                 map[string]string            `json:"retained_sha256"`
		Profiles            map[string]map[string]totals `json:"profiles"`
	}{Schema: "gooo/retained-choice-diagnosis/v1", Choices: [2]string{"operand_order", "branch_layout"}, SHA: retainedSHA, Profiles: profiles}
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	return e.Encode(report)
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: choice-audit <retained-result-directory>")
		os.Exit(2)
	}
	if err := run(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
