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
	if hash(raw) != "1f98dd3a532c66e70eb87744aa432da4777a3cd9665cdc4309ca80aab50d0123" {
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
	if json.Unmarshal(raw, &manifest) != nil || manifest.Schema != "gooo/decision-runtime-source-provenance/v2" || manifest.Module != "github.com/kimjooyoon/gooo-decision-runtime" || manifest.Origin != "8d62b89a2b119f9537cc58504c4f860e7773ae76" || len(manifest.Files) != 18 {
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
