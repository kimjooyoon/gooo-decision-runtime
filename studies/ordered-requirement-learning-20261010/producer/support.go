package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/ordered-requirement-learning-20261010/record"
)

type Report = r.Report
type ModelReport = r.ModelReport
type modelArtifact interface {
	Marshal() ([]byte, error)
	Fingerprint() string
	ArtifactSchema() string
}

func recordModel(root, name string, model modelArtifact, history []contractdecision.Epoch, ns int64, parameters int, report *Report) {
	raw, err := model.Marshal()
	must(err)
	f := newRows(root, "model-"+name+".json")
	_, err = f.Write(raw)
	must(err)
	must(f.Close())
	save(root, "history-"+name+".json", history)
	report.Models[name] = ModelReport{SHA: r.Hash(raw), Fingerprint: model.Fingerprint(), Schema: model.ArtifactSchema(), Parameters: parameters, WeightBytes: parameters * 4, ArtifactBytes: len(raw), TrainingNS: ns, FirstLoss: history[0].Loss, LastLoss: history[len(history)-1].Loss}
}
func frozenSources(path string) []r.FrozenSource {
	raw, err := os.ReadFile(path)
	must(err)
	require(r.Hash(raw) == r.CorpusSHA, "frozen corpus compressed digest")
	z, err := gzip.NewReader(bytes.NewReader(raw))
	must(err)
	defer z.Close()
	d := json.NewDecoder(io.LimitReader(z, 4<<20))
	var result []r.FrozenSource
	for {
		var row r.FrozenSource
		err := d.Decode(&row)
		if err == io.EOF {
			break
		}
		must(err)
		result = append(result, row)
	}
	require(len(result) == 48, "complete frozen corpus")
	return result
}
