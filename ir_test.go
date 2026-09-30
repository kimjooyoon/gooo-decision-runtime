package decision

import "testing"

func TestBuildTypedBinaryOperationTypes(t *testing.T) {
	intLeft := Identifier{Name: "left_value", Type: TypeInt}
	intRight := Identifier{Name: "right_value", Type: TypeInt}
	boolLeft := Identifier{Name: "enabled", Type: TypeBool}
	boolRight := Identifier{Name: "selected", Type: TypeBool}
	tests := []struct {
		operation string
		left      Identifier
		right     Identifier
		result    ValueType
	}{
		{"add", intLeft, intRight, TypeInt},
		{"subtract", intLeft, intRight, TypeInt},
		{"multiply", intLeft, intRight, TypeInt},
		{"less_than", intLeft, intRight, TypeBool},
		{"less_equal", intLeft, intRight, TypeBool},
		{"equal", intLeft, intRight, TypeBool},
		{"equal", boolLeft, boolRight, TypeBool},
		{"and", boolLeft, boolRight, TypeBool},
		{"or", boolLeft, boolRight, TypeBool},
	}
	for _, test := range tests {
		t.Run(test.operation+"/"+string(test.left.Type), func(t *testing.T) {
			ir, err := BuildTypedBinary(test.operation, test.left, test.right)
			if err != nil {
				t.Fatal(err)
			}
			if ir.Schema != TypedBinaryIRSchema || ir.Kind != "binary" || ir.Operation != test.operation || ir.ResultType != test.result {
				t.Fatalf("unexpected typed IR: %+v", ir)
			}
		})
	}
}

func TestBuildTypedBinaryRejectsInvalidTypesAndSourceLikeNames(t *testing.T) {
	intValue := Identifier{Name: "value", Type: TypeInt}
	boolValue := Identifier{Name: "valid", Type: TypeBool}
	tests := []struct {
		operation string
		left      Identifier
		right     Identifier
	}{
		{"add", boolValue, boolValue},
		{"less_than", boolValue, boolValue},
		{"and", intValue, intValue},
		{"equal", intValue, boolValue},
		{"input + os.Exit(1)", intValue, intValue},
		{"add", Identifier{Name: "x); panic(1)", Type: TypeInt}, intValue},
		{"add", Identifier{Name: "", Type: TypeInt}, intValue},
		{"add", Identifier{Name: "for", Type: TypeInt}, intValue},
		{"add", Identifier{Name: "_", Type: TypeInt}, intValue},
		{"add", Identifier{Name: "名", Type: TypeInt}, intValue},
		{"add", Identifier{Name: "x", Type: "Float"}, intValue},
	}
	for _, test := range tests {
		t.Run(test.operation+"/"+test.left.Name, func(t *testing.T) {
			if _, err := BuildTypedBinary(test.operation, test.left, test.right); err == nil {
				t.Fatal("invalid operation, identifier, or type was accepted")
			}
		})
	}
}

func TestIdentifierAcceptsOrdinaryASCIIAndRejectsKeywordsAndBlankName(t *testing.T) {
	for _, name := range []string{"left", "right_value", "_private", "value2", "X9"} {
		if _, err := BuildTypedBinary("add", Identifier{Name: name, Type: TypeInt}, Identifier{Name: "other", Type: TypeInt}); err != nil {
			t.Errorf("ordinary ASCII identifier %q rejected: %v", name, err)
		}
	}
	for _, name := range []string{"if", "for", "func", "return", "_", "名"} {
		left := Identifier{Name: name, Type: TypeInt}
		if _, err := BuildTypedBinary("add", left, Identifier{Name: "right", Type: TypeInt}); err == nil {
			t.Errorf("invalid Go identifier %q accepted by BuildTypedBinary", name)
		}
		invalidIR := TypedBinaryIR{Schema: TypedBinaryIRSchema, Kind: "binary", Operation: "add", Left: left, Right: Identifier{Name: "right", Type: TypeInt}, ResultType: TypeInt}
		if _, err := AssembleGoExpression(invalidIR); err == nil {
			t.Errorf("invalid Go identifier %q accepted by AssembleGoExpression", name)
		}
	}
}

