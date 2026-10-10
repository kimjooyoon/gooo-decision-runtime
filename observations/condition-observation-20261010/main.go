package main

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

const producer = "503659bcd8f28ee4b2cbc2e350323e54640e4d99"

func hash(raw []byte) string { h := sha256.Sum256(raw); return hex.EncodeToString(h[:]) }
func must(err error) {
	if err != nil {
		panic(err)
	}
}
func require(ok bool, message string) {
	if !ok {
		panic(message)
	}
}

var archiveHashes = map[string]string{}

func read(root, relative string) []byte {
	raw, err := os.ReadFile(filepath.Join(root, relative))
	must(err)
	archiveHashes[relative] = hash(raw)
	z, err := gzip.NewReader(strings.NewReader(string(raw)))
	must(err)
	data, err := io.ReadAll(io.LimitReader(z, (8<<20)+1))
	must(err)
	must(z.Close())
	require(len(data) <= 8<<20, "archive exceeds bounded input")
	return data
}
func decode(raw []byte, to any) { must(json.Unmarshal(raw, to)) }
func main() {
	require(len(os.Args) == 2, "usage: audit WORKBENCH_ROOT")
	root := os.Args[1]
	const contextFile = "publication/compatible-path-targets-20261010/original/context.json.gz"
	raw := read(root, contextFile)
	require(archiveHashes[contextFile] == "20a1cef0140e4af264b9641961b77fb840f4a792f577399d531e9cd846319d0b", "original context archive differs")
	var exported struct {
		Plan     pathplan.Plan `json:"expanded_plan"`
		Document string        `json:"document_sha256"`
		Source   string        `json:"original_source_sha256"`
		Context  struct {
			Plan string `json:"original_plan_sha256"`
		} `json:"context"`
	}
	decode(raw, &exported)
	prepared, err := pathplan.Prepare(exported.Plan)
	must(err)
	require(prepared.PlanSHA256() == exported.Context.Plan, "plan fingerprint differs")
	const native = "publication/joint-path-learning-20261010/native/"
	source := read(root, native+"source.gooo.gz")
	require("sha256:"+hash(source) == exported.Source, "source fingerprint differs")
	var prior struct {
		Results []struct {
			Arm       string `json:"arm"`
			Generated string `json:"generated_sha256"`
			Cases     []struct {
				Input    int64 `json:"input"`
				Expected bool  `json:"expected_negative_predicate"`
				Actual   bool  `json:"actual_condition"`
				Matched  bool  `json:"matched"`
			} `json:"cases"`
		} `json:"results"`
	}
	decode(read(root, native+"hint-audit.json.gz"), &prior)
	require(len(prior.Results) == 3, "expected three original arms")
	type row struct {
		Arm     string                     `json:"arm"`
		Choices map[string]string          `json:"choices"`
		Matched int                        `json:"matched"`
		Cases   []pathplan.ConditionResult `json:"cases"`
	}
	results := make([]row, 0, 3)
	for _, previous := range prior.Results {
		var original struct {
			Initial struct {
				Preparations []struct {
					Generation struct {
						GoooSource string `json:"gooo_source"`
						Report     struct {
							Paths struct {
								Document string `json:"document_sha256"`
								Source   string `json:"original_source_sha256"`
								Search   struct {
									Selection struct {
										Choices map[string]string `json:"choices"`
									} `json:"selection"`
								} `json:"search"`
							} `json:"body_paths"`
						} `json:"report"`
					} `json:"generation"`
				} `json:"preparations"`
			} `json:"initial"`
		}
		decode(read(root, native+previous.Arm+"/construction.json.gz"), &original)
		require(len(original.Initial.Preparations) == 1, "expected one selected activity")
		generation := original.Initial.Preparations[0].Generation
		paths := generation.Report.Paths
		require(paths.Document == exported.Document && paths.Source == exported.Source, "construction/context binding differs")
		program, err := prepared.Compile(paths.Search.Selection.Choices)
		must(err)
		prefix := "activity Choose(Integer) -> Integer computes "
		offset := strings.Index(generation.GoooSource, prefix)
		require(offset >= 0, "missing selected Gooo body")
		literal, err := strconv.QuotedPrefix(generation.GoooSource[offset+len(prefix):])
		must(err)
		body, err := strconv.Unquote(literal)
		must(err)
		require(strings.TrimSpace(body) == strings.TrimSpace(program.GoooBody()), "selected Gooo body differs from original artifact")
		generated := read(root, native+previous.Arm+"/generated.go.gz")
		require("sha256:"+hash(generated) == previous.Generated, "original native/AST audit binding differs")
		require(len(previous.Cases) == 3, "expected original three posthoc probes")
		cases := make([]pathplan.ConditionCase, len(previous.Cases))
		for i, test := range previous.Cases {
			cases[i] = pathplan.ConditionCase{ChoiceID: "comparison", Input: test.Input, Expected: test.Expected}
		}
		observed, err := prepared.CheckConditions(context.Background(), paths.Search.Selection.Choices, cases)
		must(err)
		current := row{Arm: previous.Arm, Choices: paths.Search.Selection.Choices, Cases: observed}
		for i, test := range observed {
			require(test.Observation.Reached && test.Observation.Value == previous.Cases[i].Actual && test.Passed == previous.Cases[i].Matched, "runtime observation differs from original Go AST audit")
			if test.Passed {
				current.Matched++
			}
		}
		results = append(results, current)
	}
	output := struct {
		Schema   string            `json:"schema"`
		Producer string            `json:"sdk_producer"`
		Plan     string            `json:"plan_sha256"`
		Source   string            `json:"source_sha256"`
		Document string            `json:"document_sha256"`
		Results  []row             `json:"results"`
		Archives map[string]string `json:"archive_sha256"`
		Calls    int               `json:"new_model_calls"`
		Scope    string            `json:"scope"`
	}{"gooo/intermediate-condition-observation/v1", producer, prepared.PlanSHA256(), exported.Source, exported.Document, results, archiveHashes, 0, "Replay of the three original selected Gooo bodies against already-authored posthoc predicate probes; final outputs unchanged; no retraining, new generation, heldout accuracy claim or search filtering."}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	must(encoder.Encode(output))
	fmt.Fprintln(os.Stderr, "source, plan, selected Gooo bodies, native artifact hashes and previous AST observations matched")
}
