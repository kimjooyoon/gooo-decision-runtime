// Recount the saved collision probe without inference or program execution.
package main

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"

	"github.com/kimjooyoon/gooo-decision-runtime/conditiondecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

type row struct {
	ID         string            `json:"id"`
	Source     string            `json:"gooo_source"`
	SourceSHA  string            `json:"source_sha256"`
	Document   pathplan.Document `json:"document"`
	Features   [256]float32      `json:"features"`
	FeatureSHA string            `json:"feature_sha256"`
	Candidates []struct {
		Mask   uint16                `json:"mask"`
		Cases  []pathplan.TestResult `json:"cases"`
		Passed int                   `json:"passed"`
	} `json:"candidates"`
	Valid            []uint16                     `json:"valid_masks"`
	Prediction       conditiondecision.Prediction `json:"prediction"`
	PredictNS        int64                        `json:"predict_ns"`
	PredictionPassed bool                         `json:"prediction_passed"`
}

func require(ok bool) {
	if !ok {
		panic("saved observation inconsistent")
	}
}
func hash(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }

func main() {
	require(len(os.Args) == 2)
	f, err := os.Open(filepath.Join(os.Args[1], "report.json.gz"))
	require(err == nil)
	defer f.Close()
	z, err := gzip.NewReader(f)
	require(err == nil)
	defer z.Close()
	var report struct {
		ModelSHA string `json:"model_sha256"`
		Calls    int    `json:"model_calls"`
		Native   int    `json:"native_runs"`
		Rows     []row  `json:"rows"`
		Same     bool   `json:"identical_initial_features"`
		Disjoint bool   `json:"disjoint_valid_masks"`
	}
	d := json.NewDecoder(z)
	d.UseNumber()
	require(d.Decode(&report) == nil)
	var extra any
	require(d.Decode(&extra) == io.EOF)
	require(report.ModelSHA == "a16696ed44c668f38cc2e7ee1dc6ff3f4716649d57e59df7c9490d1c670edc48")
	require(report.Calls == 2 && report.Native == 0 && len(report.Rows) == 2 && report.Same && report.Disjoint)
	require(report.Rows[0].Features == report.Rows[1].Features)
	require(report.Rows[0].Prediction == report.Rows[1].Prediction)
	require(report.Rows[0].ID == "max-forward" && report.Rows[1].ID == "max-reversed")
	passedPredictions := 0
	for i, r := range report.Rows {
		require(hash([]byte(r.Source)) == r.SourceSHA && r.PredictNS > 0)
		var b [1024]byte
		for j, x := range r.Features {
			binary.LittleEndian.PutUint32(b[j*4:], math.Float32bits(x))
		}
		require(hash(b[:]) == r.FeatureSHA)
		for _, x := range r.Features[192:] {
			require(x == 0)
		}
		require(len(r.Candidates) == 2 && len(r.Document.TestCases) == 6)
		var valid []uint16
		for j, c := range r.Candidates {
			require(int(c.Mask) == j && len(c.Cases) == 6)
			passed := 0
			for k, test := range c.Cases {
				declared := r.Document.TestCases[k]
				require(test.Input == declared.Input && test.Expected == declared.Expected)
				require(test.Expected == max(test.Input, 10) && test.Passed == (test.Actual == test.Expected))
				if test.Passed {
					passed++
				}
			}
			require(passed == c.Passed)
			if passed == 6 {
				valid = append(valid, c.Mask)
			}
		}
		require(slices.Equal(valid, r.Valid) && slices.Equal(valid, []uint16{uint16(i)}))
		require(r.PredictionPassed == slices.Contains(valid, r.Prediction.Selected))
		if r.PredictionPassed {
			passedPredictions++
		}
	}
	require(passedPredictions == 1)
	fmt.Println("PASS: identical 1024-byte initial features, disjoint valid masks {0}/{1}, identical predictions; 1/2 proposals satisfy six finite cases. New model calls: 0; new program executions: 0.")
}