func TestAssembleGoExpressionUsesClosedOperatorsAndValidatedIR(t *testing.T) {
	tests := []struct {
		operation string
		left      Identifier
		right     Identifier
		want      string
	}{
		{"add", Identifier{"left", TypeInt}, Identifier{"right", TypeInt}, "left + right"},
		{"subtract", Identifier{"left", TypeInt}, Identifier{"right", TypeInt}, "left - right"},
		{"multiply", Identifier{"left", TypeInt}, Identifier{"right", TypeInt}, "left * right"},
		{"less_than", Identifier{"left", TypeInt}, Identifier{"right", TypeInt}, "left < right"},
		{"less_equal", Identifier{"left", TypeInt}, Identifier{"right", TypeInt}, "left <= right"},
		{"equal", Identifier{"left", TypeBool}, Identifier{"right", TypeBool}, "left == right"},
		{"and", Identifier{"left", TypeBool}, Identifier{"right", TypeBool}, "left && right"},
		{"or", Identifier{"left", TypeBool}, Identifier{"right", TypeBool}, "left || right"},
	}
	for _, test := range tests {
		ir, err := BuildTypedBinary(test.operation, test.left, test.right)
		if err != nil {
			t.Fatal(err)
		}
		got, err := AssembleGoExpression(ir)
		if err != nil || got != test.want {
			t.Fatalf("AssembleGoExpression(%+v) = %q, %v; want %q", ir, got, err, test.want)
		}
	}
	invalid := TypedBinaryIR{Schema: TypedBinaryIRSchema, Kind: "binary", Operation: "add", Left: Identifier{"x); panic(1)", TypeInt}, Right: Identifier{"y", TypeInt}, ResultType: TypeInt}
	if _, err := AssembleGoExpression(invalid); err == nil {
		t.Fatal("source-like identifier was assembled")
	}
}

func TestDecideReturnsOnlyEnumAndTypedIROrAbstains(t *testing.T) {
	metadataPath, _ := writeFixtureModel(t, "fp32", 0.5)
	model, err := Load(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	var workspace Workspace
	request := DecisionRequest{Schema: DecisionRequestSchema, Text: "add the left value and right value",
		Left: Identifier{Name: "left", Type: TypeInt}, Right: Identifier{Name: "right", Type: TypeInt}}
	response, err := model.Decide(request, &workspace)
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != "decision" || response.TypedBinaryIR == nil || response.TypedBinaryIR.Operation != "add" {
		t.Fatalf("expected typed add decision, got %+v", response)
	}
	if response.WeightsSHA256 != model.WeightsSHA256() || len(response.Probabilities) != LabelCount {
		t.Fatalf("model provenance/probabilities missing: %+v", response)
	}
	for i, label := range Labels() {
		if response.Probabilities[i].Label != label {
			t.Fatalf("probability[%d] label = %q, want %q", i, response.Probabilities[i].Label, label)
		}
	}

	request.Left.Type, request.Right.Type = TypeBool, TypeBool
	response, err = model.Decide(request, &workspace)
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != "abstained" || response.AbstainReason != "TYPE_MISMATCH" || response.TypedBinaryIR != nil {
		t.Fatalf("type-incompatible top enum was not rejected: %+v", response)
	}
}

func TestDecideLowConfidenceAbstains(t *testing.T) {
	metadataPath, _ := writeFixtureModel(t, "fp32", 0.99)
	model, err := Load(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	var workspace Workspace
	response, err := model.Decide(DecisionRequest{
		Schema: DecisionRequestSchema,
		Text:   "add values",
		Left:   Identifier{Name: "left", Type: TypeInt},
		Right:  Identifier{Name: "right", Type: TypeInt},
	}, &workspace)
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != "abstained" || response.AbstainReason != "LOW_CONFIDENCE" || response.TypedBinaryIR != nil {
		t.Fatalf("low-confidence result did not abstain: %+v", response)
	}
}
