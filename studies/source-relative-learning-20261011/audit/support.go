package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	r "github.com/kimjooyoon/gooo-decision-runtime/studies/source-relative-learning-20261011/record"
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
func payload(root, name string) []byte {
	raw, err := os.ReadFile(filepath.Join(root, name))
	if err == nil {
		return raw
	}
	require(errors.Is(err, os.ErrNotExist), "cannot read retained payload")
	packed, err := os.ReadFile(filepath.Join(root, name+".gz"))
	must(err)
	z, err := gzip.NewReader(bytes.NewReader(packed))
	must(err)
	defer z.Close()
	raw, err = io.ReadAll(z)
	must(err)
	return raw
}
func read[T any](root, name string) T {
	raw := payload(root, name)
	var out T
	must(json.Unmarshal(raw, &out))
	return out
}
func lines[T any](root, name string) []T {
	d := json.NewDecoder(bytes.NewReader(payload(root, name)))
	var out []T
	for {
		var row T
		err := d.Decode(&row)
		if err == io.EOF {
			break
		}
		must(err)
		out = append(out, row)
	}
	return out
}
func hash(root, name string) string {
	return fmt.Sprintf("%x", sha256.Sum256(payload(root, name)))
}
func order(scores [2][2]float32) [4]uint16 {
	x := [4]uint16{0, 1, 2, 3}
	value := func(m uint16) float64 { return float64(scores[0][m&1]) + float64(scores[1][m>>1&1]) }
	sort.SliceStable(x[:], func(i, j int) bool { return value(x[i]) > value(x[j]) })
	return x
}
func observed(o r.Observations) bool {
	require(len(o.Outputs) > 0, "empty observation")
	good := true
	for _, x := range o.Outputs {
		require(x.Passed == (x.Actual == x.Expected), "output pass flag differs")
		good = good && x.Passed
	}
	for _, x := range o.Conditions {
		want := x.Observation.Reached && x.Observation.Value == x.Case.Expected
		require(x.Passed == want, "condition pass flag differs")
		good = good && x.Passed
	}
	return good
}

type change struct {
	Documents                 int
	Improvements, Regressions []string
}
type pairGroup struct {
	Pairs                                          int
	RequiredChanges, ProposedChanges, LogitChanges [2]int
	BothFirstCorrect, EitherFirstCorrect           int
}
type latency struct {
	Observations int
	MedianNS     float64
	P95NS        int64
}

func distribution(values []int64) latency {
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	n := len(values)
	if n == 0 {
		return latency{}
	}
	median := float64(values[n/2])
	if n%2 == 0 {
		median = (median + float64(values[n/2-1])) / 2
	}
	return latency{n, median, values[(95*n+99)/100-1]}
}
