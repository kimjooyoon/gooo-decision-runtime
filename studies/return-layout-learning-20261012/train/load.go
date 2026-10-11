package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
)

func loadCorpus(name string) ([]record, string, error) {
	raw, err := os.ReadFile(name)
	if err != nil {
		return nil, "", err
	}
	if len(raw) > 8<<20 {
		return nil, "", fmt.Errorf("corpus exceeds 8 MiB")
	}
	var rows []record
	if err = json.Unmarshal(raw, &rows); err != nil {
		return nil, "", err
	}
	if len(rows) != 100 {
		return nil, "", fmt.Errorf("100 frozen source documents required")
	}
	for i, r := range rows {
		split := "train"
		if r.Group >= 20 {
			split = "heldout"
		} else if r.Group >= 15 {
			split = "validation"
		}
		if r.Group != i/4 || r.Parity != i%4 || r.Split != split ||
			r.ID != fmt.Sprintf("layout-%02d-%d", i/4, i%4) ||
			r.SourceSHA != fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(r.Gooo))) ||
			r.Acceptable != uint64(1)<<r.Parity || len(r.Inputs) != 2 ||
			len(r.Outputs) != 6 || len(r.Conditions) != 3 || len(r.Candidates) != 4 || len(r.Probes) != 8 {
			return nil, "", fmt.Errorf("frozen source/split/label mismatch at %d", i)
		}
		for j, c := range r.Candidates {
			if int(c.Mask) != j || len(c.Outputs) != 6 || len(c.Conditions) != 3 || len(c.ProbeResults) != 8 ||
				c.Acceptable != (r.Acceptable>>j&1 != 0) {
				return nil, "", fmt.Errorf("candidate observation mismatch at %d/%d", i, j)
			}
		}
	}
	return rows, fmt.Sprintf("sha256:%x", sha256.Sum256(raw)), nil
}
