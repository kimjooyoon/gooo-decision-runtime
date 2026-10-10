// Command audit recounts the saved projection without preparing a plan,
// evaluating a candidate, fitting weights or invoking a model.
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
	"slices"

	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

type value struct {
	Present bool   `json:"present"`
	Kind    string `json:"kind"`
	Int     int64  `json:"int"`
}
type facts struct {
	Returns   [2]value `json:"returns"`
	Predicate [2]value `json:"predicate_operands"`
}
type row struct {
	ID         string         `json:"id"`
	Split      string         `json:"split"`
	SourceSHA  string         `json:"source_sha256"`
	PlanSHA    string         `json:"plan_sha256"`
	Decline    string         `json:"decline"`
	Acceptable uint64         `json:"acceptable_candidate_bits"`
	BeforeSHA  string         `json:"before_sha256"`
	AfterSHA   string         `json:"after_sha256"`
	Facts      []facts        `json:"facts"`
	Features   [][384]float32 `json:"features"`
}
type report struct {
	Schema             string `json:"schema"`
	Producer           string `json:"producer_revision"`
	DatasetSHA         string `json:"dataset_gzip_sha256"`
	ContextSHA         string `json:"contexts_gzip_sha256"`
	FeatureVersion     string `json:"feature_version"`
	PreparedPlans      int    `json:"prepared_plans"`
	ModelCalls         int    `json:"model_calls"`
	FittedModels       int    `json:"fitted_models"`
	CandidateTests     int    `json:"candidate_tests"`
	BeforeCollisions   int    `json:"before_collisions"`
	AfterCollisions    int    `json:"after_collisions"`
	Declined           int    `json:"declined"`
	DeclinedPriorPairs int    `json:"declined_prior_collision_pairs"`
	Rows               []row  `json:"rows"`
}
type context struct {
	ID         string            `json:"id"`
	Context    string            `json:"context"`
	Features   [3][][320]float32 `json:"features_v2_off_on"`
	FeatureSHA [3]string         `json:"features_sha256_v2_off_on"`
	Masks      []uint16          `json:"candidate_masks"`
	Acceptable uint64            `json:"acceptable_candidate_bits"`
}

