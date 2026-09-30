package decision

import "testing"

func TestPositionedPathFeaturesDistinguishAssignmentDirection(t *testing.T) {
	first := "Run the first assignment before the second assignment. [offset=2; factor=3; updated_offset=9]"
	second := "Run the second assignment before the first assignment. [offset=2; factor=3; updated_offset=9]"
	var uniformFirst, uniformSecond, positionFirst, positionSecond [FeatureDim]float32
	if err := FeaturesInto(first, &uniformFirst); err != nil {
		t.Fatal(err)
	}
	if err := FeaturesInto(second, &uniformSecond); err != nil {
		t.Fatal(err)
	}
	if uniformFirst != uniformSecond {
		t.Fatal("the recorded uniform-feature collision changed")
	}
	model := &Model{metadata: Metadata{FeatureVersion: PositionedIntentFeatureVersion}}
	if err := model.FeaturesInto(first, &positionFirst); err != nil {
		t.Fatal(err)
	}
	if err := model.FeaturesInto(second, &positionSecond); err != nil {
		t.Fatal(err)
	}
	if positionFirst == positionSecond {
		t.Fatal("positioned features still collapse assignment direction")
	}
	withContext := "activity Select(Integer)->Integer\nintent: " + first
	if allocations := testing.AllocsPerRun(1000, func() {
		if err := model.FeaturesInto(withContext, &positionFirst); err != nil {
			panic(err)
		}
	}); allocations != 0 {
		t.Fatalf("feature path allocated: %f", allocations)
	}
}
