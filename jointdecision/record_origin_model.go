package jointdecision

import "errors"

func (m *ThreeModel) PredictRecordOriginSharedInto(text string, workspace *ThreeWorkspace, output *ThreePrediction) error {
	if m == nil || m.inner == nil || !m.shared || m.FeatureVersion() != RecordOriginSharedFeatureVersion ||
		workspace == nil || output == nil {
		return errors.New("origin shared model/workspace/output required")
	}
	var features [ThreeFeatureDim]float32
	if err := FeaturesIntoRecordOriginThree(text, &features); err != nil {
		return err
	}
	return m.predictRecordSharedFeatures(&features, workspace, output)
}

func (m *ThreeModel) PredictRecordOriginSharedFeaturesInto(features *[ThreeFeatureDim]float32, workspace *ThreeWorkspace, output *ThreePrediction) error {
	if m == nil || m.inner == nil || !m.shared || m.FeatureVersion() != RecordOriginSharedFeatureVersion ||
		features == nil || workspace == nil || output == nil || !finite(features[:]) {
		return errors.New("finite prepared origin model/features/workspace/output required")
	}
	return m.predictRecordSharedFeatures(features, workspace, output)
}
