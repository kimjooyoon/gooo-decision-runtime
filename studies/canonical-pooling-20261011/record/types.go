package record

import (
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
	previous "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-contract-learning-20261011/record"
)

type Rows = previous.Rows
type Observations = previous.Observations
type Candidate = previous.Candidate
type Stage = previous.Stage
type Search = previous.Search
type Model = previous.Model

var Hash = previous.Hash
var Encode = previous.Encode
var Passed = previous.Passed

type Source struct {
	Frozen
	SourceSHA, PlanSHA, CasesSHA string
	Document                     pathplan.Document
	CanonicalInputs              [][528]float32
	OutputRows, ConditionRows    [][32]float32
	Acceptable, ProbeAcceptable  uint64
	Candidates                   []Candidate
}

type TrainingRecord struct {
	ID, PlanSHA, CasesSHA string
	CanonicalInputs       [][528]float32
	Outputs, Conditions   [][32]float32
	Acceptable            uint64
}

type Group struct {
	Programs, FirstDeclaredComplete, FirstProbeComplete, InitialDeclaredComplete, InitialProbeComplete int
	InitialAttempts, TotalAttempts, ModelCalls                                                         int
	SearchNS, PredictNS                                                                                int64
}

type Report struct {
	Schema, Producer, Compiler, SDKBase, CorpusSHA, SourcesSHA, SearchesSHA string
	Sources, TrainingRows, Fits, InitialSearches, ModelCalls                int
	Options                                                                 contractdecision.FitOptions
	Models                                                                  map[string]Model
	Groups                                                                  map[string]Group
	InputAudit                                                              *contractdecision.InputAudit
}
