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
	if hash(raw) != "88a2268e0254ee861db3ec38e289d890d2fa2ea19f97a13177b7b7412f7ebc00" {
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
	if json.Unmarshal(raw, &manifest) != nil || manifest.Schema != "gooo/decision-runtime-source-provenance/v2" || manifest.Module != "github.com/kimjooyoon/gooo-decision-runtime" || manifest.Origin != "ddcd1002b14996f4e6b5577a227e065441a407c5" || len(manifest.Files) != 69 {
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
