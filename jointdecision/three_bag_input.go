package jointdecision

import (
	"errors"
	"math"

	"github.com/kimjooyoon/gooo-decision-runtime"
)

const ThreeBagFeatureVersion = "triple_semantic_context_bag_v4_joint_v1"

// FeaturesIntoThreeBag uses the same full-input framing and caller dimensions
// as v3, with an explicit, separately versioned intent representation.
func FeaturesIntoThreeBag(text string, output *[ThreeFeatureDim]float32) error {
	if output == nil {
		return errors.New("three-choice bag feature output required")
	}
	parts, err := ThreeParts(text)
	if err != nil {
		return err
	}
	var candidate [ThreeFeatureDim]float32
	var part [decision.FeatureDim]float32
	scale := float32(1 / math.Sqrt(3))
	for i, text := range parts {
		if err = decision.SemanticContextBagFeaturesInto(text, &part); err != nil {
			return err
		}
		for j, v := range part {
			candidate[i*decision.FeatureDim+j] = v * scale
		}
	}
	*output = candidate
	return nil
}

func threeFeatures(text, version string, output *[ThreeFeatureDim]float32) error {
	switch version {
	case ThreeFeatureVersion:
		return FeaturesIntoThree(text, output)
	case ThreeBagFeatureVersion:
		return FeaturesIntoThreeBag(text, output)
	default:
		return errors.New("unsupported three-choice feature version")
	}
}

// LoadThreeBag explicitly loads expanded v4 artifacts for research/parity.
// LoadThree retains the original expanded-v3 contract.
func LoadThreeBag(name string) (*ThreeModel, error) {
	contract := threeContract
	contract.feature = ThreeBagFeatureVersion
	m, err := loadContract(name, contract)
	if err != nil {
		return nil, err
	}
	return &ThreeModel{inner: m, feature: ThreeBagFeatureVersion}, nil
}
