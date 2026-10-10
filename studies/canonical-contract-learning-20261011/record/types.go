package record

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

func Hash(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func Encode(value any) []byte {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return append(raw, '\n')
}

type Frozen struct {
	Spec            Spec
	Gooo            string
	Probes          []pathplan.TestCase
	ProbeConditions []pathplan.ConditionCase
}

type Observations struct {
	Outputs    []pathplan.TestResult
	Conditions []pathplan.ConditionResult
}

type Candidate struct {
	Mask                        uint16
	Gooo, Go                    string
	Declared, Probe             Observations
	Acceptable, ProbeAcceptable bool
}

type Source struct {
	Frozen
	SourceSHA, PlanSHA, CasesSHA  string
	Document                      pathplan.Document
	LegacyInputs, CanonicalInputs [][528]float32
	OutputRows, ConditionRows     [][32]float32
	Acceptable, ProbeAcceptable   uint64
	Candidates                    []Candidate
}

type Stage struct {
	Number                             int
	Gooo, SourceSHA, PlanSHA, CasesSHA string
	Document                           pathplan.Document
	Ranking                            pathplan.ContractRanking
	Progress                           []pathplan.ContractProgress
	Probe                              Observations
	NextOutput                         *pathplan.TestCase
	NextCondition                      *pathplan.ConditionCase
	NS                                 int64
}

type Search struct {
	ID, Mode, Split, Family, Form string
	Stages                        []Stage
}

type Model struct {
	SHA, Fingerprint, Schema               string
	Parameters, WeightBytes, ArtifactBytes int
	FitNS                                  int64
	FirstLoss, LastLoss                    float64
}

type Group struct {
	Programs, FirstDeclaredComplete, FirstProbeComplete, InitialDeclaredComplete, InitialProbeComplete, FinalDeclaredComplete, FinalProbeComplete int
	FirstOutputPassed, FirstOutputTotal, FirstConditionPassed, FirstConditionTotal                                                                int
	FirstProbeOutputPassed, FirstProbeOutputTotal, FirstProbeConditionPassed, FirstProbeConditionTotal                                            int
	InitialAttempts, TotalAttempts, FeedbackStages, ModelCalls                                                                                    int
	SearchNS, PredictNS                                                                                                                           int64
}

type Report struct {
	Schema, Producer, Compiler, SDKBase, CorpusSHA, SourcesSHA, SearchesSHA    string
	Sources, TrainingRows, Fits, InitialSearches, FeedbackSearches, ModelCalls int
	Options                                                                    contractdecision.FitOptions
	Models                                                                     map[string]Model
	Groups                                                                     map[string]Group
	InputAudits                                                                map[string]*contractdecision.InputAudit
}

type Rows [][32]float32

func (r Rows) CaseFeatureVersion() string      { return decision.DeclaredCaseFeatureVersion }
func (r Rows) ConditionFeatureVersion() string { return decision.DeclaredConditionFeatureVersion }

func (r Rows) CaseCount() int                               { return len(r) }
func (r Rows) CaseFeatures(i int) ([32]float32, error)      { return r[i], nil }
func (r Rows) ConditionCount() int                          { return len(r) }
func (r Rows) ConditionFeatures(i int) ([32]float32, error) { return r[i], nil }

func Passed(o Observations) bool {
	if len(o.Outputs) == 0 {
		return false
	}
	for _, result := range o.Outputs {
		if !result.Passed {
			return false
		}
	}
	return pathplan.ConditionsPassed(o.Conditions)
}
