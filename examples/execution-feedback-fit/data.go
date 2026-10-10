package main

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime/debug"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

var versions = [3]string{"v2", "v3_output_off", "v3_output_on"}
var featureVersions = [3]string{decision.ConditionBranchFeatureVersion, decision.ExecutionFeatureVersion, decision.ExecutionFeatureVersion}

type sourceRecord struct {
	ID       string            `json:"id"`
	Split    string            `json:"split"`
	Source   string            `json:"gooo_source"`
	SHA      string            `json:"source_sha256"`
	Document pathplan.Document `json:"document"`
}
type dataset struct {
	Schema   string         `json:"schema"`
	Producer string         `json:"producer_revision"`
	Compiler string         `json:"compiler_revision"`
	SDK      string         `json:"sdk_version"`
	Go       string         `json:"go_version"`
	Records  []sourceRecord `json:"records"`
}
type candidate struct {
	Mask            uint16                     `json:"mask"`
	Choices         map[string]string          `json:"choices"`
	TypeError       string                     `json:"type_error,omitempty"`
	GoooBody        string                     `json:"gooo_body"`
	GoSHA           string                     `json:"go_sha256"`
	Cases           []pathplan.TestResult      `json:"cases"`
	Conditions      []pathplan.ConditionResult `json:"conditions"`
	OutputPassed    int                        `json:"output_passed"`
	ConditionPassed int                        `json:"condition_passed"`
	Accepted        bool                       `json:"accepted"`
}
type programRecord struct {
	ID         string      `json:"id"`
	Split      string      `json:"split"`
	PlanSHA    string      `json:"plan_sha256"`
	Acceptable uint64      `json:"acceptable_candidate_bits"`
	Candidates []candidate `json:"candidates"`
}
type inputRecord struct {
	ID         string            `json:"id"`
	Split      string            `json:"split"`
	Context    string            `json:"context"`
	Features   [3][][320]float32 `json:"features_v2_off_on"`
	FeatureSHA [3]string         `json:"features_sha256_v2_off_on"`
	Masks      []uint16          `json:"candidate_masks"`
	Acceptable uint64            `json:"acceptable_candidate_bits"`
}
type preparedRecord struct {
	source     sourceRecord
	plan       *pathplan.PreparedPlan
	candidates []candidate
}

