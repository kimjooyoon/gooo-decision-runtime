package main

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"time"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

type source struct {
	ID       string            `json:"id"`
	Split    string            `json:"split"`
	Gooo     string            `json:"gooo_source"`
	SHA      string            `json:"source_sha256"`
	Document pathplan.Document `json:"document"`
}
type savedContext struct {
	ID         string            `json:"id"`
	Context    string            `json:"context"`
	Features   [3][][320]float32 `json:"features_v2_off_on"`
	FeatureSHA [3]string         `json:"features_sha256_v2_off_on"`
	Masks      []uint16          `json:"candidate_masks"`
	Acceptable uint64            `json:"acceptable_candidate_bits"`
}
type row struct {
	ID         string                     `json:"id"`
	Split      string                     `json:"split"`
	SourceSHA  string                     `json:"source_sha256"`
	PlanSHA    string                     `json:"plan_sha256"`
	Decline    string                     `json:"decline,omitempty"`
	Acceptable uint64                     `json:"acceptable_candidate_bits"`
	BeforeSHA  string                     `json:"before_sha256"`
	AfterSHA   string                     `json:"after_sha256,omitempty"`
	Facts      []decision.BranchValueFlow `json:"facts"`
	Features   [][384]float32             `json:"features"`
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

func must(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
func hash(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }

func gzipDecoder(name string) (*json.Decoder, func()) {
	f, err := os.Open(name)
	must(err == nil, "open "+name)
	z, err := gzip.NewReader(f)
	must(err == nil, "gzip "+name)
	d := json.NewDecoder(z)
	d.UseNumber()
	return d, func() { z.Close(); f.Close() }
}

func initialContexts(name string) map[string]savedContext {
	d, close := gzipDecoder(name)
	defer close()
	result := map[string]savedContext{}
	for {
		var c savedContext
		err := d.Decode(&c)
		if err == io.EOF {
			break
		}
		must(err == nil, "context JSON")
		if c.Context == "initial" {
			_, exists := result[c.ID]
			must(!exists, "duplicate initial context")
			result[c.ID] = c
		}
	}
	return result
}

func featuresSHA(values [][384]float32, width int) string {
	h := sha256.New()
	_, _ = h.Write([]byte{byte(len(values))})
	var raw [1536]byte
	for _, input := range values {
		for j, value := range input[:width] {
			binary.LittleEndian.PutUint32(raw[j*4:], math.Float32bits(value))
		}
		_, _ = h.Write(raw[:width*4])
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func project(s source, saved savedContext) row {
	must(hash([]byte(s.Gooo)) == s.SHA && saved.ID == s.ID, "source identity")
	p, err := s.Document.Prepare()
	must(err == nil, "source preparation")
	initial, err := p.InitialExecutionInput(s.Document.TestCases)
	must(err == nil, "source input")
	out := row{ID: s.ID, Split: s.Split, SourceSHA: s.SHA, PlanSHA: p.PlanSHA256(), Acceptable: saved.Acceptable,
		Facts: make([]decision.BranchValueFlow, 0, len(s.Document.Plan.Decisions)), Features: make([][384]float32, 0, len(s.Document.Plan.Decisions))}
	must(len(saved.Features[2]) == len(s.Document.Plan.Decisions), "choice count")
	for i, mask := range saved.Masks {
		must(int(mask) == i, "complete ascending saved mask pool")
	}
	must(saved.Acceptable != 0 && saved.Acceptable>>len(saved.Masks) == 0, "bounded acceptable pool")
	for i, choice := range s.Document.Plan.Decisions {
		var before [320]float32
		must(initial.ExecutionFeaturesInto(choice.ID, &before) == nil && before == saved.Features[2][i], "unchanged v3 input")
		flow, err := p.BranchValueFlow(choice.ID)
		if err != nil {
			out.Decline = err.Error()
			out.BeforeSHA = saved.FeatureSHA[2]
			return out
		}
		var after [384]float32
		must(decision.ExecutionFlowFeaturesInto(before, flow, &after) == nil, "flow encoding")
		must(slices.Equal(before[:], after[:320]), "preserved prefix")
		out.Facts = append(out.Facts, flow)
		out.Features = append(out.Features, after)
	}
	out.BeforeSHA, out.AfterSHA = featuresSHA(out.Features, 320), featuresSHA(out.Features, 384)
	must(out.BeforeSHA == saved.FeatureSHA[2], "saved vector digest")
	return out
}

func countCollisions(rows []row) (before, after, declined int) {
	for i, a := range rows {
		for _, b := range rows[i+1:] {
			if a.Acceptable&b.Acceptable != 0 {
				continue
			}
			if a.BeforeSHA == b.BeforeSHA {
				before++
				if a.Decline != "" || b.Decline != "" {
					declined++
				}
			}
			if a.Decline == "" && b.Decline == "" && a.AfterSHA == b.AfterSHA {
				after++
			}
		}
	}
	return
}

func producer() string {
	info, ok := debug.ReadBuildInfo()
	must(ok && info.GoVersion == "go1.27.2", "Go1.27.2 producer")
	var revision string
	modified := true
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			revision = s.Value
		}
		if s.Key == "vcs.modified" {
			modified = s.Value != "false"
		}
	}
	must(revision != "" && !modified, "clean committed producer")
	return revision
}

func main() {
	must(len(os.Args) == 3, "usage: flow-projection ORIGINAL_RESULT_DIRECTORY NEW_OUTPUT_DIRECTORY")
	revision := producer()
	in, out := os.Args[1], os.Args[2]
	must(os.Mkdir(out, 0700) == nil, "new output directory required")
	must(os.WriteFile(filepath.Join(out, "started.txt"), []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0600) == nil, "start marker")
	var dataset struct {
		Schema  string   `json:"schema"`
		Records []source `json:"records"`
	}
	d, close := gzipDecoder(filepath.Join(in, "dataset.json.gz"))
	must(d.Decode(&dataset) == nil && dataset.Schema == "gooo/execution-feedback-source-dataset/v1" && len(dataset.Records) == 128, "saved dataset")
	close()
	contexts := initialContexts(filepath.Join(in, "contexts.jsonl.gz"))
	must(len(contexts) == 128, "128 original initial contexts")
	r := report{Schema: "gooo/static-value-flow-projection/v1", Producer: revision, FeatureVersion: decision.ExecutionFlowFeatureVersion, Rows: make([]row, 0, 128)}
	datasetRaw, err := os.ReadFile(filepath.Join(in, "dataset.json.gz"))
	must(err == nil, "dataset bytes")
	contextRaw, err := os.ReadFile(filepath.Join(in, "contexts.jsonl.gz"))
	must(err == nil, "context bytes")
	r.DatasetSHA, r.ContextSHA = hash(datasetRaw), hash(contextRaw)
	for _, s := range dataset.Records {
		row := project(s, contexts[s.ID])
		r.PreparedPlans++
		if row.Decline != "" {
			r.Declined++
		}
		r.Rows = append(r.Rows, row)
	}
	r.BeforeCollisions, r.AfterCollisions, r.DeclinedPriorPairs = countCollisions(r.Rows)
	raw, err := json.Marshal(r)
	must(err == nil && os.WriteFile(filepath.Join(out, "report.json"), raw, 0600) == nil, "report")
	must(os.WriteFile(filepath.Join(out, "completed.txt"), []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0600) == nil, "completion marker")
	fmt.Printf("plans=%d declined=%d collisions=%d->%d declined_pairs=%d model_calls=0 candidate_tests=0\n", r.PreparedPlans, r.Declined, r.BeforeCollisions, r.AfterCollisions, r.DeclinedPriorPairs)
}
