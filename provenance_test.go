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
	if hash(raw) != "03cc7f2116dce620d530388e44e509219a2c160200f77098329ed369405ecf8c" {
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
	if json.Unmarshal(raw, &manifest) != nil || manifest.Schema != "gooo/decision-runtime-source-provenance/v2" || manifest.Module != "github.com/kimjooyoon/gooo-decision-runtime" || manifest.Origin != "809f98266d87a2608bc335588625b760fbcc6c96" || len(manifest.Files) != 16 {
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
