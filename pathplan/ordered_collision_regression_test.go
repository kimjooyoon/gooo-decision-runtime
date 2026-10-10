package pathplan

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
)

// Reuse stored source documents as regression fixtures. No training, prediction,
// candidate search, outcome evaluation or rewriting of the study is performed.
func TestOrderedContextSeparatesFortyRecordedInputCollisions(t *testing.T) {
	const root = "../studies/requirement-learning-20261010/result/"
	raw, err := os.ReadFile(root + "audit.json")
	if err != nil {
		t.Fatal(err)
	}
	var audit struct{ InputCollisions []struct{ IDs [2]string } }
	if err := json.Unmarshal(raw, &audit); err != nil || len(audit.InputCollisions) != 40 {
		t.Fatal("frozen collision inventory", err)
	}
	file, err := os.Open(root + "sources.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	z, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	documents := make(map[string]Document)
	decoder := json.NewDecoder(z)
	for {
		var row struct {
			Spec     struct{ ID string }
			Document Document
		}
		if err := decoder.Decode(&row); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		documents[row.Spec.ID] = row.Document
	}
	for _, collision := range audit.InputCollisions {
		var old [2][2][384]float32
		var ordered [2][decision.OrderedExpressionFeatureDim]float32
		for i, id := range collision.IDs {
			doc := documents[id]
			p, err := doc.Prepare()
			if err != nil || len(doc.Plan.Decisions) != 2 {
				t.Fatal(id, err)
			}
			input, err := p.InitialContractInput(doc.TestCases)
			if err != nil {
				t.Fatal(err)
			}
			for j, choice := range doc.Plan.Decisions {
				if err := input.RelationalSourceFeaturesInto(choice.ID, &old[i][j]); err != nil {
					t.Fatal(err)
				}
			}
			view, err := p.OrderedBranchContext(doc.Plan.Decisions[0].ID)
			if err != nil || !view.Available {
				t.Fatal(id, view, err)
			}
			if err := decision.OrderedExpressionFeaturesInto(view.Expressions, &ordered[i]); err != nil {
				t.Fatal(err)
			}
		}
		if old[0] != old[1] || ordered[0] == ordered[1] {
			t.Fatal("recorded collision was altered or remains indistinguishable", collision.IDs)
		}
	}
	t.Log("40/40 recorded source-input collisions distinguished; model accuracy unmeasured")
}
