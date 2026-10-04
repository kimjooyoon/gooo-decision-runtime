package jointdecision

import (
	"math"
	"strings"
	"sync"
	"testing"
)

func recordChoices() [3]RecordChoice {
	return [3]RecordChoice{
		{Field: "title", First: `"draft"`, Second: "input0.title", Intent: "원래 제목을 사용한다. Keep the original title."},
		{Field: "state", First: `"wait"`, Second: `"ready"`, Intent: "Set state to ready."},
		{Field: "reason", First: `"deferred"`, Second: `input0.reason + ":accepted"`, Intent: "기존 사유 뒤에 accepted를 붙인다."},
	}
}

func TestRecordSourceFeaturesAndCompleteIntent(t *testing.T) {
	choices := recordChoices()
	text, err := EncodeRecordThree(choices)
	if err != nil {
		t.Fatal(err)
	}
	var original [ThreeFeatureDim]float32
	if err = FeaturesIntoRecordThree(text, &original); err != nil {
		t.Fatal(err)
	}
	var norm float64
	for _, value := range original {
		norm += float64(value) * float64(value)
	}
	if math.Abs(norm-1) > 1e-6 || !strings.Contains(text, choices[0].Intent) {
		t.Fatal("complete intent or normalization differs", norm)
	}
	// All current source alternatives are carried; neither outcomes nor stable
	// field IDs are substituted for structural expression observations.
	parts, _ := ThreeParts(text)
	if strings.Contains(text, "expected") || !strings.Contains(parts[2], "input0.reason") {
		t.Fatal("record expressions lost or outcomes included")
	}
	for _, change := range []func(*[3]RecordChoice){
		func(c *[3]RecordChoice) { c[0].First, c[0].Second = c[0].Second, c[0].First },
		func(c *[3]RecordChoice) { c[0].Second = "input0.reason" },
		func(c *[3]RecordChoice) { c[2].Second = `":accepted" + input0.reason` },
		func(c *[3]RecordChoice) { c[0].Intent = "Use a draft title." },
	} {
		changed := choices
		change(&changed)
		encoded, err := EncodeRecordThree(changed)
		var got [ThreeFeatureDim]float32
		if err != nil || FeaturesIntoRecordThree(encoded, &got) != nil || got == original {
			t.Fatal("ordered alternative/field/intent was invisible", err)
		}
	}
	choices[0].Field, choices[0].Second = "caption", "renamed.caption"
	renamed, _ := EncodeRecordThree(choices)
	var got [ThreeFeatureDim]float32
	if FeaturesIntoRecordThree(renamed, &got) != nil || got != original {
		t.Fatal("consistent field/local renaming changed structural meaning")
	}
	choices = recordChoices()
	choices[0].Second = "(((input0.title)))"
	parenthesized, _ := EncodeRecordThree(choices)
	if FeaturesIntoRecordThree(parenthesized, &got) != nil || got != original {
		t.Fatal("parentheses changed expression features")
	}
}

func TestRecordFeatureFailuresKeepCallerStorage(t *testing.T) {
	text, _ := EncodeRecordThree(recordChoices())
	var output [ThreeFeatureDim]float32
	if FeaturesIntoRecordThree(text, &output) != nil {
		t.Fatal("valid context rejected")
	}
	before := output
	legacy, _ := EncodeThree(threeParts(t))
	for _, bad := range []string{legacy, "", text + "x", strings.Replace(text, `input0.title`, `input0.`, 1),
		strings.Replace(text, `"field":"title"`, `"field":"title","field":"title"`, 1)} {
		if FeaturesIntoRecordThree(bad, &output) == nil || output != before {
			t.Fatal("failed record input changed caller output")
		}
	}
	if FeaturesIntoThree(text, &output) == nil || output != before || FeaturesIntoRecordThree(text, nil) == nil {
		t.Fatal("legacy ABI accepted record context or nil storage")
	}
	for _, value := range []string{strings.Repeat("한", 180), string([]byte{0xff})} {
		choices := recordChoices()
		choices[0].Intent = value
		if _, err := EncodeRecordThree(choices); err == nil {
			t.Fatal("oversize or invalid UTF-8 intent was shortened")
		}
	}
}

func TestRecordModelVariantsAndConcurrentWorkspaces(t *testing.T) {
	text, _ := EncodeRecordThree(recordChoices())
	for _, variant := range []string{"fp32", "ptq_ternary", "qat_ternary"} {
		name, meta, weights := threeFixture(t, variant)
		if _, err := LoadRecordThree(name); err == nil {
			t.Fatal("record loader accepted integer-trained feature metadata")
		}
		meta.Feature = RecordFieldFeatureVersion
		writeFixture(t, name, meta, weights)
		model, err := LoadRecordThree(name)
		if err != nil || model.FeatureVersion() != RecordFieldFeatureVersion {
			t.Fatal("record-specific artifact", err)
		}
		if _, err = LoadThree(name); err == nil {
			t.Fatal("integer loader accepted record-trained feature metadata")
		}
		var group sync.WaitGroup
		for range 4 {
			group.Go(func() {
				var workspace ThreeWorkspace
				var prediction ThreePrediction
				if err := model.PredictRecordInto(text, &workspace, &prediction); err != nil || prediction.Mask != 7 {
					t.Error("record prediction", err, prediction)
				}
				prior, scratch := prediction, workspace
				if model.PredictRecordInto("invalid", &workspace, &prediction) == nil || prior != prediction || scratch != workspace {
					t.Error("failed record prediction changed owned storage")
				}
			})
		}
		group.Wait()
	}
}

func BenchmarkRecordFeatures(b *testing.B) {
	text, _ := EncodeRecordThree(recordChoices())
	var output [ThreeFeatureDim]float32
	b.ReportAllocs()
	for b.Loop() {
		if err := FeaturesIntoRecordThree(text, &output); err != nil {
			b.Fatal(err)
		}
	}
}
