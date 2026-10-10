package decision_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestConditionEvolutionRetainsExactPreviousExtractionManifest(t *testing.T) {
	type entry struct {
		Path  string `json:"copied_path"`
		SHA   string `json:"sha256"`
		Bytes int    `json:"bytes"`
	}
	type manifest struct {
		Files  []entry `json:"files"`
		Origin string  `json:"origin_revision"`
	}
	read := func(path string) []byte {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	hash := func(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
	oldRaw, newRaw := read("source-provenance-v0.2.26.json"), read("source-provenance.json")
	if hash(oldRaw) != "88a2268e0254ee861db3ec38e289d890d2fa2ea19f97a13177b7b7412f7ebc00" {
		t.Fatal("historical extraction manifest changed")
	}
	var old, current manifest
	if json.Unmarshal(oldRaw, &old) != nil || json.Unmarshal(newRaw, &current) != nil || len(old.Files) != 69 ||
		len(current.Files) != 69 || old.Origin != current.Origin {
		t.Fatal("extraction history changed")
	}
	var evolution struct {
		Base   string `json:"base_revision"`
		Before string `json:"previous_manifest_sha256"`
		After  string `json:"updated_manifest_sha256"`
		Files  []struct {
			Path        string `json:"path"`
			Before      string `json:"before_sha256"`
			After       string `json:"after_sha256"`
			BeforeBytes int    `json:"before_bytes"`
			AfterBytes  int    `json:"after_bytes"`
		} `json:"changed_extracted_files"`
	}
	if json.Unmarshal(read("source-evolution-condition-cases.json"), &evolution) != nil ||
		evolution.Base != "d7d92dea081de1a44a606e6b426c318ba1ec5820" || evolution.Before != hash(oldRaw) || evolution.After != hash(newRaw) {
		t.Fatal("source evolution does not bind its exact predecessor")
	}
	changes := map[string]bool{}
	for _, change := range evolution.Files {
		if changes[change.Path] {
			t.Fatal("duplicate source evolution")
		}
		changes[change.Path] = true
		found := false
		for i, prior := range old.Files {
			if prior.Path != change.Path {
				continue
			}
			next := current.Files[i]
			if next.Path != prior.Path || prior.SHA != change.Before || next.SHA != change.After ||
				prior.Bytes != change.BeforeBytes || next.Bytes != change.AfterBytes {
				t.Fatal("unbound source delta", change.Path)
			}
			found = true
		}
		if !found {
			t.Fatal("evolution names unextracted source")
		}
	}
	for i, prior := range old.Files {
		if current.Files[i].Path != prior.Path || (prior.SHA != current.Files[i].SHA) != changes[prior.Path] {
			t.Fatal("unrecorded extraction change", prior.Path)
		}
	}
}
