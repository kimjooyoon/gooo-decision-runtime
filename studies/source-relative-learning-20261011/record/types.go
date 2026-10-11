package record

import (
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
	previous "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-role-learning-20261011/record"
)

type Candidate = previous.Candidate
type Observations = previous.Observations
type Rows = previous.Rows
type Model = previous.Model
type Prediction = previous.Prediction

var Hash = previous.Hash
var Encode = previous.Encode
var Passed = previous.Passed

type Source struct {
	Frozen
	SourceSHA, PlanSHA, CasesSHA    string
	Document                        pathplan.Document
	CanonicalInputs, RelativeInputs [][528]float32
	Relations                       []pathplan.SourceRelativeConditionContext
	OutputRows, ConditionRows       [][32]float32
	Acceptable, ProbeAcceptable     uint64
	Candidates                      []Candidate
}
type TrainingRecord struct {
	ID, PlanSHA, CasesSHA           string
	CanonicalInputs, RelativeInputs [][528]float32
	Outputs, Conditions             [][32]float32
	Acceptable                      uint64
}
type Search struct {
	ID, Mode, Split, Family, Form                    string
	InputSHA                                         string
	Predictions                                      []Prediction
	Logits                                           [2][2]float32
	Order                                            [4]uint16
	Attempts                                         []Candidate
	Selected                                         *uint16
	ResultMask                                       uint16
	ResultState                                      string
	ResultPassed, ResultTotal                        int
	Probe                                            Observations
	PreparationNS, PredictionNS, RuleNS, ProbeNS, NS int64
	RuleBucketChecks, RelationRows                   int
}
type Group struct {
	Documents, FirstDeclared, FirstProbe, FinalDeclared, ResultProbe int
	ZeroValidPalette, MultipleValidPalette, UniqueRoleDocuments      int
	ComparisonCorrect, BranchCorrect, CandidateChecks, ModelCalls    int
	PartialDocuments, ResultPassed, ResultTotal                      int
	RuleBucketChecks, RelationRows                                   int
	SearchNS, PreparationNS, PredictionNS, RuleNS, ProbeNS           int64
}
type Report struct {
	Schema, Producer, Compiler, SDKBase, CorpusSHA    string
	Options                                           contractdecision.FitOptions
	Sources, TrainingRows, Fits, Searches, ModelCalls int
	Models                                            map[string]Model
	Groups                                            map[string]Group
}

func Score(o Observations) (passed, total int) {
	for _, row := range o.Outputs {
		total++
		if row.Passed {
			passed++
		}
	}
	for _, row := range o.Conditions {
		total++
		if row.Passed {
			passed++
		}
	}
	return passed, total
}
