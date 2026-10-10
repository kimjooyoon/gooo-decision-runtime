package record

import "github.com/kimjooyoon/gooo-decision-runtime/contractdecision"

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
}
