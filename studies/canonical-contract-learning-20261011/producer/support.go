package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-contract-learning-20261011/record"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func require(ok bool, reason string) {
	if !ok {
		panic(reason)
	}
}
func newRows(root, name string) *os.File {
	f, err := os.OpenFile(filepath.Join(root, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	must(err)
	return f
}
func save(root, name string, value any) {
	f := newRows(root, name)
	_, err := f.Write(r.Encode(value))
	must(err)
	must(f.Close())
}
func fileSHA(path string) string { raw, err := os.ReadFile(path); must(err); return r.Hash(raw) }
func identity() string {
	info, ok := debug.ReadBuildInfo()
	require(ok && info.GoVersion == "go1.27.2", "Go1.27.2 producer required")
	rev, clean := "", false
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" {
			rev = setting.Value
		}
		if setting.Key == "vcs.modified" {
			clean = setting.Value == "false"
		}
	}
	require(clean && len(rev) == 40, "clean committed producer required")
	return rev
}
func frozenSources(path string) []r.Frozen {
	raw, err := os.ReadFile(path)
	must(err)
	require(r.Hash(raw) == r.CorpusSHA, "frozen source digest")
	z, err := gzip.NewReader(bytes.NewReader(raw))
	must(err)
	defer z.Close()
	d := json.NewDecoder(io.LimitReader(z, 8<<20))
	var rows []r.Frozen
	for {
		var row r.Frozen
		err := d.Decode(&row)
		if err == io.EOF {
			break
		}
		must(err)
		rows = append(rows, row)
	}
	specs := r.Specs()
	require(len(rows) == len(specs) && len(rows) == 320, "frozen row count")
	for i, row := range rows {
		require(row.Spec == specs[i] && row.Gooo == r.Gooo(specs[i]) && reflect.DeepEqual(row.Probes, r.Probes(specs[i])) && reflect.DeepEqual(row.ProbeConditions, r.ProbeConditions(specs[i])), "frozen source and independent probes")
	}
	return rows
}

type trainingRecord struct {
	ID, PlanSHA, CasesSHA         string
	LegacyInputs, CanonicalInputs [][528]float32
	Outputs, Conditions           [][32]float32
	Acceptable                    uint64
}

type artifact interface {
	Marshal() ([]byte, error)
	Fingerprint() string
	ArtifactSchema() string
}

func recordModel(root, name string, model artifact, history []contractdecision.Epoch, ns int64, report *r.Report) []byte {
	raw, err := model.Marshal()
	must(err)
	f := newRows(root, "model-"+name+".json")
	_, err = f.Write(raw)
	must(err)
	must(f.Close())
	stored, err := os.ReadFile(filepath.Join(root, "model-"+name+".json"))
	must(err)
	require(bytes.Equal(raw, stored), "saved artifact differs")
	save(root, "history-"+name+".json", history)
	report.Models[name] = r.Model{SHA: r.Hash(raw), Fingerprint: model.Fingerprint(), Schema: model.ArtifactSchema(), Parameters: contractdecision.InteractionRequirementParameterCount,
		WeightBytes: 4 * contractdecision.InteractionRequirementParameterCount, ArtifactBytes: len(raw), FitNS: ns, FirstLoss: history[0].Loss, LastLoss: history[len(history)-1].Loss}
	return stored
}
