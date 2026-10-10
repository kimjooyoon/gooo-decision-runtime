package record

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/contract-goals-20261010/record"
)

const SourceSHA = "773db4e279b497465a92ad2d7df683f0a2898d72365f6f48ac351e5f7036b413"

type ModelSpec struct{ Mode, Study, SHA, SavedSHA string }

var Models = []ModelSpec{
	{"mean", "contract-goals-20261010", "c53ef0386dfc8ee2422b2b9605b609b0c23315905599ecc2f4555c26de514b5f", "8141bacac8e4af8156ceb1c473294be4bf869d5d887df42b61cc962e3277eff6"},
	{"extreme", "contract-pooling-20261010", "f69944394dbe66b3449c5f87cb5ee51b09a9f34c4a2483e5fc119a005160e8b7", "ccdf0b3312c123d7b4c3bc804e9443d43489f38ac024578af7bbb85f25e2b827"},
}

type Row struct {
	ID, Mode, SourceSHA, PlanSHA, CaseSHA, ModelSHA, Fingerprint string
	InputSHA                                                     [2]string
	Prediction                                                   contractdecision.Prediction
	Trace                                                        contractdecision.Explanation
	NS                                                           int64
}

type Pair struct {
	ID, Mode, Split                                        string
	Sources                                                [2]string
	PoolEqual, SourceEqual, JointEqual, HiddenEqual        bool
	OptionsEqual, SameProposal, BothValid, BothInvalid     bool
	CrossedUnits, TotalPairedUnits, SourceDominated, Units int
	Margins                                                [2][2]float64
}

type Group struct {
	Pairs, PoolEqual, SourceEqual, JointEqual, HiddenEqual int
	OptionsEqual, SameProposal, BothValid, BothInvalid     int
	CrossedUnits, TotalPairedUnits, SourceDominated, Units int
}

type Summary struct {
	Groups map[string]Group
	Pairs  []Pair
}

func Need(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
func Must(err error) {
	if err != nil {
		panic(err)
	}
}
func Read(root, name, digest string) []byte {
	raw, err := os.ReadFile(filepath.Join(root, name))
	Must(err)
	Need(digest == "" || r.Hash(raw) == digest, "pinned digest "+name)
	if filepath.Ext(name) != ".gz" {
		return raw
	}
	z, err := gzip.NewReader(bytes.NewReader(raw))
	Must(err)
	defer z.Close()
	b, err := io.ReadAll(io.LimitReader(z, 32<<20))
	Must(err)
	return b
}
func Lines[T any](raw []byte) []T {
	d := json.NewDecoder(bytes.NewReader(raw))
	var rows []T
	for {
		var row T
		err := d.Decode(&row)
		if err == io.EOF {
			return rows
		}
		Must(err)
		rows = append(rows, row)
	}
}

func Sources(studies string) []r.Source {
	rows := Lines[r.Source](Read(filepath.Join(studies, "contract-goals-20261010/result"), "sources.jsonl.gz", SourceSHA))
	Need(len(rows) == 196, "all196 sources")
	return rows
}

func Saved(studies string, spec ModelSpec) map[string]r.Search {
	rows := Lines[r.Search](Read(filepath.Join(studies, spec.Study, "result"), "searches.jsonl.gz", spec.SavedSHA))
	result := map[string]r.Search{}
	for _, row := range rows {
		if row.Mode == "deterministic" {
			continue
		}
		Need(result[row.ID].ID == "", "duplicate saved source")
		result[row.ID] = row
	}
	Need(len(result) == 196, fmt.Sprint("all saved ", spec.Mode))
	return result
}
