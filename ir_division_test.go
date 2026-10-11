package decision

import "testing"

func TestDivisionTypedIRPreservesClassifierLabels(t *testing.T) {
	ir, err := BuildTypedBinary("divide", Identifier{Name: "left", Type: TypeInt}, Identifier{Name: "right", Type: TypeInt})
	if err != nil {
		t.Fatal(err)
	}
	text, err := AssembleGoExpression(ir)
	if err != nil || text != "left / right" || ir.ResultType != TypeInt {
		t.Fatal(text, err, ir)
	}
	for _, types := range [][2]ValueType{{TypeBool, TypeInt}, {TypeInt, TypeBool}, {TypeBool, TypeBool}} {
		if _, err := BuildTypedBinary("divide", Identifier{Name: "left", Type: types[0]}, Identifier{Name: "right", Type: types[1]}); err == nil {
			t.Fatal("Boolean division accepted")
		}
	}
	if LabelCount != 8 {
		t.Fatal("source operation changed the existing classifier output ABI")
	}
}
