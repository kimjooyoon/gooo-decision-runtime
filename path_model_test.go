package decision

import (
	"encoding/json"
	"os"
	"testing"
)

func TestPathAndOperationModelContractsAreDistinct(t *testing.T) {
	for _, variant := range []string{"fp32", "ptq_ternary", "qat_ternary"} {
		path, _ := writeFixtureModel(t, variant, 0.5)
		if _, err := LoadPath(path); err == nil {
			t.Fatal("operation bundle accepted by path loader")
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var metadata Metadata
		if err := json.Unmarshal(raw, &metadata); err != nil {
			t.Fatal(err)
		}
		metadata.Schema = PathMetadataSchema
		labels := PathLabels()
		metadata.Labels = append([]string(nil), labels[:]...)
		raw, err = json.Marshal(metadata)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatal("path bundle accepted by operation loader")
		}
		model, err := LoadPath(path)
		if err != nil {
			t.Fatal(err)
		}
		var workspace Workspace
		var output Prediction
		if err := model.PredictInto("Choose the first variable.", &workspace, &output); err != nil {
			t.Fatal(err)
		}
		if model.PredictLabel(&output) != "reference_first" || model.Schema() != PathMetadataSchema {
			t.Fatal("wrong path model label")
		}
		if _, err := model.Decide(DecisionRequest{Schema: DecisionRequestSchema, Text: "Use a local.", Left: Identifier{Name: "a", Type: TypeInt}, Right: Identifier{Name: "b", Type: TypeInt}}, &workspace); err == nil {
			t.Fatal("path model accepted by binary operation bridge")
		}
	}
}
