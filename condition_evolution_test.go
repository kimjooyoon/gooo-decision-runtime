package decision_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestConditionEvolutionRetainsExactPreviousExtractionManifest(t *testing.T) {
	for _, edge := range []struct {
		name, before, after, evolution, base, beforeSHA string
	}{
		{"condition-cases", "source-provenance-v0.2.26.json", "source-provenance-v0.2.27.json",
			"source-evolution-condition-cases.json", "d7d92dea081de1a44a606e6b426c318ba1ec5820",
			"88a2268e0254ee861db3ec38e289d890d2fa2ea19f97a13177b7b7412f7ebc00"},
		{"condition-feedback", "source-provenance-v0.2.27.json", "source-provenance.json",
			"source-evolution-condition-feedback.json", "c1eec1d5688076eb216f8d7612b11aa4c55cec9f",
			"b50eceb5755896f3a5b4f81f71f97e49fd31d26f4acd83e224465ca166eae084"},
	} {
		t.Run(edge.name, func(t *testing.T) {
			checkConditionEvolution(t, edge.before, edge.after, edge.evolution, edge.base, edge.beforeSHA)
		})
	}
}

func checkConditionEvolution(t *testing.T, before, after, record, base, beforeSHA string) {
	t.Helper()
	type entry struct {
		Path        string `json:"copied_path"`
		SHA         string `json:"sha256"`
		Bytes       int    `json:"bytes"`
		OriginPath  string `json:"origin_path"`
		OriginSHA   string `json:"origin_sha256"`
		OriginBytes int    `json:"origin_bytes"`
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
	oldRaw, newRaw := read(before), read(after)
	if hash(oldRaw) != beforeSHA {
		t.Fatal("historical extraction manifest changed")
	}
	var old, current manifest
	if json.Unmarshal(oldRaw, &old) != nil || json.Unmarshal(newRaw, &current) != nil || len(old.Files) != 69 ||
		len(current.Files) != 69 || old.Origin != current.Origin {
		t.Fatal("extraction history changed")
	}
	var evolution struct {
		Schema   string `json:"schema"`
		Previous string `json:"previous_manifest"`
		Base     string `json:"base_revision"`
		Before   string `json:"previous_manifest_sha256"`
		After    string `json:"updated_manifest_sha256"`
		Files    []struct {
			Path        string `json:"path"`
			Before      string `json:"before_sha256"`
			After       string `json:"after_sha256"`
			BeforeBytes int    `json:"before_bytes"`
			AfterBytes  int    `json:"after_bytes"`
		} `json:"changed_extracted_files"`
	}
	if json.Unmarshal(read(record), &evolution) != nil || evolution.Schema != "gooo/decision-runtime-source-evolution/v1" ||
		evolution.Previous != before || evolution.Base != base || evolution.Before != hash(oldRaw) || evolution.After != hash(newRaw) {
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
		next := current.Files[i]
		if next.Path != prior.Path || next.OriginPath != prior.OriginPath || next.OriginSHA != prior.OriginSHA ||
			next.OriginBytes != prior.OriginBytes || (prior.SHA != next.SHA) != changes[prior.Path] ||
			(!changes[prior.Path] && next.Bytes != prior.Bytes) {
			t.Fatal("unrecorded extraction change", prior.Path)
		}
	}
}
