package criteriadecision

import (
	"math"
	"strings"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime/criteriainput"
)

// Synthetic protocol rows carry no claim of compiler-source provenance.
func scalarTestFeatures(choices int) criteriainput.Features {
	return criteriainput.Features{Schema: criteriainput.FeatureSchema,
		InputSHA256: "sha256:" + strings.Repeat("1", 64), CandidatePlanSHA256: strings.Repeat("2", 64),
		ChoiceCount: uint8(choices), PredicateCount: 1}
}

func scalarTestModel(t *testing.T, parameters *[ParameterCount]float32) *Model {
	t.Helper()
	metadata, weights, err := EncodeArtifact(parameters)
	if err != nil {
		t.Fatal(err)
	}
	model, err := Load(metadata, weights)
	if err != nil {
		t.Fatal(err)
	}
	return model
}

func TestScalarUnloadedModelDoesNotRank(t *testing.T) {
	var model Model
	features := scalarTestFeatures(1)
	var work Workspace
	before := Prediction{Schema: "unchanged", ProposedMask: 7}
	output := before
	if model.PredictInto(&features, &work, &output) == nil {
		t.Fatal("uninitialized model was accepted")
	}
	if output != before {
		t.Fatal("declined model changed output")
	}
}

func TestScalarCompleteUniformRanking(t *testing.T) {
	var parameters [ParameterCount]float32
	model := scalarTestModel(t, &parameters)
	for choices := 1; choices <= criteriainput.MaxChoices; choices++ {
		features := scalarTestFeatures(choices)
		var work Workspace
		var output Prediction
		if err := model.PredictInto(&features, &work, &output); err != nil {
			t.Fatal(err)
		}
		count := 1 << choices
		if int(output.CandidateCount) != count || output.ProposedMask != 0 {
			t.Fatalf("unexpected complete rank count/proposal for %d choices", choices)
		}
		var total float64
		for i, candidate := range output.Candidates[:count] {
			if int(candidate.Mask) != i || candidate.Score != 0 || math.Abs(candidate.Probability-1/float64(count)) > 1e-12 {
				t.Fatalf("unexpected uniform candidate %d", i)
			}
			total += candidate.Probability
		}
		if math.Abs(total-1) > 1e-12 {
			t.Fatal("joint probabilities do not sum to one")
		}
		for _, marginal := range output.Marginals[:choices] {
			if math.Abs(marginal[0]-0.5) > 1e-12 || math.Abs(marginal[1]-0.5) > 1e-12 {
				t.Fatal("marginals differ from the joint distribution")
			}
		}
		featureSHA, err := FeatureSHA256(&features)
		if err != nil || output.FeatureSHA256 != featureSHA || output.ModelMetadataSHA256 == "" {
			t.Fatal("ranking bindings are incomplete")
		}
	}
}

func TestScalarPairTermsRankJointChoices(t *testing.T) {
	var parameters [ParameterCount]float32
	parameters[mixBias], parameters[pairWeights] = 1, 2
	model := scalarTestModel(t, &parameters)
	features := scalarTestFeatures(2)
	var work Workspace
	var output Prediction
	if err := model.PredictInto(&features, &work, &output); err != nil {
		t.Fatal(err)
	}
	for i, mask := range [4]uint16{0, 3, 1, 2} {
		if output.Candidates[i].Mask != mask {
			t.Fatal("pair coupling or deterministic tie order changed")
		}
	}
	if output.Candidates[0].Score <= output.Candidates[2].Score ||
		output.Candidates[0].Probability <= output.Candidates[2].Probability {
		t.Fatal("pair energy did not affect the joint ranking")
	}
}

func TestScalarFeatureDigestIgnoresMetadataNames(t *testing.T) {
	features := scalarTestFeatures(2)
	features.Choices[0].ID, features.Predicates[0].ID = "choice://original", "predicate://original"
	features.Choices[0].Values[0], features.Predicates[0].Values[1] = 0.5, 0.25
	before, err := FeatureSHA256(&features)
	if err != nil {
		t.Fatal(err)
	}
	features.Choices[0].ID, features.Predicates[0].ID = "choice://renamed", "predicate://renamed"
	features.Choices[0].Options = [2]string{"한국어", "English"}
	after, err := FeatureSHA256(&features)
	if err != nil || after != before {
		t.Fatal("binding names entered the numeric digest")
	}
	features.Choices[0].Values[0] = 0.75
	changed, err := FeatureSHA256(&features)
	if err != nil || changed == before {
		t.Fatal("active numerical change was not bound")
	}
}

func TestScalarModelArtifactsRejectMalformed(t *testing.T) {
	var parameters [ParameterCount]float32
	parameters[0] = float32(math.NaN())
	if _, _, err := EncodeArtifact(&parameters); err == nil {
		t.Fatal("non-finite parameter accepted")
	}
	parameters[0] = 0
	metadata, weights, err := EncodeArtifact(&parameters)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Load(metadata, weights[:len(weights)-1]); err == nil {
		t.Fatal("incomplete weight carrier accepted")
	}
	model, err := Load(metadata, weights)
	if err != nil {
		t.Fatal(err)
	}
	weights[0] = 1
	if _, err = Load(metadata, weights); err == nil {
		t.Fatal("changed weight identity accepted")
	}
	if model.weights[0] != 0 {
		t.Fatal("caller mutation changed the loaded model")
	}
}
