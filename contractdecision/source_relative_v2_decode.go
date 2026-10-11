package contractdecision

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

func DecodeSourceRelativeV2(raw []byte) (*SourceRelativeV2Model, error) {
	if len(raw) == 0 || len(raw) > 512<<10 {
		return nil, errors.New("bounded source-relative V2 artifact required")
	}
	if err := decision.RejectDuplicateJSONKeys(raw); err != nil {
		return nil, err
	}
	var a sourceRelativeRequirementArtifact
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&a); err != nil {
		return nil, err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return nil, errors.New("trailing V2 artifact")
	}
	if a.Schema != SourceRelativeV2Schema || a.SourceFeatures != SourceRelativeV2FeatureVersion {
		return nil, errors.New("source-relative V2 artifact identity differs")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	// Delegate numeric count, null, finite and all unchanged metadata checks to
	// the V1 decoder only after requiring the V2 schema/feature pair explicitly.
	fields["schema"], _ = json.Marshal(SourceRelativeRequirementSchema)
	fields["source_features"], _ = json.Marshal(SourceRelativeOrderedSourceFeatureVersion)
	translated, err := json.Marshal(fields)
	if err != nil {
		return nil, err
	}
	model, err := DecodeSourceRelativeRequirementConditioned(translated)
	if err != nil {
		return nil, err
	}
	return NewSourceRelativeV2(model.inner.Weights())
}
