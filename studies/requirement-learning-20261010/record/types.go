package record

import (
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
	old "github.com/kimjooyoon/gooo-decision-runtime/studies/contract-goals-20261010/record"
)

const Compiler = "0d61324996d7723f4c8109cbb8508de8de3cb72c"

// Frozen after source-only preflight corrections and before any model fitting.
const CorpusSHA = "890e4b1ee557fd7fc4938ca95e35884893386a8eece5979d2c97e9a2f3ce147f"

type Spec struct {
	ID, Pair, Family, Form, Split                      string
	K                                                  int64
	Reverse, OutputGoal, ConditionGoal, Wording, Order int
}
type FrozenSource struct {
	Spec Spec
	Gooo string
}
type Candidate = old.Candidate
type Search = old.Search
type Group = old.Group
type Source struct {
	Spec                              Spec
	Gooo, SourceSHA, PlanSHA, CaseSHA string
	Document                          pathplan.Document
	Inputs                            [][384]float32
	InputSHA                          [2]string
	CaseRows, ConditionRows           [][32]float32
	Acceptable                        uint64
	Candidates                        []Candidate
}
type Training struct {
	ID, PlanSHA, CaseSHA string
	Inputs               [][384]float32
	Cases, Conditions    [][32]float32
	Masks                []uint16
	Acceptable           uint64
}
type ModelReport struct {
	SHA, Fingerprint, Schema                        string
	Parameters, WeightBytes, ArtifactBytes          int
	TrainingNS, PredictionMedianNS, PredictionP95NS int64
	FirstLoss, LastLoss                             float64
}
type Report struct {
	Schema, Producer, Compiler, CorpusSHA, TrainingSHA, RecordsSHA                             string
	Sources, TrainingRows, Fits, Searches, ModelCalls, NativeExecutions                        int
	OracleCandidates, OracleCompileCalls, OracleOutputEvaluations, OracleConditionObservations int
	Options                                                                                    contractdecision.FitOptions
	Models                                                                                     map[string]ModelReport
	Groups                                                                                     map[string]Group
	AuditedIDs                                                                                 []string
	InputAudits                                                                                map[string]*contractdecision.InputAudit
}

var Hash = old.Hash
var Encode = old.Encode
var FeatureSHA = old.FeatureSHA
