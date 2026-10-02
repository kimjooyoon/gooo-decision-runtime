package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

func TestExampleEmitsResolvedGooo(t *testing.T) {
	var output bytes.Buffer
	if err := run(&output); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Before pathplan.ProbeRanking `json:"before"`
		After  pathplan.ProbeRanking `json:"after"`
		Source string                `json:"gooo_source"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Before.SurvivingMasks) != 2 || len(result.After.SurvivingMasks) != 1 ||
		!strings.Contains(result.Source, "return (2 - input)") {
		t.Fatal(output.String())
	}
}
