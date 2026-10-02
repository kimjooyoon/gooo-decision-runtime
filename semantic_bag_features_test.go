package decision

import (
	"reflect"
	"strings"
	"testing"
)

func TestSemanticBagFullInputSourceAndAtomicBounds(t *testing.T) {
	text, err := EncodeSemanticContextInput(semanticTestFields(), "요청: Use the first variable. 전체 입력을 유지한다.")
	if err != nil {
		t.Fatal(err)
	}
	var old, bag [FeatureDim]float32
	if err = SemanticContextFeaturesInto(text, &old); err != nil {
		t.Fatal(err)
	}
	if err = SemanticContextBagFeaturesInto(text, &bag); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(old[:64], bag[:64]) || old == bag {
		t.Fatal("source changed or representation was not distinct")
	}
	if n := testing.AllocsPerRun(100, func() {
		if e := SemanticContextBagFeaturesInto(text, &bag); e != nil {
			panic(e)
		}
	}); n != 0 {
		t.Fatal("bag features allocate", n)
	}
	before := bag
	for _, invalid := range []string{"", "plain text", "\xff", text[:semanticContextHeaderBytes], text + strings.Repeat("x", 512), strings.Replace(text, "8000", "ff00", 1)} {
		if SemanticContextBagFeaturesInto(invalid, &bag) == nil || bag != before {
			t.Fatal("invalid input changed caller storage")
		}
	}
	if SemanticContextBagFeaturesInto(text, nil) == nil {
		t.Fatal("nil output accepted")
	}
	boundary, err := EncodeSemanticContextInput(semanticTestFields(), strings.Repeat("x", InputMaxBytes-semanticContextHeaderBytes))
	if err != nil || SemanticContextBagFeaturesInto(boundary, &bag) != nil {
		t.Fatal("full boundary input", err)
	}
}

// This is an intended negative control: global fragment counts lose operation
// order even while every byte is consumed. Keep the limitation observable.
func TestSemanticBagRetainsOrderAliasingCounterexample(t *testing.T) {
	a := "Step: add one. Step: multiply by two. Step: end."
	b := "Step: multiply by two. Step: add one. Step: end."
	fragments := func(s string) map[string]int {
		out := map[string]int{}
		for _, width := range []int{2, 3} {
			for i := 0; i+width <= len(s); i++ {
				out[s[i:i+width]]++
			}
		}
		return out
	}
	if !reflect.DeepEqual(fragments(a), fragments(b)) {
		t.Fatal("counterexample lacks identical fragment multiset")
	}
	var bags, positioned [2][FeatureDim]float32
	for i, intent := range []string{a, b} {
		text, err := EncodeSemanticContextInput(semanticTestFields(), intent)
		if err != nil {
			t.Fatal(err)
		}
		if err = SemanticContextBagFeaturesInto(text, &bags[i]); err != nil {
			t.Fatal(err)
		}
		if err = SemanticContextFeaturesInto(text, &positioned[i]); err != nil {
			t.Fatal(err)
		}
	}
	if bags[0] != bags[1] || positioned[0] == positioned[1] {
		t.Fatal("order alias/position control differs")
	}
	for _, x := range []int64{-2, 0, 3} {
		if (x+1)*2 == x*2+1 {
			t.Fatal("finite order reference failed to distinguish behavior")
		}
	}
}
