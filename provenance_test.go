package decision_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestProductionSourceMatchesPinnedOrigin(t *testing.T) {
	raw, err := os.ReadFile("source-provenance.json")
	if err != nil {
		t.Fatal(err)
	}
	hash := func(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
	if hash(raw) != "230c72560143464a2df2cbf3ddf124123af9aff36e1d71faf5d0e489c18e4cc2" {
		t.Fatal("fixed source extraction manifest changed")
	}
	var manifest struct {
		Schema string `json:"schema"`
		Module string `json:"module"`
		Origin string `json:"origin_revision"`
		Files  []struct {
			Path  string `json:"copied_path"`
			SHA   string `json:"sha256"`
			Bytes int    `json:"bytes"`
		} `json:"files"`
	}
	if json.Unmarshal(raw, &manifest) != nil || manifest.Schema != "gooo/decision-runtime-source-provenance/v2" || manifest.Module != "github.com/kimjooyoon/gooo-decision-runtime" || manifest.Origin != "07fec50503f9ec189ff46bab6df60ff4e6234ace" || len(manifest.Files) != 42 {
		t.Fatal("source extraction identity mismatch")
	}
	seen := map[string]bool{}
	for _, file := range manifest.Files {
		if seen[file.Path] {
			t.Fatal("duplicate source entry")
		}
		seen[file.Path] = true
		data, err := os.ReadFile(file.Path)
		if err != nil {
			t.Fatal(err)
		}
		if len(data) != file.Bytes || hash(data) != file.SHA {
			t.Fatalf("copied bytes changed: %s", file.Path)
		}
	}
}
