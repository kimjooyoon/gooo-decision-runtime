// Read frozen input arrays and outcomes. This program imports no model or compiler.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
)

const corpusHash = "9d98229364e904a0ff397aaa324d751cf3e82ae012311d7faa43d428f76e5cdc"

var observationHashes = map[string]string{
	"v2-full":          "2d21286f0ba755355db419fb208ecbb6c704cdde197640d9f5dcccc01d2f7b41",
	"v3-full":          "b9d26913fb4e2f3ada33aa29aaf12b7308691a395a66e8f64538784acbcea54c",
	"v3-prose-neutral": "d533cfe1163a2db278b0772f9950a62e4ee0b45a513be3b491dfa6bd6470cfb3",
}

type specification struct {
	ID, Split, Contract string
	Family              int
}
type candidate struct {
	Mask       uint16
	Acceptable bool
	Outputs    []struct {
		Input, Expected, Actual int64
		Passed                  bool
	}
	Conditions []struct {
		Case        struct{ Expected bool }
		Observation struct{ Reached, Value bool }
		Passed      bool
	}
}
type row struct {
	Specification specification
	V2            [2][528]float32 `json:"v2_inputs"`
	V3            [2][528]float32 `json:"v3_inputs"`
	Outputs       [6][32]float32
	Conditions    [3][32]float32
	Candidates    [4]candidate
	Acceptable    uint64
}
type measurement struct {
	Complete   bool
	Outputs    int `json:"outputs_passed"`
	Conditions int `json:"conditions_passed"`
}
type observation struct {
	ID, Split, Contract string
	Family              int
	FirstMask           uint16 `json:"first_mask"`
	First               measurement
}
type group struct {
	Rows         int
	Intersection uint64
	Allowed      [4]int
	Splits       map[string]bool
}
type collisionSummary struct {
	Rows                   int `json:"rows"`
	UniqueInputs           int `json:"unique_raw_inputs"`
	ConflictingGroups      int `json:"conflicting_groups"`
	ConflictingRows        int `json:"rows_in_conflicting_groups"`
	CrossSplitGroups       int `json:"cross_split_input_groups"`
	SharedChoiceUpperBound int `json:"raw_input_shared_choice_upper_bound_rows"`
}
type familySummary struct {
	Family                      int       `json:"family"`
	Operations                  [2]string `json:"ordered_operations"`
	Rows                        int       `json:"rows"`
	Contracts                   int       `json:"goal_contracts"`
	FirstComplete               int       `json:"first_complete_rows"`
	AllConditionsButIncomplete  int       `json:"all_conditions_but_incomplete_rows"`
	AllVariantsCorrectContracts int       `json:"contracts_with_every_variant_first_complete"`
	ByAllowedMask               [4]int    `json:"correct_first_choice_by_required_mask"`
	RequiredMaskRows            [4]int    `json:"rows_by_required_mask"`
}

func demand(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
func sha(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func pinnedRead(path, expected string) []byte {
	raw, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	demand(sha(raw) == expected, "frozen input digest mismatch: "+filepath.Base(path))
	return raw
}
func measure(c candidate) measurement {
	demand(len(c.Outputs) == 6 && len(c.Conditions) == 3, "candidate shape")
	m := measurement{Complete: true}
	for _, item := range c.Outputs {
		demand(item.Passed == (item.Expected == item.Actual), "stored output comparison")
		if item.Passed {
			m.Outputs++
		} else {
			m.Complete = false
		}
	}
	for _, item := range c.Conditions {
		passed := item.Observation.Reached && item.Observation.Value == item.Case.Expected
		demand(item.Passed == passed, "stored Boolean comparison")
		if passed {
			m.Conditions++
		} else {
			m.Complete = false
		}
	}
	demand(c.Acceptable == m.Complete, "stored candidate acceptance")
	return m
}
func loadRows(root string) map[string]row {
	raw := pinnedRead(filepath.Join(root, "corpus.json.gz"), corpusHash)
	z, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		panic(err)
	}
	defer z.Close()
	d := json.NewDecoder(z)
	rows := map[string]row{}
	for {
		var next row
		err = d.Decode(&next)
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}
		_, found := rows[next.Specification.ID]
		demand(!found && next.Specification.ID != "", "duplicate/missing corpus ID")
		var acceptable uint64
		for mask, c := range next.Candidates {
			demand(c.Mask == uint16(mask), "candidate mask order")
			if measure(c).Complete {
				acceptable |= 1 << uint(mask)
			}
		}
		demand(acceptable == next.Acceptable && acceptable != 0, "acceptable-mask identity")
		rows[next.Specification.ID] = next
	}
	demand(len(rows) == 2048, "frozen corpus denominator")
	return rows
}

