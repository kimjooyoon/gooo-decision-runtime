// Audit reads saved records only. It never fits, predicts or runs a candidate.
package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	r "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-pooling-20261011/record"
)

func require(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
func plain(name string) []byte { b, err := os.ReadFile(name); must(err); return b }
func inflated(name string) []byte {
	z, err := gzip.NewReader(bytes.NewReader(plain(name)))
	must(err)
	defer z.Close()
	b, err := io.ReadAll(io.LimitReader(z, 64<<20))
	must(err)
	return b
}
func decode[T any](b []byte) T {
	var value T
	must(json.Unmarshal(b, &value))
	return value
}
func rows[T any](b []byte) []T {
	d := json.NewDecoder(bytes.NewReader(b))
	var result []T
	for {
		var value T
		err := d.Decode(&value)
		if err == io.EOF {
			return result
		}
		must(err)
		result = append(result, value)
	}
}

type Delta struct {
	Documents      int
	ImprovementIDs []string
	RegressionIDs  []string
}
type Timing struct {
	Observations int
	MedianNS     float64
	P95NS        int64
}
type Summary struct {
	Schema                           string
	Documents, Fits, Sessions, Calls int
	Groups                           map[string]r.Group
	ExtremeComparedWithMean          map[string]Delta
	EquivalentFormPairs              map[string]int
	Prediction                       map[string]Timing
}

func audit(root string) (summary Summary, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("saved-record audit: %v", p)
		}
	}()
	report := decode[r.Report](inflated(filepath.Join(root, "report.json.gz")))
	require(report.Schema == "gooo/canonical-pooling-study/v1" && report.Producer == "706c590ba188afed5992fba07d313dbd6bd3de09", "producer identity")
	require(report.Compiler == r.Compiler && report.SDKBase == r.SDKBase && report.CorpusSHA == r.CorpusSHA, "source identity")
	require(report.Sources == 256 && report.TrainingRows == 64 && report.Fits == 2 && report.InitialSearches == 768 && report.ModelCalls == 512, "original counts")
	require(string(plain(filepath.Join(root, "original.exit"))) == "0\n", "original process failed")
	started := decode[map[string]string](plain(filepath.Join(root, "started.json")))
	completed := decode[map[string]string](plain(filepath.Join(root, "completed.json")))
	require(started["producer"] == report.Producer && completed["producer"] == report.Producer &&
		started["compiler"] == report.Compiler && started["sdk_base"] == report.SDKBase && started["corpus"] == report.CorpusSHA, "execution identity")
	start, err := time.Parse(time.RFC3339Nano, started["time"])
	must(err)
	finish, err := time.Parse(time.RFC3339Nano, completed["time"])
	must(err)
	require(finish.After(start), "execution interval")
	corpusRaw := plain(filepath.Join(root, "..", "corpus.jsonl.gz"))
	require(r.Hash(corpusRaw) == r.CorpusSHA, "frozen corpus digest")
	frozen := rows[r.Frozen](inflated(filepath.Join(root, "..", "corpus.jsonl.gz")))
	sourceRaw := inflated(filepath.Join(root, "sources.jsonl.gz"))
	searchRaw := inflated(filepath.Join(root, "searches.jsonl.gz"))
	require(r.Hash(sourceRaw) == report.SourcesSHA && r.Hash(searchRaw) == report.SearchesSHA, "original row digests")
	sources := rows[r.Source](sourceRaw)
	searches := rows[r.Search](searchRaw)
	validateTraining(root, report, sources)
	summary = summarize(report, frozen, sources, searches)
	return summary, nil
}

func main() {
	root := "studies/canonical-pooling-20261011/result"
	if len(os.Args) == 2 {
		root = os.Args[1]
	}
	require(len(os.Args) <= 2, "usage: audit [RESULT_DIRECTORY]")
	summary, err := audit(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_, err = os.Stdout.Write(r.Encode(summary))
	must(err)
}
