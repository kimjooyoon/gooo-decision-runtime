package jointdecision

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func graphFixture() RecordGraphInput {
	input := RecordGraphInput{Nodes: []RecordGraphNode{
		{Kind: "input", Input: 1},
		{Kind: "input", Input: 2},
		{Kind: "expression", Operator: "&&", Parents: [2]uint16{1, 2}},
		{Kind: "expression", Operator: "||", Parents: [2]uint16{1, 2}},
		{Kind: "literal", Operator: "INT", Literal: "0"},
		{Kind: "literal", Operator: "INT", Literal: "1"},
	}}
	for i, field := range []string{"allowed", "value", "count"} {
		input.Choices[i] = RecordGraphChoice{RecordChoice: RecordChoice{field, "first", "second", "두 조건을 모두 만족한다. Require both conditions."}, FieldID: "gooo://example/" + field}
	}
	input.Choices[0].Roots, input.Choices[1].Roots, input.Choices[2].Roots = [2]uint16{3, 4}, [2]uint16{1, 2}, [2]uint16{5, 6}
	return input
}

func graphArray(t *testing.T, input RecordGraphInput) [ThreeFeatureDim]float32 {
	t.Helper()
	text, err := EncodeRecordGraphThree(input)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeRecordGraphThree(text)
	if err != nil || !reflect.DeepEqual(input, decoded) {
		t.Fatal("source graph changed in transport", err)
	}
	var features [ThreeFeatureDim]float32
	if err = FeaturesIntoRecordGraphThree(text, &features); err != nil {
		t.Fatal(err)
	}
	return features
}

func TestRecordGraphEightChoiceOrdersRemainDistinct(t *testing.T) {
	seen := map[[ThreeFeatureDim]float32]bool{}
	for order := range 8 {
		input := graphFixture()
		for part := range 3 {
			if order&(1<<part) != 0 {
				choice := &input.Choices[part]
				choice.First, choice.Second = choice.Second, choice.First
				choice.Roots[0], choice.Roots[1] = choice.Roots[1], choice.Roots[0]
			}
		}
		features := graphArray(t, input)
		if seen[features] {
			t.Fatal("operator, input identity or literal choice collided", order)
		}
		seen[features] = true
		var square float64
		for _, value := range features {
			square += float64(value) * float64(value)
		}
		if math.Abs(square-1) > 1e-6 {
			t.Fatal("graph feature normalization differs", square)
		}
	}
}

func TestRecordGraphOperandAndConditionOrderRemainVisible(t *testing.T) {
	base := graphFixture()
	base.Nodes[2].Operator = "-"
	original := graphArray(t, base)
	base.Nodes[2].Parents = [2]uint16{2, 1}
	if original == graphArray(t, base) {
		t.Fatal("subtraction operands lost their order")
	}
	base = graphFixture()
	base.Nodes = append(base.Nodes, RecordGraphNode{Kind: "join", Parents: [2]uint16{5, 6}, Condition: 3})
	base.Choices[2].Roots = [2]uint16{7, 5}
	original = graphArray(t, base)
	base.Nodes[6].Parents = [2]uint16{6, 5}
	if original == graphArray(t, base) {
		t.Fatal("conditional arms lost their order")
	}
	base.Nodes[6].Parents = [2]uint16{5, 6}
	base.Nodes[6].Condition = 4
	if original == graphArray(t, base) {
		t.Fatal("conditional join lost its selecting expression")
	}
}

func TestRecordGraphAliasNamesAndUnusedNodesDoNotChangeFeatures(t *testing.T) {
	input := graphFixture()
	want := graphArray(t, input)
	input.Nodes = append(input.Nodes, RecordGraphNode{Kind: "copy", Parents: [2]uint16{3}},
		RecordGraphNode{Kind: "read", Parents: [2]uint16{7}}, RecordGraphNode{Kind: "literal", Operator: "STRING", Literal: `"unused"`})
	input.Choices[0].First, input.Choices[0].Roots[0] = "renamedLocal", 8
	if got := graphArray(t, input); got != want {
		t.Fatal("pure alias spelling or unreachable source node changed features")
	}
	input.Nodes = append(input.Nodes, RecordGraphNode{Kind: "guard", Operator: "true", Parents: [2]uint16{4}},
		RecordGraphNode{Kind: "read", Parents: [2]uint16{3}, Guard: 10})
	input.Choices[0].Roots[0] = 11
	if got := graphArray(t, input); got == want {
		t.Fatal("execution guard was discarded with the alias")
	}
}

func TestRecordGraphNodeNumbersAndLiteralIdentity(t *testing.T) {
	input := graphFixture()
	want := graphArray(t, input)
	input.Nodes[0], input.Nodes[1] = input.Nodes[1], input.Nodes[0]
	remap := func(id uint16) uint16 {
		if id == 1 || id == 2 {
			return 3 - id
		}
		return id
	}
	for i := range input.Nodes {
		for j := range input.Nodes[i].Parents {
			input.Nodes[i].Parents[j] = remap(input.Nodes[i].Parents[j])
		}
	}
	for i := range input.Choices {
		for j := range input.Choices[i].Roots {
			input.Choices[i].Roots[j] = remap(input.Choices[i].Roots[j])
		}
	}
	if graphArray(t, input) != want {
		t.Fatal("node numbering changed semantic features")
	}
	for _, pair := range [][3]string{{"BOOL", "true", "false"}, {"STRING", `"한글"`, `"English"`},
		{"INT", "9007199254740992", "9007199254740993"}, {"STRING", `"A"`, `"a"`}} {
		input := graphFixture()
		input.Nodes[4].Operator, input.Nodes[4].Literal = pair[0], pair[1]
		before := graphArray(t, input)
		input.Nodes[4].Literal = pair[2]
		if graphArray(t, input) == before {
			t.Fatal("distinct source constants collided in the fixture", pair)
		}
	}
}