func inputKey(r row, profile string) [32]byte {
	source := r.V3
	if profile == "v2-full" {
		source = r.V2
	}
	if profile == "v3-prose-neutral" {
		for i := range source {
			clear(source[i][64:192])
		}
	}
	data := make([]byte, 0, 5376)
	appendRow := func(values []float32) {
		for _, value := range values {
			data = binary.LittleEndian.AppendUint32(data, math.Float32bits(value))
		}
	}
	for _, values := range source {
		appendRow(values[:])
	}
	for _, values := range r.Outputs {
		appendRow(values[:])
	}
	for _, values := range r.Conditions {
		appendRow(values[:])
	}
	return sha256.Sum256(data)
}
func add(g *group, r row) {
	if g.Rows == 0 {
		g.Intersection = r.Acceptable
		g.Splits = map[string]bool{}
	} else {
		g.Intersection &= r.Acceptable
	}
	g.Rows++
	g.Splits[r.Specification.Split] = true
	for mask := range 4 {
		if r.Acceptable&(1<<uint(mask)) != 0 {
			g.Allowed[mask]++
		}
	}
}
func summarize(groups map[[32]byte]*group) collisionSummary {
	s := collisionSummary{UniqueInputs: len(groups)}
	for _, g := range groups {
		s.Rows += g.Rows
		if g.Intersection == 0 {
			s.ConflictingGroups++
			s.ConflictingRows += g.Rows
		}
		if len(g.Splits) > 1 {
			s.CrossSplitGroups++
		}
		best := 0
		for _, n := range g.Allowed {
			if n > best {
				best = n
			}
		}
		s.SharedChoiceUpperBound += best
	}
	return s
}

func diagnose(root, profile string, rows map[string]row) any {
	raw := pinnedRead(filepath.Join(root, profile, "observations.json"), observationHashes[profile])
	var observations []observation
	if err := json.Unmarshal(raw, &observations); err != nil {
		panic(err)
	}
	demand(len(observations) == len(rows), "observation denominator")
	seen := map[string]bool{}
	groups := map[[32]byte]*group{}
	held := map[[32]byte]*group{}
	families := map[int]*familySummary{}
	contracts := map[string][2]int{}
	contractFamilies := map[string]int{}
	operators := [4]string{"add", "subtract", "multiply", "divide"}
	for _, observation := range observations {
		r, ok := rows[observation.ID]
		demand(ok && !seen[observation.ID] && observation.FirstMask < 4, "observation identity")
		seen[observation.ID] = true
		spec := r.Specification
		demand(observation.Split == spec.Split && observation.Family == spec.Family && observation.Contract == spec.Contract, "observation grouping")
		demand(measure(r.Candidates[observation.FirstMask]) == observation.First, "saved first result differs")
		key := inputKey(r, profile)
		if groups[key] == nil {
			groups[key] = &group{}
		}
		add(groups[key], r)
		if spec.Split != "evaluation" {
			continue
		}
		if held[key] == nil {
			held[key] = &group{}
		}
		add(held[key], r)
		if families[spec.Family] == nil {
			families[spec.Family] = &familySummary{Family: spec.Family, Operations: [2]string{operators[spec.Family/4], operators[spec.Family%4]}}
		}
		f := families[spec.Family]
		f.Rows++
		pair := contracts[spec.Contract]
		pair[0]++
		if observation.First.Complete {
			f.FirstComplete++
			pair[1]++
		}
		if observation.First.Conditions == 3 && !observation.First.Complete {
			f.AllConditionsButIncomplete++
		}
		contracts[spec.Contract] = pair
		contractFamilies[spec.Contract] = spec.Family
		for mask := range 4 {
			if r.Acceptable&(1<<uint(mask)) != 0 {
				f.RequiredMaskRows[mask]++
				if observation.First.Complete {
					f.ByAllowedMask[mask]++
				}
			}
		}
	}
	for contract, counts := range contracts {
		f := families[contractFamilies[contract]]
		f.Contracts++
		if counts[0] == counts[1] {
			f.AllVariantsCorrectContracts++
		}
	}
	ids := []int{}
	for id := range families {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	ordered := []familySummary{}
	for _, id := range ids {
		ordered = append(ordered, *families[id])
	}
	demand(len(contracts) == 16 && len(families) == 4 && summarize(held).Rows == 512, "held grouping denominator")
	return map[string]any{"observations_sha256": sha(raw), "all_inputs": summarize(groups),
		"evaluation_inputs": summarize(held), "evaluation_families": ordered}
}
func main() {
	demand(len(os.Args) == 2, "usage: audit <original-result-directory>")
	rows := loadRows(os.Args[1])
	profiles := map[string]any{}
	for _, profile := range []string{"v2-full", "v3-full", "v3-prose-neutral"} {
		profiles[profile] = diagnose(os.Args[1], profile, rows)
	}
	report := map[string]any{"schema": "gooo/frozen-input-diagnosis/v1", "corpus_sha256": corpusHash,
		"new_fits": 0, "new_model_calls": 0, "candidate_executions": 0, "native_executions": 0,
		"source_rows": 2048, "observed_rows_read": 6144, "profiles": profiles,
		"scope": "Raw float32 input identity and stored first outcomes only; no hidden-layer/pooling audit or population accuracy claim. Sibling variants share 64 contracts; evaluation has 16 contracts across 4 held operator-pair families."}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		panic(err)
	}
}