func hash(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func save(out, name string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(out, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(raw)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}
func appendRow(f *os.File, value any) error { return json.NewEncoder(f).Encode(value) }
func rowFile(out, name string) (*os.File, error) {
	return os.OpenFile(filepath.Join(out, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
}
func identity() (string, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", fmt.Errorf("producer build identity absent")
	}
	var sha string
	modified := true
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			sha = s.Value
		}
		if s.Key == "vcs.modified" {
			modified = s.Value != "false"
		}
	}
	if sha == "" || modified {
		return "", fmt.Errorf("clean committed producer required")
	}
	return sha, nil
}
func featureSHA(inputs [][320]float32, version int) string {
	width := 320
	if version == 0 {
		width = 256
	}
	digest := sha256.New()
	_, _ = digest.Write([]byte{byte(len(inputs))})
	var raw [1280]byte
	for _, input := range inputs {
		for i, value := range input[:width] {
			binary.LittleEndian.PutUint32(raw[4*i:], math.Float32bits(value))
		}
		_, _ = digest.Write(raw[:width*4])
	}
	return fmt.Sprintf("%x", digest.Sum(nil))
}

func evaluate(ctx context.Context, p *pathplan.PreparedPlan, doc pathplan.Document, mask uint16) (candidate, error) {
	c := candidate{Mask: mask, Choices: map[string]string{}}
	for i, choice := range doc.Plan.Decisions {
		c.Choices[choice.ID] = choice.Options[mask>>i&1].Label
	}
	body, err := p.Compile(c.Choices)
	if err != nil {
		c.TypeError = err.Error()
		return c, nil
	}
	c.GoooBody, c.GoSHA = body.GoooBody(), hash([]byte(body.GoSource()))
	c.Conditions, err = p.CheckDeclaredConditions(ctx, c.Choices)
	if err != nil {
		return c, err
	}
	for _, condition := range c.Conditions {
		if condition.Passed {
			c.ConditionPassed++
		}
	}
	for _, test := range doc.TestCases {
		value, err := body.Evaluate(test.Input)
		if err != nil {
			return c, err
		}
		passed := value.Int == test.Expected
		c.Cases = append(c.Cases, pathplan.TestResult{Input: test.Input, Expected: test.Expected, Actual: value.Int, Passed: passed})
		if passed {
			c.OutputPassed++
		}
	}
	c.Accepted = c.OutputPassed == len(doc.TestCases) && c.ConditionPassed == len(c.Conditions)
	return c, nil
}

func prepare(ctx context.Context, d dataset, sourceFile, contextFile *os.File) ([]preparedRecord, []inputRecord, error) {
	var programs []preparedRecord
	var rows []inputRecord
	for _, source := range d.Records {
		p, err := source.Document.Prepare()
		if err != nil {
			return nil, nil, err
		}
		count := 1 << len(source.Document.Plan.Decisions)
		proof := programRecord{ID: source.ID, Split: source.Split, PlanSHA: p.PlanSHA256()}
		masks := make([]uint16, count)
		for mask := range count {
			masks[mask] = uint16(mask)
			c, err := evaluate(ctx, p, source.Document, uint16(mask))
			if err != nil {
				return nil, nil, err
			}
			proof.Candidates = append(proof.Candidates, c)
			if c.Accepted {
				proof.Acceptable |= 1 << mask
			}
		}
		if err := appendRow(sourceFile, proof); err != nil {
			return nil, nil, err
		}
		if proof.Acceptable == 0 {
			return nil, nil, fmt.Errorf("no acceptable source candidate: %s", source.ID)
		}
		programs = append(programs, preparedRecord{source, p, proof.Candidates})
		for observation := -1; observation < count; observation++ {
			label := "initial"
			input, err := p.InitialExecutionInput(source.Document.TestCases)
			if observation >= 0 {
				label = fmt.Sprintf("observed-%d", observation)
				input, err = p.ObserveExecutionInput(ctx, proof.Candidates[observation].Choices, source.Document.TestCases)
			}
			if err != nil {
				return nil, nil, err
			}
			r := inputRecord{ID: source.ID, Split: source.Split, Context: label, Masks: masks, Acceptable: proof.Acceptable}
			for version := range versions {
				r.Features[version] = make([][320]float32, len(source.Document.Plan.Decisions))
				for i, choice := range source.Document.Plan.Decisions {
					if err := input.ExecutionFeaturesInto(choice.ID, &r.Features[version][i]); err != nil {
						return nil, nil, err
					}
					if version < 2 {
						clear(r.Features[version][i][256:])
					}
				}
				r.FeatureSHA[version] = featureSHA(r.Features[version], version)
			}
			if err := appendRow(contextFile, r); err != nil {
				return nil, nil, err
			}
			rows = append(rows, r)
		}
	}
	return programs, rows, nil
}

func validateDataset(raw []byte, producer string) (dataset, error) {
	var d dataset
	if len(raw) > 8<<20 {
		return d, fmt.Errorf("dataset exceeds 8MiB")
	}
	if err := decision.RejectDuplicateJSONKeys(raw); err != nil {
		return d, err
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return d, err
	}
	if d.Schema != "gooo/execution-feedback-source-dataset/v1" || d.Producer != producer || d.Compiler != "41f6e4668906c8ccc6798f3d43be34602ec3aa28" || d.SDK != "v0.2.32-experimental" || len(d.Records) != 128 {
		return d, fmt.Errorf("fixed source protocol identity differs")
	}
	counts := map[string]int{}
	seen := map[string]bool{}
	for _, r := range d.Records {
		if r.ID == "" || seen[r.ID] || hash([]byte(r.Source)) != r.SHA || len(r.Document.Plan.Decisions) < 1 || len(r.Document.Plan.Decisions) > 3 || len(r.Document.TestCases) == 0 {
			return d, fmt.Errorf("source identity, choices or cases differ")
		}
		seen[r.ID] = true
		counts[r.Split]++
	}
	want := map[string]int{"train": 32, "wording": 32, "constants": 32, "assignment": 16, "new_family": 16}
	if len(counts) != len(want) {
		return d, fmt.Errorf("split names differ")
	}
	for k, v := range want {
		if counts[k] != v {
			return d, fmt.Errorf("split count differs: %s", k)
		}
	}
	return d, nil
}

func trainingRows(rows []inputRecord) []inputRecord {
	var training []inputRecord
	for _, r := range rows {
		if r.Split == "train" {
			training = append(training, r)
		}
	}
	return training
}
