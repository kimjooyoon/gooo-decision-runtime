package main

import (
	"encoding/json"
	"testing"
)

func TestSavedCasesKeepIntegersAboveFloatPrecision(t *testing.T) {
	var c candidate
	if err := json.Unmarshal([]byte(`{"cases":[{"input":-9007199254740995,"expected":18014398509481990,"actual":9007199254740993,"passed":false}]}`), &c); err != nil {
		t.Fatal(err)
	}
	if c.Cases[0].Input != -9007199254740995 || c.Cases[0].Expected != 18014398509481990 || c.Cases[0].Actual != 9007199254740993 {
		t.Fatal(c)
	}
}

func TestFeatureDigestIncludesStaticSuffix(t *testing.T) {
	var a, b [384]float32
	b[383] = .125
	if featureSHA([][384]float32{a}) == featureSHA([][384]float32{b}) {
		t.Fatal("static suffix ignored")
	}
}
