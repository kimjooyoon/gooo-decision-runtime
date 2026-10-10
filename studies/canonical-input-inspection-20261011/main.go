// Inspect the retained Gooo-derived plans using the new source projection.
// This program never fits, predicts or evaluates a candidate body.
package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

const recordsSHA = "93fa5436660ef4c5793cf0b0c5cde10f0520e6f236753a607a03227a2f627c10"
const defaultRecords = "studies/multifamily-requirement-learning-20261011/result/sources.jsonl.gz"

type source struct {
	Spec          struct{ ID, Family, Form, Split string }
	Document      pathplan.Document
	OrderedInputs [][528]float32
	Acceptable    uint64
}
type pair struct {
	Direct, Assignment, Family string
	LegacySame, CanonicalSame  bool
}
type counts struct{ Pairs, LegacySame, CanonicalSame int }
type result struct {
	Schema, RecordsSHA, SourceFeatureVersion                       string
	Documents, SourceProjections, ModelCalls, CandidateEvaluations int
	Groups                                                         map[string]counts
	Pairs                                                          []pair
	Information                                                    *contractdecision.InputAudit
}

func inspect(path string) (result, error) {
	var report result
	raw, err := os.ReadFile(path)
	if err != nil {
		return report, err
	}
	z, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return report, err
	}
	data, err := io.ReadAll(io.LimitReader(z, 32<<20))
	closeErr := z.Close()
	if err != nil {
		return report, err
	}
	if closeErr != nil {
		return report, closeErr
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != recordsSHA {
		return report, fmt.Errorf("original source records differ")
	}
	var sources []source
	byID := map[string]int{}
	var rows [][][528]float32
	var samples []contractdecision.OrderedRequirementSample
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 4096), 2<<20)
	for scanner.Scan() {
		var x source
		if err := json.Unmarshal(scanner.Bytes(), &x); err != nil {
			return report, err
		}
		p, err := x.Document.Prepare()
		if err != nil {
			return report, err
		}
		input, err := p.InitialContractInput(x.Document.TestCases)
		if err != nil {
			return report, err
		}
		canonical := make([][528]float32, len(x.Document.Plan.Decisions))
		for i, choice := range x.Document.Plan.Decisions {
			var legacy [528]float32
			if err := input.OrderedSourceFeaturesInto(choice.ID, &legacy); err != nil {
				return report, err
			}
			if legacy != x.OrderedInputs[i] {
				return report, fmt.Errorf("legacy input changed: %s", x.Spec.ID)
			}
			if err := input.CanonicalOrderedSourceFeaturesInto(choice.ID, &canonical[i]); err != nil {
				return report, err
			}
		}
		byID[x.Spec.ID] = len(sources)
		sources, rows = append(sources, x), append(rows, canonical)
		samples = append(samples, contractdecision.OrderedRequirementSample{Inputs: canonical, Cases: input, Conditions: input, Masks: []uint16{0, 1, 2, 3}, Acceptable: x.Acceptable})
		report.SourceProjections += len(canonical)
	}
	if err := scanner.Err(); err != nil {
		return report, err
	}
	report.Schema, report.RecordsSHA, report.SourceFeatureVersion = "gooo/canonical-source-inspection/v1", recordsSHA, contractdecision.CanonicalOrderedSourceFeatureVersion
	report.Documents, report.Groups = len(sources), map[string]counts{}
	for i, a := range sources {
		if a.Spec.Form != "direct" || a.Spec.Split == "train" {
			continue
		}
		id := bytes.Replace([]byte(a.Spec.ID), []byte("-direct-"), []byte("-assignment-"), 1)
		j, ok := byID[string(id)]
		if !ok {
			return report, fmt.Errorf("missing paired assignment")
		}
		b := sources[j]
		if !reflect.DeepEqual(a.Document.TestCases, b.Document.TestCases) || !reflect.DeepEqual(a.Document.Plan.ConditionCases, b.Document.Plan.ConditionCases) || a.Acceptable != b.Acceptable {
			return report, fmt.Errorf("paired requirements or accepted masks differ")
		}
		p := pair{a.Spec.ID, b.Spec.ID, a.Spec.Family, reflect.DeepEqual(a.OrderedInputs, b.OrderedInputs), reflect.DeepEqual(rows[i], rows[j])}
		report.Pairs = append(report.Pairs, p)
		c := report.Groups[p.Family]
		c.Pairs++
		if p.LegacySame {
			c.LegacySame++
		}
		if p.CanonicalSame {
			c.CanonicalSame++
		}
		report.Groups[p.Family] = c
	}
	report.Information, err = contractdecision.AuditCanonicalRequirements(context.Background(), samples)
	return report, err
}

func main() {
	path := defaultRecords
	if len(os.Args) == 2 {
		path = os.Args[1]
	} else if len(os.Args) != 1 {
		panic("usage: canonical-input-inspection [SOURCES_JSONL_GZIP]")
	}
	report, err := inspect(path)
	if err != nil {
		panic(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		panic(err)
	}
}
