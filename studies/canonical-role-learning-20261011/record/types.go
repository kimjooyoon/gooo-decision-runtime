package record

import (
	"errors"
	"math/bits"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	previous "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-pooling-20261011/record"
)

type Source = previous.Source
type Candidate = previous.Candidate
type Observations = previous.Observations
type Rows = previous.Rows
type Model = previous.Model

var Hash = previous.Hash
var Encode = previous.Encode
var Passed = previous.Passed

type TrainingRecord struct {
	ID, PlanSHA, CasesSHA       string
	CanonicalInputs             [][528]float32
	Outputs, Conditions         [][32]float32
	Joint, Comparison, Branches uint64
}
type Prediction struct {
	Model, Fingerprint string
	Logits             [2][2]float32
	NS                 int64
}
type Search struct {
	ID, Mode, Split, Family, Form string
	Predictions                   []Prediction
	Logits                        [2][2]float32
	Order                         [4]uint16
	Attempts                      []Candidate
	Selected                      *uint16
	Probe                         Observations
	NS                            int64
}
type Group struct {
	Documents, FirstDeclared, FirstProbe, FinalDeclared, FinalProbe int
	ComparisonCorrect, BranchCorrect, CandidateChecks, ModelCalls   int
	SearchNS, PredictionNS                                          int64
}
type Report struct {
	Schema, Producer, Compiler, SDKBase, CorpusSHA    string
	Options                                           contractdecision.FitOptions
	Sources, TrainingRows, Fits, Searches, ModelCalls int
	Models                                            map[string]Model
	Groups                                            map[string]Group
}

// ProjectRole keeps the accepted bit and allows either value of the other bit.
// Bits name source choices, not candidate-array positions outside this four-mask study.
func ProjectRole(accepted uint64, role int) (uint64, error) {
	if role < 0 || role > 1 || bits.OnesCount64(accepted) != 1 || accepted & ^uint64(15) != 0 {
		return 0, errors.New("one accepted four-mask candidate and role 0 or 1 required")
	}
	mask := bits.TrailingZeros64(accepted)
	var allowed uint64
	for candidate := range 4 {
		if candidate>>role&1 == mask>>role&1 {
			allowed |= 1 << candidate
		}
	}
	return allowed, nil
}
