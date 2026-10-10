package main

import (
	"encoding/json"
	"path/filepath"
	"slices"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/flowdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

type sourceRecord struct {
	ID       string            `json:"id"`
	Split    string            `json:"split"`
	Source   string            `json:"gooo_source"`
	SHA      string            `json:"source_sha256"`
	Document pathplan.Document `json:"document"`
}
type frozenContext struct {
	ID         string            `json:"id"`
	Split      string            `json:"split"`
	Context    string            `json:"context"`
	Features   [3][][320]float32 `json:"features_v2_off_on"`
	FeatureSHA [3]string         `json:"features_sha256_v2_off_on"`
	Masks      []uint16          `json:"candidate_masks"`
	Acceptable uint64            `json:"acceptable_candidate_bits"`
}
type projectionRow struct {
	ID         string                     `json:"id"`
	Split      string                     `json:"split"`
	SourceSHA  string                     `json:"source_sha256"`
	PlanSHA    string                     `json:"plan_sha256"`
	Decline    string                     `json:"decline"`
	Acceptable uint64                     `json:"acceptable_candidate_bits"`
	AfterSHA   string                     `json:"after_sha256"`
	Facts      []decision.BranchValueFlow `json:"facts"`
	Features   [][384]float32             `json:"features"`
}
type inputRecord struct {
	ID         string         `json:"id"`
	Split      string         `json:"split"`
	Context    string         `json:"context"`
	Features   [][384]float32 `json:"features"`
	SHA        string         `json:"features_sha256"`
	Masks      []uint16       `json:"candidate_masks"`
	Acceptable uint64         `json:"acceptable_candidate_bits"`
}
type preparedSource struct {
	source     sourceRecord
	plan       *pathplan.PreparedPlan
	projection projectionRow
}

func load(original, projected string) ([]preparedSource, []inputRecord, map[string]string) {
	digests := map[string]string{}
	raw, sha := gzipBytes(filepath.Join(original, "dataset.json.gz"))
	digests["dataset.json.gz"] = sha
	var dataset struct {
		Schema  string         `json:"schema"`
		Records []sourceRecord `json:"records"`
	}
	must(json.Unmarshal(raw, &dataset))
	require(dataset.Schema == "gooo/execution-feedback-source-dataset/v1" && len(dataset.Records) == 128, "original 128 sources")
	raw, sha = gzipBytes(filepath.Join(projected, "report.json.gz"))
	digests["projection-report.json.gz"] = sha
	var projection struct {
		Schema     string          `json:"schema"`
		DatasetSHA string          `json:"dataset_gzip_sha256"`
		ContextSHA string          `json:"contexts_gzip_sha256"`
		Rows       []projectionRow `json:"rows"`
	}
	must(json.Unmarshal(raw, &projection))
	require(projection.Schema == "gooo/static-value-flow-projection/v1" && len(projection.Rows) == 128 && projection.DatasetSHA == digests["dataset.json.gz"], "frozen static projection")
	projections := map[string]projectionRow{}
	for _, p := range projection.Rows {
		_, exists := projections[p.ID]
		require(!exists && p.Decline == "", "unique nondeclined projection")
		projections[p.ID] = p
	}
	var sources []preparedSource
	byID := map[string]preparedSource{}
	for _, source := range dataset.Records {
		p, ok := projections[source.ID]
		require(ok && hash([]byte(source.Source)) == source.SHA && p.SourceSHA == source.SHA && p.Split == source.Split, "source provenance")
		plan, err := source.Document.Prepare()
		must(err)
		require(plan.PlanSHA256() == p.PlanSHA && len(p.Features) == len(source.Document.Plan.Decisions) && len(p.Facts) == len(p.Features), "plan and choice identity")
		require(featureSHA(p.Features, 384) == p.AfterSHA, "static initial input digest")
		_, exists := byID[source.ID]
		require(!exists, "unique source")
		s := preparedSource{source, plan, p}
		byID[source.ID] = s
		sources = append(sources, s)
	}
	raw, sha = gzipBytes(filepath.Join(original, "contexts.jsonl.gz"))
	digests["contexts.jsonl.gz"] = sha
	require(sha == projection.ContextSHA, "frozen context identity")
	frozen := decodeLines[frozenContext](raw)
	require(len(frozen) == 704, "704 original contexts")
	seen := map[string]bool{}
	counts := map[string]int{}
	var inputs []inputRecord
	for _, f := range frozen {
		s, ok := byID[f.ID]
		require(ok && f.Split == s.source.Split && f.Acceptable == s.projection.Acceptable, "frozen source labels")
		require(len(f.Masks) == 1<<len(s.projection.Facts) && len(f.Features[2]) == len(s.projection.Facts), "complete choice pool")
		for i, mask := range f.Masks {
			require(int(mask) == i, "ascending complete masks")
		}
		require(f.Acceptable != 0 && f.Acceptable>>len(f.Masks) == 0, "bounded acceptable set")
		key := f.ID + "/" + f.Context
		require(!seen[key], "unique context")
		seen[key] = true
		counts[f.ID]++
		r := inputRecord{ID: f.ID, Split: f.Split, Context: f.Context, Masks: f.Masks, Acceptable: f.Acceptable, Features: make([][384]float32, len(f.Features[2]))}
		for i, prefix := range f.Features[2] {
			must(decision.ExecutionFlowFeaturesInto(prefix, s.projection.Facts[i], &r.Features[i]))
			if f.Context == "initial" {
				require(r.Features[i] == s.projection.Features[i], "unchanged initial projection")
			}
		}
		require(featureSHA(r.Features, 320) == f.FeatureSHA[2], "unchanged v3 context prefix")
		r.SHA = featureSHA(r.Features, 384)
		inputs = append(inputs, r)
	}
	for _, s := range sources {
		require(seen[s.source.ID+"/initial"] && counts[s.source.ID] == 1+(1<<len(s.projection.Facts)), "complete saved context set")
	}
	return sources, inputs, digests
}

func trainingSamples(rows []inputRecord, enabled bool) []flowdecision.Sample {
	var result []flowdecision.Sample
	for _, r := range rows {
		if r.Split != "train" {
			continue
		}
		features := slices.Clone(r.Features)
		if !enabled {
			for i := range features {
				clear(features[i][320:])
			}
		}
		result = append(result, flowdecision.Sample{Inputs: features, Masks: slices.Clone(r.Masks), Acceptable: r.Acceptable})
	}
	return result
}
