package record

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

const Compiler = "89b000d046ab706bb7d61b91148aa4fd805821f6"

type Candidate struct {
	Mask                 uint16
	GoooSource, GoSource string
	Outputs              []pathplan.TestResult
	Conditions           []pathplan.ConditionResult
	Acceptable           bool
}
type Source struct {
	Spec                              Spec
	Gooo, SourceSHA, PlanSHA, CaseSHA string
	Document                          pathplan.Document
	Inputs                            [][384]float32
	InputSHA                          [2]string
	CaseRows                          [][32]float32
	Acceptable                        uint64
	Candidates                        []Candidate
}
type Training struct {
	ID, PlanSHA, CaseSHA string
	Inputs               [][384]float32
	Cases                [][32]float32
	Masks                []uint16
	Acceptable           uint64
}
type Search struct {
	ID, Split, Family, Mode string
	Ranking                 pathplan.ContractRanking
	Progress                []pathplan.ContractProgress
	GoooBody, GoSource      string
	NS                      int64
}
type Group struct {
	Programs, FirstValid, Complete, Attempts, ModelCalls                           int
	FirstOutputPassed, FirstOutputTotal, FirstConditionPassed, FirstConditionTotal int
	NS, PredictNS                                                                  int64
}
type Report struct {
	Schema, Producer, Compiler, ModelSHA, Fingerprint, TrainingSHA, RecordsSHA                                  string
	Sources, TrainingRows, Fits, Searches, ModelCalls, NativeExecutions, Parameters, WeightBytes, ArtifactBytes int
	OracleCandidates, OracleCompileCalls, OracleOutputEvaluations, OracleConditionObservations                  int
	Options                                                                                                     contractdecision.FitOptions
	TrainingNS                                                                                                  int64
	FirstLoss, LastLoss                                                                                         float64
	PredictionMedianNS, PredictionP95NS                                                                         int64
	Groups                                                                                                      map[string]Group
}

func Hash(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func Encode(v any) []byte {
	b, e := json.Marshal(v)
	if e != nil {
		panic(e)
	}
	return b
}
func FeatureSHA(row [384]float32) string {
	var raw [1536]byte
	for i, v := range row {
		binary.LittleEndian.PutUint32(raw[i*4:], math.Float32bits(v))
	}
	return Hash(raw[:])
}
