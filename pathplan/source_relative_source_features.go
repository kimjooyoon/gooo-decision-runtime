package pathplan

import (
	"errors"
	"fmt"
)

// SourceRelativeOrderedSourceFeaturesInto prepares the published source-relative
// ABI: canonical source/intent/expressions plus condition relations at [192:208].
// It preserves the caller's destination on failure. All remaining reserved cells
// stay zero. This reads declared requirements without compiling a candidate.
func (input *ContractInput) SourceRelativeOrderedSourceFeaturesInto(id string, out *[528]float32) error {
	if out == nil {
		return errors.New("source-relative destination required")
	}
	var row [528]float32
	if err := input.CanonicalOrderedSourceFeaturesInto(id, &row); err != nil {
		return err
	}
	view, err := input.SourceRelativeConditions(id)
	if err != nil {
		return err
	}
	if !view.Available {
		return fmt.Errorf("SOURCE_RELATIVE_CONDITION_CONTEXT_UNAVAILABLE: %s", view.Reason)
	}
	copy(row[192:208], view.Features[:])
	*out = row
	return nil
}
