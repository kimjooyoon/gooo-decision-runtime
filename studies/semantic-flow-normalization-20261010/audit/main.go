// Recount saved records only: no Prepare, Compile, Evaluate, Fit or Predict.
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

	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

const producer = "3ecbcecdddd53b3e6e389e0004289c8e74409baf"
const compiler = "c6784f34e4ce9e208e53c361c757e04961233196"

type output struct {
	Input, Expected, Actual int64
	Passed                  bool
}
type candidate struct {
	Mask       uint16
	Outputs    []output
	Conditions []pathplan.ConditionResult
	Acceptable bool
}
type row struct {
	ID, Group, Form, Split, Source, SourceSHA, PlanSHA string
	Document                                           pathplan.Document
	Contexts                                           []pathplan.SemanticBranchContext
	V4, V5                                             [][384]float32
	V4SHA, V5SHA                                       string
	Acceptable                                         uint64
	Candidates                                         []candidate
}

func require(ok bool, s string) {
	if !ok {
		panic(s)
	}
}
func hash(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
func featureSHA(features [][384]float32) string {
	h := sha256.New()
	h.Write([]byte{byte(len(features))})
	var raw [1536]byte
	for _, f := range features {
		for i, x := range f {
			binary.LittleEndian.PutUint32(raw[i*4:], math.Float32bits(x))
		}
		h.Write(raw[:])
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
func auditRow(x row) {
	require(hash([]byte(x.Source)) == x.SourceSHA, "source bytes")
	p, e := json.Marshal(x.Document.Plan)
	require(e == nil && hash(p) == x.PlanSHA, "plan bytes")
	require(len(x.V4) == 2 && len(x.V5) == 2 && len(x.Contexts) == 2, "two source choices")
	require(featureSHA(x.V4) == x.V4SHA && featureSHA(x.V5) == x.V5SHA, "feature bytes")
	for i, view := range x.Contexts {
		a, b := x.V4[i], x.V5[i]
		require(slices.Equal(a[64:236], b[64:236]) && slices.Equal(a[256:320], b[256:320]), "intent and feedback channels")
		for _, segment := range [][]float32{b[192:236], b[256:320]} {
			for _, v := range segment {
				require(v == 0, "future observation")
			}
		}
		if !view.Normalized {
			require(a == b && x.Split == "control", "raw fallback")
			continue
		}
		require(b[255] == 2 && view.Reason == "SINGLE_BRANCH_KNOWN_ATOMS", "normalization marker")
		values := append(view.Flow.Returns[:], view.Flow.Predicate[:]...)
		for slot, v := range values {
			require(v.Present && v.Kind != "unknown", "known source atom")
			flags := [8]bool{true, v.Kind == "input", v.Kind == "int", v.Kind == "bool", false, v.Kind == "int" && v.Int < 0, v.Kind == "int" && v.Int == 0, v.Kind == "int" && v.Int > 0}
			for j, on := range flags {
				var want float32
				if on {
					want = .125
				}
				require(b[320+slot*16+j] == want, "source atom flags")
			}
			for j := range 8 {
				require(b[328+slot*16+j] == float32(byte(uint64(v.Int)>>uint(56-j*8)))/2048, "exact source bytes")
			}
		}
	}
	require(len(x.Candidates) == 4 && len(x.Document.TestCases) == 8 && len(x.Document.Plan.ConditionCases) == 3, "finite scope")
	var acceptable uint64
	for mask, c := range x.Candidates {
		require(c.Mask == uint16(mask) && len(c.Outputs) == 8 && len(c.Conditions) == 3, "candidate scope")
		good := true
		for i, o := range c.Outputs {
			expected := x.Document.TestCases[i]
			require(o.Input == expected.Input && o.Expected == expected.Expected && o.Passed == (o.Actual == o.Expected), "exact finite output")
			good = good && o.Passed
		}
		for i, cnd := range c.Conditions {
			require(cnd.Case == x.Document.Plan.ConditionCases[i], "condition source")
			require(cnd.Passed == (cnd.Observation.Reached && cnd.Observation.Value == cnd.Case.Expected), "condition outcome")
			for _, o := range c.Outputs {
				if o.Input == cnd.Case.Input {
					require(cnd.Output.Int == o.Actual, "two compiled observations agree")
				}
			}
			good = good && cnd.Passed
		}
		require(good == c.Acceptable, "candidate acceptance")
		if good {
			acceptable |= 1 << mask
		}
	}
	require(acceptable != 0 && acceptable == x.Acceptable, "complete mask set")
}
func audit(root string) map[string]any {
	f, e := os.Open(filepath.Join(root, "records.jsonl.gz"))
	require(e == nil, "records")
	defer f.Close()
	z, e := gzip.NewReader(f)
	require(e == nil, "gzip")
	defer z.Close()
	decoder := json.NewDecoder(z)
	var rows []row
	ids := map[string]bool{}
	groups := map[string]row{}
	train, normalized, controls := 0, 0, 0
	for {
		var x row
		err := decoder.Decode(&x)
		if err == io.EOF {
			break
		}
		require(err == nil, "JSON record")
		auditRow(x)
		require(!ids[x.ID], "unique source")
		ids[x.ID] = true
		rows = append(rows, x)
		if x.Form == "direct" {
			groups[x.Group] = x
		}
		if x.Split == "future_train" {
			require(x.Form == "direct", "training form")
			train++
		}
		if x.Contexts[0].Normalized && x.Contexts[1].Normalized {
			normalized++
		} else {
			controls++
		}
	}
	pairs, oldEqual, newEqual, conflicts4, conflicts5 := 0, 0, 0, 0, 0
	for i, x := range rows {
		if x.Form != "direct" && x.Split != "control" {
			d, ok := groups[x.Group]
			require(ok, "direct form")
			pairs++
			require(x.Acceptable == d.Acceptable, "unchanged valid candidates")
			if x.V4SHA == d.V4SHA {
				oldEqual++
			}
			if x.V5SHA == d.V5SHA {
				newEqual++
			}
		}
		for _, y := range rows[:i] {
			if x.Acceptable&y.Acceptable == 0 {
				if x.V4SHA == y.V4SHA {
					conflicts4++
				}
				if x.V5SHA == y.V5SHA {
					conflicts5++
				}
			}
		}
	}
	require(len(rows) == 162 && train == 16 && normalized == 160 && controls == 2 && pairs == 128, "complete dataset")
	require(oldEqual == 0 && newEqual == 128 && conflicts4 == 0 && conflicts5 == 0, "observed representation result")
	raw, e := os.ReadFile(filepath.Join(root, "report.json"))
	require(e == nil, "report")
	var report map[string]json.RawMessage
	require(json.Unmarshal(raw, &report) == nil, "report JSON")
	for k, want := range map[string]int{"Sources": 162, "PairedComparisons": 128, "EqualV4": 0, "EqualV5": 128, "ChangedAcceptable": 0, "NormalizedSources": 160, "RetainedControls": 2, "ConflictPairsV4": 0, "ConflictPairsV5": 0, "CandidateRecords": 648, "ExplicitCompileCalls": 1296, "OutputEvaluations": 5184, "ConditionObservations": 1944, "Fits": 0, "ModelCalls": 0, "NativeExecutions": 0} {
		var got int
		require(json.Unmarshal(report[k], &got) == nil && got == want, "report "+k)
	}
	for k, want := range map[string]string{"Producer": producer, "Compiler": compiler} {
		var got string
		require(json.Unmarshal(report[k], &got) == nil && got == want, k)
	}
	return map[string]any{"status": "PASS", "sources": len(rows), "matched_pairs": pairs, "v4_equal": oldEqual, "v5_equal": newEqual, "future_training_direct_sources": train, "retained_controls": controls, "new_model_calls": 0, "new_candidate_tests": 0, "new_native_runs": 0}
}
func main() {
	require(len(os.Args) == 2, "usage: audit RESULT_DIRECTORY")
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	require(e.Encode(audit(os.Args[1])) == nil, "audit output")
}
