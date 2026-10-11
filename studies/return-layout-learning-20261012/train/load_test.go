package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

// Synthetic loader regressions execute neither fitting nor inference.
func TestFrozenLayoutLoaderRejectsSplitAndSourceMutation(t *testing.T) {
	rows := make([]record, 100)
	for i := range rows {
		r := &rows[i]
		r.Group, r.Parity = i/4, i%4
		r.ID = fmt.Sprintf("layout-%02d-%d", r.Group, r.Parity)
		r.Split = "train"
		if r.Group >= 20 {
			r.Split = "heldout"
		} else if r.Group >= 15 {
			r.Split = "validation"
		}
		r.Gooo = r.ID
		r.SourceSHA = fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(r.Gooo)))
		r.Acceptable = uint64(1) << r.Parity
		r.Inputs = make([][528]float32, 2)
		r.Outputs, r.Conditions = make([][32]float32, 6), make([][32]float32, 3)
		r.Probes = make([]pathplan.TestCase, 8)
		for mask := range 4 {
			r.Candidates = append(r.Candidates, candidate{Mask: uint16(mask),
				Outputs: make([]pathplan.TestResult, 6), Conditions: make([]pathplan.ConditionResult, 3),
				ProbeResults: make([]pathplan.TestResult, 8), Acceptable: mask == r.Parity})
		}
	}
	name := filepath.Join(t.TempDir(), "corpus.json")
	write := func() {
		raw, err := json.Marshal(rows)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(name, raw, 0644); err != nil {
			t.Fatal(err)
		}
	}
	write()
	if _, _, err := loadCorpus(name); err != nil {
		t.Fatal(err)
	}
	rows[60].Split = "train"
	write()
	if _, _, err := loadCorpus(name); err == nil {
		t.Fatal("validation entered training")
	}
	rows[60].Split = "validation"
	rows[0].Gooo = "changed"
	write()
	if _, _, err := loadCorpus(name); err == nil {
		t.Fatal("unbound source accepted")
	}
}
