package jointdecision

import "errors"

func LoadRecordGraphSharedThree(name string) (*ThreeModel, error) {
	return loadRecordSharedContract(name, RecordGraphSharedFeatureVersion, RecordGraphInputMaxBytes)
}

func (m *ThreeModel) PredictRecordGraphSharedInto(text string, workspace *ThreeWorkspace, output *ThreePrediction) error {
	if m == nil || m.inner == nil || !m.shared || m.FeatureVersion() != RecordGraphSharedFeatureVersion ||
		workspace == nil || output == nil {
		return errors.New("source graph shared model/workspace/output required")
	}
	var features [ThreeFeatureDim]float32
	if err := FeaturesIntoRecordGraphThree(text, &features); err != nil {
		return err
	}
	return m.predictRecordSharedFeatures(&features, workspace, output)
}

func (m *ThreeModel) PredictRecordGraphSharedFeaturesInto(features *[ThreeFeatureDim]float32,
	workspace *ThreeWorkspace, output *ThreePrediction) error {
	if m == nil || m.inner == nil || !m.shared || m.FeatureVersion() != RecordGraphSharedFeatureVersion ||
		features == nil || workspace == nil || output == nil || !finite(features[:]) {
		return errors.New("finite prepared source graph model/features/workspace/output required")
	}
	return m.predictRecordSharedFeatures(features, workspace, output)
}
