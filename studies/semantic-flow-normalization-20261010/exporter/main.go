package main

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
	"github.com/kimjooyoon/meta-ontology-go/internal/bodycodegen"
)

const compiler = "c6784f34e4ce9e208e53c361c757e04961233196"

type output struct {
	Input, Expected, Actual int64
	Passed                  bool
}
type candidate struct {
	Mask       uint16                     `json:"mask"`
	Outputs    []output                   `json:"outputs"`
	Conditions []pathplan.ConditionResult `json:"conditions"`
	Acceptable bool                       `json:"acceptable"`
}
type row struct {
	ID, Group, Form, Split     string
	Source, SourceSHA, PlanSHA string
	Document                   pathplan.Document
	Contexts                   []pathplan.SemanticBranchContext
	V4, V5                     [][384]float32
	V4SHA, V5SHA               string
	Acceptable                 uint64
	Candidates                 []candidate
}
type report struct {
	Schema, Producer, Compiler                                                                           string
	Sources, PairedComparisons, EqualV4, EqualV5, ChangedAcceptable, NormalizedSources, RetainedControls int
	ConflictPairsV4, ConflictPairsV5                                                                     int
	CandidateRecords, ExplicitCompileCalls, OutputEvaluations, ConditionObservations                     int
	Fits, ModelCalls, NativeExecutions                                                                   int
	Rows                                                                                                 []row
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func require(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
func hash(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func encode(x any) []byte    { b, e := json.Marshal(x); must(e); return b }
func save(root, name string, b []byte) {
	f, e := os.OpenFile(filepath.Join(root, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	must(e)
	_, e = f.Write(b)
	must(e)
	must(f.Close())
}
func featureSHA(features [][384]float32) string {
	h := sha256.New()
	h.Write([]byte{byte(len(features))})
	var b [1536]byte
	for _, f := range features {
		for i, x := range f {
			binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(x))
		}
		h.Write(b[:])
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
func producer() string {
	b, ok := debug.ReadBuildInfo()
	require(ok && b.GoVersion == "go1.27.2", "exact toolchain")
	var revision string
	clean := false
	for _, s := range b.Settings {
		if s.Key == "vcs.revision" {
			revision = s.Value
		}
		if s.Key == "vcs.modified" {
			clean = s.Value == "false"
		}
	}
	require(clean && len(revision) == 40, "clean committed SDK producer")
	return revision
}
func observe(ctx context.Context, s specification) row {
	text := source(s)
	doc, err := bodycodegen.DecodeSourcePathDocument(ctx, s.ID+".gooo", []byte(text), "Choose", nil)
	must(err)
	p, err := doc.Prepare()
	must(err)
	r := row{ID: s.ID, Group: s.Group, Form: s.Form, Split: s.Split, Source: text, SourceSHA: hash([]byte(text)), PlanSHA: p.PlanSHA256(), Document: doc}
	input, err := p.InitialExecutionInput(doc.TestCases)
	must(err)
	for _, choice := range doc.Plan.Decisions {
		view, err := p.SemanticBranchContext(choice.ID)
		must(err)
		r.Contexts = append(r.Contexts, view)
		var a, b [384]float32
		must(input.ExecutionFlowFeaturesInto(choice.ID, &a))
		must(input.ExecutionSemanticFlowFeaturesInto(choice.ID, &b))
		r.V4 = append(r.V4, a)
		r.V5 = append(r.V5, b)
	}
	r.V4SHA, r.V5SHA = featureSHA(r.V4), featureSHA(r.V5)
	for mask := range 4 {
		choices := map[string]string{}
		for i, choice := range doc.Plan.Decisions {
			choices[choice.ID] = choice.Options[mask>>i&1].Label
		}
		program, err := p.Compile(choices)
		must(err)
		c := candidate{Mask: uint16(mask), Acceptable: true}
		for _, test := range doc.TestCases {
			value, err := program.Evaluate(test.Input)
			must(err)
			passed := value.Int == test.Expected
			c.Outputs = append(c.Outputs, output{test.Input, test.Expected, value.Int, passed})
			c.Acceptable = c.Acceptable && passed
		}
		c.Conditions, err = p.CheckConditions(ctx, choices, doc.Plan.ConditionCases)
		must(err)
		for _, condition := range c.Conditions {
			c.Acceptable = c.Acceptable && condition.Passed
		}
		if c.Acceptable {
			r.Acceptable |= 1 << mask
		}
		r.Candidates = append(r.Candidates, c)
	}
	require(r.Acceptable != 0, "source has no acceptable candidate: "+s.ID)
	return r
}
func summarize(r *report) {
	direct := map[string]row{}
	for _, x := range r.Rows {
		r.Sources++
		r.CandidateRecords += len(x.Candidates)
		for _, c := range x.Candidates {
			r.ExplicitCompileCalls += 2
			r.OutputEvaluations += len(c.Outputs)
			r.ConditionObservations += len(c.Conditions)
		}
		normalized := true
		for _, c := range x.Contexts {
			normalized = normalized && c.Normalized
		}
		if normalized {
			r.NormalizedSources++
		} else if x.Split == "control" && x.V4SHA == x.V5SHA {
			r.RetainedControls++
		}
		if x.Form == "direct" {
			direct[x.Group] = x
		}
	}
	for i, x := range r.Rows {
		if x.Form != "direct" && x.Split != "control" {
			d, ok := direct[x.Group]
			require(ok, "missing direct form")
			r.PairedComparisons++
			if x.V4SHA == d.V4SHA {
				r.EqualV4++
			}
			if x.V5SHA == d.V5SHA {
				r.EqualV5++
			}
			if x.Acceptable != d.Acceptable {
				r.ChangedAcceptable++
			}
		}
		for _, y := range r.Rows[:i] {
			if x.Acceptable&y.Acceptable == 0 {
				if x.V4SHA == y.V4SHA {
					r.ConflictPairsV4++
				}
				if x.V5SHA == y.V5SHA {
					r.ConflictPairsV5++
				}
			}
		}
	}
}
func main() {
	require(len(os.Args) == 3, "usage: semantic-flow-export COMPILER_CHECKOUT FRESH_OUTPUT")
	revision := producer()
	head, err := exec.Command("git", "-C", os.Args[1], "rev-parse", "HEAD").Output()
	must(err)
	require(strings.TrimSpace(string(head)) == compiler, "compiler revision")
	status, err := exec.Command("git", "-C", os.Args[1], "status", "--porcelain").Output()
	must(err)
	require(len(status) == 0, "clean compiler source")
	out := os.Args[2]
	must(os.Mkdir(out, 0700))
	save(out, "started.json", encode(map[string]string{"producer": revision, "compiler": compiler, "time": time.Now().UTC().Format(time.RFC3339)}))
	r := report{Schema: "gooo/semantic-flow-normalization/v1", Producer: revision, Compiler: compiler}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	f, err := os.OpenFile(filepath.Join(out, "records.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	must(err)
	for _, s := range specifications() {
		record := observe(ctx, s)
		must(json.NewEncoder(f).Encode(record))
		r.Rows = append(r.Rows, record)
	}
	must(f.Close())
	summarize(&r)
	require(r.Sources == 162 && r.PairedComparisons == 128 && r.CandidateRecords == 648, "complete fixed scope")
	r.Rows = nil
	save(out, "report.json", encode(r))
	save(out, "completed.json", encode(map[string]any{"time": time.Now().UTC().Format(time.RFC3339), "sources": r.Sources, "candidate_records": r.CandidateRecords, "model_calls": 0, "fits": 0}))
	fmt.Printf("%d sources, %d candidate records, %d matched pairs; %d -> %d equal inputs; no model calls.\n", r.Sources, r.CandidateRecords, r.PairedComparisons, r.EqualV4, r.EqualV5)
}
