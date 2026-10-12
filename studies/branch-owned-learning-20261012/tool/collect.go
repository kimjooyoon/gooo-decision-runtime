package main

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
	"github.com/kimjooyoon/meta-ontology-go/internal/bodycodegen"
)

func collect(root string) error {
	if _, err := os.Stat(filepath.Join(root, "code-verified.txt")); err != nil {
		return err
	}
	if err := begin(root, "collect"); err != nil {
		return err
	}
	var specs []specification
	raw, err := os.ReadFile(filepath.Join(root, "prefit", "specifications.json"))
	if err != nil {
		return err
	}
	if err = json.Unmarshal(raw, &specs); err != nil || len(specs) != 1536 {
		return fmt.Errorf("specification coverage: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	file, err := os.OpenFile(filepath.Join(root, "corpus.json.gz"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	z := gzip.NewWriter(file)
	encoder := json.NewEncoder(z)
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return fmt.Errorf("producer build identity missing")
	}
	versions := map[string]string{}
	for _, dependency := range info.Deps {
		if dependency.Replace != nil {
			return fmt.Errorf("producer dependency replacement")
		}
		versions[dependency.Path] = dependency.Version
	}
	if info.GoVersion != "go1.27.2" || versions["github.com/kimjooyoon/gooo-decision-runtime"] != "v0.2.46-experimental" || versions["github.com/kimjooyoon/meta-ontology-go"] != "v0.6.34-dev" {
		return fmt.Errorf("producer versions differ")
	}
	if err = writeJSON(filepath.Join(root, "producer-build.json"), map[string]any{"go": info.GoVersion, "module": info.Main, "dependencies": versions, "scope": "public compiler library plus public SDK, no installed-public-binary/native execution claim"}); err != nil {
		return err
	}
	for _, spec := range specs {
		if err := ctx.Err(); err != nil {
			return err
		}
		source, err := os.ReadFile(filepath.Join(root, "prefit", filepath.FromSlash(spec.Source)))
		if err != nil {
			return err
		}
		if fmt.Sprintf("%x", sha256.Sum256(source)) != spec.SourceSHA {
			return fmt.Errorf("source drift: %s", spec.ID)
		}
		doc, err := bodycodegen.DecodeSourcePathDocument(ctx, spec.ID+".gooo", source, "Choose", nil)
		if err != nil {
			return fmt.Errorf("%s: %w", spec.ID, err)
		}
		if len(doc.Plan.Decisions) != 2 || len(doc.TestCases) != 8 || len(doc.Plan.ConditionCases) != 3 {
			return fmt.Errorf("source contract shape: %s", spec.ID)
		}
		prepared, err := doc.Prepare()
		if err != nil {
			return err
		}
		input, err := prepared.InitialContractInput(doc.TestCases)
		if err != nil {
			return err
		}
		row := record{Specification: spec, Document: doc, PlanSHA: input.PlanSHA256(), CaseSHA: input.CaseSHA256()}
		for i, choice := range doc.Plan.Decisions {
			if err = input.SourceRelativeV4FeaturesInto(choice.ID, &row.V4[i]); err != nil {
				return err
			}
			if err = input.SourceRelativeV3ProseNeutralFeaturesInto(choice.ID, &row.V3[i]); err != nil {
				return err
			}
			if !slices.Equal(row.V4[i][:256], row.V3[i][:256]) || !slices.Equal(row.V4[i][352:], row.V3[i][352:]) {
				return fmt.Errorf("unexpected V4/neutral difference outside branch cells")
			}
			row.BranchOutputs[i], err = input.BranchOutputs(choice.ID)
			if err != nil || !row.BranchOutputs[i].Available {
				return fmt.Errorf("branch context: %s %v", row.BranchOutputs[i].Reason, err)
			}
			row.OutputRelations[i], err = input.SourceRelativeOutputs(choice.ID)
			if err != nil || !row.OutputRelations[i].Available {
				return fmt.Errorf("output context: %w", err)
			}
		}
		for i := range 8 {
			row.Outputs[i], err = input.CaseFeatures(i)
			if err != nil {
				return err
			}
		}
		for i := range 3 {
			row.Conditions[i], err = input.ConditionFeatures(i)
			if err != nil {
				return err
			}
		}
		target := -1
		for _, choice := range doc.Plan.Decisions {
			if choice.Kind == pathplan.BranchLayout {
				target = choice.Target
			}
		}
		if target < 0 {
			return fmt.Errorf("source if target missing")
		}
		for mask := range 4 {
			choices := map[string]string{}
			for i, choice := range doc.Plan.Decisions {
				choices[choice.ID] = choice.Options[(mask>>i)&1].Label
			}
			program, err := prepared.Compile(choices)
			if err != nil {
				return err
			}
			c := candidate{Mask: uint16(mask), Acceptable: true}
			for _, test := range doc.TestCases {
				value, err := program.Evaluate(test.Input)
				if err != nil {
					return err
				}
				passed := value.Int == test.Expected
				c.Outputs = append(c.Outputs, pathplan.TestResult{Input: test.Input, Expected: test.Expected, Actual: value.Int, Passed: passed})
				c.Acceptable = c.Acceptable && passed
			}
			for _, test := range doc.Plan.ConditionCases {
				value, observation, err := program.ObserveCondition(test.Input, target)
				if err != nil {
					return err
				}
				passed := observation.Reached && observation.Value == test.Expected
				status := "MISMATCH"
				if !observation.Reached {
					status = "NOT_REACHED"
				} else if passed {
					status = "MATCH"
				}
				c.Conditions = append(c.Conditions, pathplan.ConditionResult{Case: test, Observation: observation, Output: value, Status: status, Passed: passed})
				c.Acceptable = c.Acceptable && passed
			}
			for _, test := range spec.Probes {
				value, err := program.Evaluate(test.Input)
				if err != nil {
					return err
				}
				c.Probes = append(c.Probes, pathplan.TestResult{Input: test.Input, Expected: test.Expected, Actual: value.Int, Passed: value.Int == test.Expected})
			}
			if c.Acceptable {
				row.Acceptable |= 1 << mask
			}
			row.Candidates[mask] = c
		}
		if row.Acceptable == 0 {
			return fmt.Errorf("no declared acceptable mask: %s", spec.ID)
		}
		if err = encoder.Encode(row); err != nil {
			return err
		}
	}
	if err = z.Close(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return writeJSON(filepath.Join(root, "collection-complete.json"), map[string]any{"sources": 1536, "candidate_histories": 6144, "model_calls": 0, "fits": 0, "native_executions": 0})
}