func TestRecordGraphInvalidInputPreservesOutput(t *testing.T) {
	for _, mutate := range []func(*RecordGraphInput){
		func(i *RecordGraphInput) { i.Nodes = nil },
		func(i *RecordGraphInput) { i.Nodes = append(i.Nodes, make([]RecordGraphNode, 512)...) },
		func(i *RecordGraphInput) { i.Nodes[2].Parents[0] = 3 },
		func(i *RecordGraphInput) { i.Nodes[2].Guard = 999 },
		func(i *RecordGraphInput) { i.Nodes[0].Input = 0 },
		func(i *RecordGraphInput) { i.Nodes[0].Input = 17 },
		func(i *RecordGraphInput) { i.Nodes[2].Operator = "unknown" },
		func(i *RecordGraphInput) { i.Nodes[2].Operator = "!" },
		func(i *RecordGraphInput) { i.Nodes[4].Literal = "0x00" },
		func(i *RecordGraphInput) { i.Nodes[4].Literal = "1.0" },
		func(i *RecordGraphInput) { i.Nodes[4].Literal = string([]byte{0xff}) },
		func(i *RecordGraphInput) { i.Nodes[4].Literal = strings.Repeat("0", 1025) },
		func(i *RecordGraphInput) { i.Choices[0].Roots[0] = 0 },
		func(i *RecordGraphInput) { i.Choices[0].Roots[0] = 7 },
		func(i *RecordGraphInput) { i.Choices[0].Intent = "" },
	} {
		input := graphFixture()
		mutate(&input)
		if _, err := EncodeRecordGraphThree(input); err == nil {
			t.Fatal("invalid graph encoded", input)
		}
		raw, _ := json.Marshal(input)
		out := [ThreeFeatureDim]float32{99}
		before := out
		if FeaturesIntoRecordGraphThree(recordGraphPrefix+string(raw), &out) == nil || out != before {
			t.Fatal("invalid graph changed caller storage")
		}
	}
	text, _ := EncodeRecordGraphThree(graphFixture())
	for _, bad := range []string{"", text + "x", text[:len(text)-1], text + strings.Repeat(" ", RecordGraphInputMaxBytes),
		strings.Replace(text, `"choices":`, `"extra":0,"choices":`, 1)} {
		out := [ThreeFeatureDim]float32{99}
		if FeaturesIntoRecordGraphThree(bad, &out) == nil || out[0] != 99 {
			t.Fatal("noncanonical text changed output")
		}
	}
}

func TestRecordGraphSharedModelsRequireNewContract(t *testing.T) {
	text, _ := EncodeRecordGraphThree(graphFixture())
	for _, variant := range []string{"fp32", "ptq_ternary", "qat_ternary"} {
		t.Run(variant, func(t *testing.T) {
			name, meta, raw := recordSharedFixture(t, variant)
			if _, err := LoadRecordGraphSharedThree(name); err == nil {
				t.Fatal("v1 weights accepted as graph weights")
			}
			// Synthetic ABI weights, never a trained-quality observation.
			meta.Feature, meta.MaxBytes = RecordGraphSharedFeatureVersion, RecordGraphInputMaxBytes
			writeFixture(t, name, meta, raw)
			model, err := LoadRecordGraphSharedThree(name)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = LoadRecordOriginSharedThree(name); err == nil {
				t.Fatal("v2 accepted graph model metadata")
			}
			var workspace, prepared ThreeWorkspace
			var prediction, again ThreePrediction
			if err := model.PredictRecordGraphSharedInto(text, &workspace, &prediction); err != nil {
				t.Fatal(err)
			}
			if model.PredictRecordGraphSharedFeaturesInto(&workspace.Features, &prepared, &again) != nil || workspace != prepared || prediction != again {
				t.Fatal("text and prepared graph inference differ")
			}
			if allocations := testing.AllocsPerRun(100, func() {
				if model.PredictRecordGraphSharedFeaturesInto(&workspace.Features, &prepared, &again) != nil {
					t.Fatal("prepared inference failed")
				}
			}); allocations != 0 {
				t.Fatal("prepared inference allocated", allocations)
			}
			var wait sync.WaitGroup
			for range 12 {
				wait.Go(func() {
					var w ThreeWorkspace
					var p ThreePrediction
					if model.PredictRecordGraphSharedInto(text, &w, &p) != nil || p != prediction {
						t.Error("concurrent graph request differs")
					}
				})
			}
			wait.Wait()
			before, prior := workspace, prediction
			if model.PredictRecordGraphSharedInto("broken", &workspace, &prediction) == nil || workspace != before || prediction != prior {
				t.Fatal("invalid text changed prediction")
			}
			if model.PredictRecordOriginSharedInto(originText(t), &workspace, &prediction) == nil || workspace != before || prediction != prior {
				t.Fatal("graph model accepted v2 prediction contract")
			}
		})
	}
}
