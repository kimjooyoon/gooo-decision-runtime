package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
)

type pairOutcome struct {
	ID              string `json:"id"`
	Contract        string `json:"contract"`
	NeutralComplete bool   `json:"neutral_first_complete"`
	OwnedComplete   bool   `json:"owned_first_complete"`
	NeutralMask     uint16 `json:"neutral_mask"`
	OwnedMask       uint16 `json:"owned_mask"`
}

type identityGroup struct {
	Acceptable uint64
	Split      string
	Conflict   bool
	CrossSplit bool
}

// Read retained observations only. No fit, predictor, body or native call.
func audit(root string) error {
	rows, corpusSHA, err := load(root)
	if err != nil {
		return err
	}
	observations := [2]map[string]observation{}
	observationSHA := map[string]string{}
	for index, profile := range []string{"v3-prose-neutral", "v4-branch-owned"} {
		raw, err := os.ReadFile(filepath.Join(root, profile, "observations.json"))
		if err != nil {
			return err
		}
		observationSHA[profile] = fmt.Sprintf("%x", sha256.Sum256(raw))
		var items []observation
		if err = json.Unmarshal(raw, &items); err != nil {
			return err
		}
		if len(items) != 1536 {
			return fmt.Errorf("observation denominator")
		}
		observations[index] = map[string]observation{}
		for _, o := range items {
			if _, ok := observations[index][o.ID]; ok {
				return fmt.Errorf("duplicate observation")
			}
			observations[index][o.ID] = o
		}
	}
	groups := [2]map[string]identityGroup{{}, {}}
	pairs := make([]pairOutcome, 0, 384)
	wins, losses, both, neither := 0, 0, 0, 0
	for _, row := range rows {
		var accepted uint64
		for mask, c := range row.Candidates {
			if c.Mask != uint16(mask) || len(c.Outputs) != 8 || len(c.Conditions) != 3 || len(c.Probes) != 4 {
				return fmt.Errorf("candidate shape")
			}
			acceptable := true
			for _, r := range c.Outputs {
				if r.Passed != (r.Actual == r.Expected) {
					return fmt.Errorf("output observation differs")
				}
				acceptable = acceptable && r.Passed
			}
			for _, r := range c.Conditions {
				match := r.Observation.Reached && r.Observation.Value == r.Case.Expected
				if r.Passed != match {
					return fmt.Errorf("condition observation differs")
				}
				acceptable = acceptable && match
			}
			for _, r := range c.Probes {
				if r.Passed != (r.Actual == r.Expected) {
					return fmt.Errorf("probe observation differs")
				}
			}
			if acceptable != c.Acceptable {
				return fmt.Errorf("candidate acceptance differs")
			}
			if acceptable {
				accepted |= 1 << mask
			}
		}
		if accepted != row.Acceptable || accepted == 0 {
			return fmt.Errorf("accepted set differs")
		}
		for profile := range 2 {
			o, ok := observations[profile][row.Specification.ID]
			if !ok || o.Split != row.Specification.Split || o.Contract != row.Specification.Contract || o.FirstMask > 3 || o.CheckedMask > 3 || o.Attempts < 1 || o.Attempts > 4 {
				return fmt.Errorf("observation binding")
			}
			if !reflect.DeepEqual(o.First, measured(row.Candidates[o.FirstMask])) || !reflect.DeepEqual(o.Checked, measured(row.Candidates[o.CheckedMask])) || !reflect.DeepEqual(o.Fixed, measured(row.Candidates[0])) {
				return fmt.Errorf("retained outcome differs")
			}
			values := row.V3
			if profile == 1 {
				values = row.V4
			}
			h := sha256.New()
			var cell [4]byte
			add := func(v float32) { binary.LittleEndian.PutUint32(cell[:], math.Float32bits(v)); h.Write(cell[:]) }
			for _, input := range values {
				for _, v := range input {
					add(v)
				}
			}
			for _, input := range row.Outputs {
				for _, v := range input {
					add(v)
				}
			}
			for _, input := range row.Conditions {
				for _, v := range input {
					add(v)
				}
			}
			key := fmt.Sprintf("%x", h.Sum(nil))
			g, exists := groups[profile][key]
			if !exists {
				g = identityGroup{Acceptable: accepted, Split: row.Specification.Split}
			} else {
				g.Acceptable &= accepted
				g.Conflict = g.Conflict || g.Acceptable == 0
				g.CrossSplit = g.CrossSplit || g.Split != row.Specification.Split
			}
			groups[profile][key] = g
		}
		if row.Specification.Split != "evaluation" {
			continue
		}
		a, b := observations[0][row.Specification.ID], observations[1][row.Specification.ID]
		pairs = append(pairs, pairOutcome{row.Specification.ID, row.Specification.Contract, a.First.Complete, b.First.Complete, a.FirstMask, b.FirstMask})
		switch {
		case a.First.Complete && b.First.Complete:
			both++
		case b.First.Complete:
			wins++
		case a.First.Complete:
			losses++
		default:
			neither++
		}
	}
	identities := map[string]map[string]int{}
	for i, name := range []string{"v3-prose-neutral", "v4-branch-owned"} {
		conflicts, cross := 0, 0
		for _, g := range groups[i] {
			if g.Conflict {
				conflicts++
			}
			if g.CrossSplit {
				cross++
			}
		}
		identities[name] = map[string]int{"distinct_raw_inputs": len(groups[i]), "conflicting_accepted_mask_groups": conflicts, "cross_split_groups": cross}
	}
	if len(pairs) != 384 {
		return fmt.Errorf("paired evaluation denominator")
	}
	if err = writeJSON(filepath.Join(root, "paired-evaluation.json"), pairs); err != nil {
		return err
	}
	return writeJSON(filepath.Join(root, "audit.json"), map[string]any{"schema": "gooo/branch-owned-retained-audit/v1", "corpus_sha256": corpusSHA, "observation_sha256": observationSHA, "documents": 1536, "candidate_histories": 6144, "evaluation_rows": 384, "evaluation_contracts": 24, "evaluation_families": 4, "v4_only_complete": wins, "neutral_only_complete": losses, "both_complete": both, "neither_complete": neither, "raw_input_identity": identities, "raw_input_float32_count": 1408, "model_calls": 0, "candidate_executions": 0, "fits": 0, "scope": "same retained paired rows and exact raw inputs; sibling row rates are descriptive, representation equality does not establish learnability"})
}
