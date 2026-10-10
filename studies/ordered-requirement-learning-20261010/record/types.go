package record

import (
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
	old "github.com/kimjooyoon/gooo-decision-runtime/studies/contract-goals-20261010/record"
	requirements "github.com/kimjooyoon/gooo-decision-runtime/studies/requirement-learning-20261010/record"
)

const Compiler = "8a759cba557b6c6e22aeea57040e8e1c69e6ec68"

// Frozen after source-only preflight corrections and before any model fitting.
const CorpusSHA = "49e29239b9381627eddc01aaeb500541c0820eaecd4c607d739e34e5d953a96f"

type Spec = requirements.Spec
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
	OrderedInputs                     [][528]float32
	OrderedSHA                        [2]string
	OrderedReason                     string
	CaseRows, ConditionRows           [][32]float32
	Acceptable                        uint64
	Candidates                        []Candidate
}
type Training struct {
	ID, PlanSHA, CaseSHA string
	Inputs               [][384]float32
	OrderedInputs        [][528]float32
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
	AuditedIDs                                                                                 map[string][]string
	InputAudits                                                                                map[string]*contractdecision.InputAudit
}

var Hash = old.Hash
var Encode = old.Encode
var FeatureSHA = old.FeatureSHA

var Gooo = requirements.Gooo
var Body = requirements.Body
var Cases = requirements.Cases
var Conditions = requirements.Conditions
