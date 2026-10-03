package decision_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestWholeCandidateSourceMatchesPinnedOrigin(t *testing.T) {
	raw, err := os.ReadFile("source-provenance-order-judge-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	digest := func(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }
	if digest(raw) != "410778ab5059c8fbe2cffcc30d316a9f3a818795ba157ecc205f1b550c90a859" {
		t.Fatal("candidate extraction manifest changed")
	}
	var m struct {
		Schema string `json:"schema"`
		Origin string `json:"origin_revision"`
		Files  []struct {
			Path  string `json:"copied_path"`
			SHA   string `json:"sha256"`
			Bytes int    `json:"bytes"`
		} `json:"files"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m.Schema != "gooo/order-judge-sdk-source/v1" || m.Origin != "ada8a3fd728a9288b5d7060982bcb3801e8bccfa" || len(m.Files) != 12 {
		t.Fatal("candidate extraction identity differs")
	}
	seen := map[string]bool{}
	for _, f := range m.Files {
		if seen[f.Path] {
			t.Fatal("duplicate candidate file")
		}
		seen[f.Path] = true
		b, err := os.ReadFile(f.Path)
		if err != nil {
			t.Fatal(err)
		}
		if len(b) != f.Bytes || digest(b) != f.SHA {
			t.Fatalf("candidate copied bytes differ: %s", f.Path)
		}
	}
}