func require(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
func hash(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func gzipBytes(path string) ([]byte, string) {
	raw, err := os.ReadFile(path)
	require(err == nil, "read "+path)
	f, err := os.Open(path)
	require(err == nil, "open "+path)
	defer f.Close()
	z, err := gzip.NewReader(f)
	require(err == nil, "gzip "+path)
	defer z.Close()
	data, err := io.ReadAll(z)
	require(err == nil, "decompress "+path)
	return data, hash(raw)
}

func digest(values [][384]float32, width int) string {
	raw := []byte{byte(len(values))}
	for _, v := range values {
		for _, x := range v[:width] {
			raw = binary.LittleEndian.AppendUint32(raw, math.Float32bits(x))
		}
	}
	return hash(raw)
}

// Independently decode the appended representation, rather than calling the
// production encoder and trusting its output again.
func decode(fields []float32) value {
	require(len(fields) == 16, "value slot length")
	for _, flag := range fields[:8] {
		require(flag == 0 || flag == .125, "Boolean static flag")
	}
	var bits uint64
	for _, b := range fields[8:] {
		x := float64(b) * 2048
		require(x >= 0 && x <= 255 && math.Trunc(x) == x, "exact integer byte")
		bits = bits<<8 | uint64(x)
	}
	v := value{Present: fields[0] != 0, Int: int64(bits)}
	if !v.Present {
		require(slices.Equal(fields, make([]float32, 16)), "absent slot zero")
		return value{}
	}
	kinds := [...]string{"input", "int", "bool", "unknown"}
	for i, kind := range kinds {
		if fields[i+1] != 0 {
			require(v.Kind == "", "one kind")
			v.Kind = kind
		}
	}
	require(v.Kind != "", "kind required")
	if v.Kind == "input" || v.Kind == "unknown" {
		require(v.Int == 0, "symbolic value has no literal")
	}
	if v.Kind == "bool" {
		require(v.Int == 0 || v.Int == 1, "Boolean literal")
	}
	signs := [3]bool{v.Kind == "int" && v.Int < 0, v.Kind == "int" && v.Int == 0, v.Kind == "int" && v.Int > 0}
	for i, want := range signs {
		require((fields[5+i] != 0) == want, "literal sign")
	}
	return v
}

func auditRows(r report, sources map[string][3]string, contexts map[string]context) {
	seen := map[string]bool{}
	for _, row := range r.Rows {
		require(!seen[row.ID], "unique source row")
		seen[row.ID] = true
		s, ok := sources[row.ID]
		require(ok && s == [3]string{row.Split, row.SourceSHA, row.PlanSHA}, "source and serialized plan identity")
		c, ok := contexts[row.ID]
		require(ok && row.Acceptable == c.Acceptable && row.Acceptable != 0, "original labels")
		require(row.Decline == "", "prepared accepted row")
		require(len(row.Features) == len(row.Facts) && len(row.Features) == len(c.Features[2]), "choice arrays")
		require(len(c.Masks) == 1<<len(row.Features), "complete mask pool")
		for i, mask := range c.Masks {
			require(int(mask) == i, "ascending masks")
		}
		require(c.Acceptable>>len(c.Masks) == 0, "bounded original labels")
		for i, features := range row.Features {
			require(slices.Equal(features[:320], c.Features[2][i][:]), "original v3 prefix")
			for _, x := range features {
				require(!math.IsNaN(float64(x)) && !math.IsInf(float64(x), 0), "finite feature")
			}
			f := row.Facts[i]
			for j, want := range [4]value{f.Returns[0], f.Returns[1], f.Predicate[0], f.Predicate[1]} {
				require(decode(features[320+j*16:336+j*16]) == want, "saved facts match exact fields")
			}
		}
		require(row.BeforeSHA == digest(row.Features, 320) && row.BeforeSHA == c.FeatureSHA[2], "original digest")
		require(row.AfterSHA == digest(row.Features, 384), "new digest")
	}
}

func collisions(rows []row) (int, int, map[string]int) {
	before, after := 0, 0
	groups := map[string]int{}
	for i, a := range rows {
		for _, b := range rows[i+1:] {
			if a.Acceptable&b.Acceptable != 0 {
				continue
			}
			if a.BeforeSHA == b.BeforeSHA {
				before++
				pair := []string{a.Split, b.Split}
				slices.Sort(pair)
				groups[pair[0]+"/"+pair[1]]++
			}
			if a.AfterSHA == b.AfterSHA {
				after++
			}
		}
	}
	return before, after, groups
}

func main() {
	require(len(os.Args) == 3, "usage: audit RESULT_DIRECTORY ORIGINAL_RESULT_DIRECTORY")
	result, original := os.Args[1], os.Args[2]
	raw, _ := gzipBytes(filepath.Join(result, "report.json.gz"))
	var r report
	require(json.Unmarshal(raw, &r) == nil, "report JSON")
	require(r.Schema == "gooo/static-value-flow-projection/v1" && r.Producer == "ac4519b8b2dd50d7e155aa8485783e77b13412de", "clean producer identity")
	require(r.FeatureVersion == "source_intent_condition_output_value_flow_v4", "explicit ABI")
	require(r.PreparedPlans == 128 && len(r.Rows) == 128 && r.ModelCalls == 0 && r.FittedModels == 0 && r.CandidateTests == 0, "declared work")
	require(r.Declined == 0 && r.DeclinedPriorPairs == 0, "no excluded rows")
	data, sha := gzipBytes(filepath.Join(original, "dataset.json.gz"))
	require(sha == r.DatasetSHA, "original dataset digest")
	var dataset struct {
		Records []struct {
			ID       string            `json:"id"`
			Split    string            `json:"split"`
			Source   string            `json:"gooo_source"`
			SHA      string            `json:"source_sha256"`
			Document pathplan.Document `json:"document"`
		} `json:"records"`
	}
	require(json.Unmarshal(data, &dataset) == nil && len(dataset.Records) == 128, "128 frozen sources")
	sources := map[string][3]string{}
	for _, s := range dataset.Records {
		_, exists := sources[s.ID]
		require(!exists && hash([]byte(s.Source)) == s.SHA, "unique source SHA")
		plan, err := json.Marshal(s.Document.Plan)
		require(err == nil, "original typed plan serialization")
		sources[s.ID] = [3]string{s.Split, s.SHA, hash(plan)}
	}
	data, sha = gzipBytes(filepath.Join(original, "contexts.jsonl.gz"))
	require(sha == r.ContextSHA, "original context digest")
	contexts := map[string]context{}
	for line := range bytes.SplitSeq(data, []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		var c context
		require(json.Unmarshal(line, &c) == nil, "context JSON")
		if c.Context == "initial" {
			_, exists := contexts[c.ID]
			require(!exists, "unique original context")
			contexts[c.ID] = c
		}
	}
	require(len(contexts) == 128, "128 initial contexts")
	auditRows(r, sources, contexts)
	before, after, groups := collisions(r.Rows)
	require(before == r.BeforeCollisions && after == r.AfterCollisions, "recount matches report")
	output, err := json.MarshalIndent(map[string]any{"rows": len(r.Rows), "before_collisions": before, "after_collisions": after, "prior_pairs_by_split": groups, "new_model_calls": 0, "new_candidate_tests": 0, "new_plan_preparations": 0}, "", "  ")
	require(err == nil, "audit output")
	fmt.Println(string(output))
}
