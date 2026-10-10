// Recounts saved diagnostic traces. No model forward pass or candidate execution.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/kimjooyoon/gooo-decision-runtime/flowdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

const producer = "5ccf5e21e8c41256eb50d7b0506f5893148bcb99"

type record struct {
	ID, Split, Model, ModelSHA, Fingerprint, FeatureVersion, SourceSHA, PlanSHA, InputSHA string
	Acceptable                                                                            uint64
	ChoiceIDs, Intents                                                                    []string
	Inputs                                                                                [][384]float32
	Prediction                                                                            flowdecision.Prediction
	Explanation                                                                           flowdecision.Explanation
	Valid                                                                                 bool
	ActiveHidden                                                                          [2]int
}
type previous struct {
	ID, Model, SourceSHA, InputSHA string
	Acceptable                     uint64
	Prediction                     flowdecision.Prediction
}
type frozenSource struct {
	ID, Form, Split, PlanSHA, SourceSHA string
	Document                            pathplan.Document
}
type pair struct {
	Model, MaxID, MinID                                              string
	DifferentInputs, OnlyBranchIntentDiffers, DisjointAcceptableSets bool
	SameBranchHidden, SameBranchScores, SamePrediction               bool
	MaxActive, MinActive                                             int
}
type report struct {
	Schema     string         `json:"schema"`
	Producer   string         `json:"producer"`
	Calls      int            `json:"diagnostic_forward_calls"`
	Fits       int            `json:"new_fits"`
	Executions int            `json:"new_candidate_executions"`
	Sources    int            `json:"direct_sources"`
	Pairs      int            `json:"pairs"`
	Counts     map[string]int `json:"counts"`
}

func must(e error) {
	if e != nil {
		panic(e)
	}
}
func require(ok bool, s string) {
	if !ok {
		panic(s)
	}
}
func hash(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }
func read(root, name string) []byte {
	b, e := os.ReadFile(filepath.Join(root, name))
	must(e)
	if strings.HasSuffix(name, ".gz") {
		z, e := gzip.NewReader(bytes.NewReader(b))
		must(e)
		b, e = io.ReadAll(z)
		must(e)
		must(z.Close())
	}
	return b
}
func decode[T any](b []byte) T { var t T; must(json.Unmarshal(b, &t)); return t }
func lines[T any](b []byte) []T {
	var out []T
	for l := range bytes.SplitSeq(b, []byte{'\n'}) {
		if len(l) > 0 {
			out = append(out, decode[T](l))
		}
	}
	return out
}
func inputSHA(f [][384]float32) string {
	h := sha256.New()
	_, _ = h.Write([]byte{byte(len(f))})
	for _, a := range f {
		var b [1536]byte
		for i, x := range a {
			binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(x))
		}
		_, _ = h.Write(b[:])
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
func trace(r record, old previous, m *flowdecision.Model) {
	require(r.SourceSHA == old.SourceSHA && r.InputSHA == old.InputSHA && r.Prediction == old.Prediction && r.Acceptable == old.Acceptable, "original record identity")
	require(len(r.Inputs) == 2 && inputSHA(r.Inputs) == r.InputSHA && reflect.DeepEqual(r.ChoiceIDs, []string{"comparison", "branches"}), "actual encoded input")
	require(len(r.Intents) == 2 && r.Intents[0] != "" && r.Intents[1] != "", "source intent text")
	e := r.Explanation
	require(e.ChoiceCount == 2 && e.CandidateCount == 4 && r.Prediction.Count == 4 && r.Prediction.Selected < 4, "finite trace bounds")
	require(r.Fingerprint == m.Fingerprint() && r.FeatureVersion == m.FeatureVersion(), "frozen model identity")
	weights := m.Weights()
	for c := range 16 {
		active := 0
		for _, h := range e.Hidden[c] {
			require(!math.IsNaN(float64(h)) && !math.IsInf(float64(h), 0) && h >= 0, "finite ReLU activation")
			if h > 0 {
				active++
			}
		}
		if c >= 2 {
			require(e.Hidden[c] == [24]float32{} && e.OptionScores[c] == [2]float32{}, "unused choices")
			continue
		}
		require(active == r.ActiveHidden[c], "active hidden recount")
		if active == 0 {
			require(e.OptionScores[c] == [2]float32{weights[len(weights)-2], weights[len(weights)-1]}, "inactive choice uses only final biases")
		}
	}
	for mask := range 64 {
		if mask >= 4 {
			require(e.CandidateScores[mask] == 0, "unused candidate")
			continue
		}
		want := float64(e.OptionScores[0][mask&1]) + float64(e.OptionScores[1][mask>>1&1])
		require(e.CandidateScores[mask] == want, "complete candidate score")
	}
	require(r.Valid == (r.Acceptable>>r.Prediction.Selected&1 != 0), "frozen case validity")
}
func compare(a, b record) pair {
	p := pair{Model: a.Model, MaxID: a.ID, MinID: b.ID, OnlyBranchIntentDiffers: true, DisjointAcceptableSets: a.Acceptable&b.Acceptable == 0, SameBranchHidden: a.Explanation.Hidden[1] == b.Explanation.Hidden[1], SameBranchScores: a.Explanation.OptionScores[1] == b.Explanation.OptionScores[1], SamePrediction: a.Prediction == b.Prediction, MaxActive: a.ActiveHidden[1], MinActive: b.ActiveHidden[1]}
	for c := range 2 {
		for i, v := range a.Inputs[c] {
			if v != b.Inputs[c][i] {
				p.DifferentInputs = true
				if c != 1 || i < 64 || i >= 192 {
					p.OnlyBranchIntentDiffers = false
				}
			}
		}
	}
	return p
}
func audit(root, studies string) map[string]any {
	r := decode[report](read(root, "report.json"))
	require(r.Schema == "gooo/intent-selection-diagnostics/v1" && r.Producer == producer && r.Calls == 64 && r.Fits == 0 && r.Executions == 0 && r.Sources == 32 && r.Pairs == 32, "fixed protocol")
	rows := lines[record](read(root, "records.jsonl.gz"))
	require(len(rows) == 64, "all64 traces")
	frozenRoot := filepath.Join(studies, "semantic-flow-normalization-20261010/result")
	frozenBytes, err := os.ReadFile(filepath.Join(frozenRoot, "records.jsonl.gz"))
	must(err)
	require(hash(frozenBytes) == "05fa80975f0e6ea56e0e653fc1207d5894805957cd7a4a54091b67d0e1a7ed40", "frozen source collection")
	frozen := map[string]frozenSource{}
	for _, s := range lines[frozenSource](read(frozenRoot, "records.jsonl.gz")) {
		frozen[s.ID] = s
	}
	byID := map[string]record{}
	counts := map[string]int{}
	for _, spec := range []struct{ Name, Dir, ModelSHA, PredSHA string }{
		{"v5", "semantic-flow-learning-20261010", "8127de6776d13b9c06710d1da762bb254401c1d8ff7e26ee6bb3c2d50d5cb8db", "c62ad8c7920519d335255e1f3e424fdf1a6084d1856ad72ae6878d8754c6ce5c"},
		{"v6", "relational-flow-learning-20261010", "3ba2c75cd7a3dd398e445bdd31abaaa3b3337e6261beac9e88faba4ea12c9c3b", "f585f2514054113447931339b3d6e411382919303bd475c3588ee27983b0f333"},
	} {
		base := filepath.Join(studies, spec.Dir, "result")
		modelBytes := read(base, "model-"+spec.Name+".json")
		require(hash(modelBytes) == spec.ModelSHA, "original model bytes")
		m, e := flowdecision.Decode(modelBytes)
		must(e)
		compressed, e := os.ReadFile(filepath.Join(base, "judgments.jsonl.gz"))
		must(e)
		require(hash(compressed) == spec.PredSHA, "original prediction bytes")
		old := map[string]previous{}
		for _, p := range lines[previous](read(base, "judgments.jsonl.gz")) {
			if p.Model == spec.Name {
				old[p.ID] = p
			}
		}
		n := 0
		for _, row := range rows {
			if row.Model != spec.Name {
				continue
			}
			key := row.Model + "/" + row.ID
			_, seen := byID[key]
			require(!seen && strings.HasSuffix(row.ID, "-direct") && row.ModelSHA == spec.ModelSHA, "unique exact trace")
			trace(row, old[row.ID], m)
			s, ok := frozen[row.ID]
			require(ok && s.Form == "direct" && row.PlanSHA == s.PlanSHA && row.SourceSHA == s.SourceSHA && row.Split == s.Split, "source plan identity")
			for i, choice := range s.Document.Plan.Decisions {
				require(row.ChoiceIDs[i] == choice.ID && row.Intents[i] == choice.Intent, "unchanged authored intent")
			}
			byID[key] = row
			n++
			kind, _, _ := strings.Cut(row.ID, "-")
			require(kind == "max" || kind == "min", "task kind")
			counts[spec.Name+"/"+kind+"/rows"]++
			if row.Valid {
				counts[spec.Name+"/"+kind+"/valid"]++
			}
			if row.ActiveHidden[1] == 0 {
				counts[spec.Name+"/"+kind+"/all_branch_hidden_zero"]++
			}
		}
		require(n == 32, "32 traces per model")
	}
	require(reflect.DeepEqual(counts, r.Counts), "trace aggregates")
	ps := decode[[]pair](read(root, "pairs.json"))
	require(len(ps) == 32, "all pairs")
	seen := map[string]bool{}
	collapsed := map[string]int{"v5": 0, "v6": 0}
	for _, p := range ps {
		key := p.Model + "/" + p.MaxID
		require(!seen[key] && p.MinID == "min-"+strings.TrimPrefix(p.MaxID, "max-"), "unique opposite pair")
		seen[key] = true
		a, okA := byID[key]
		b, okB := byID[p.Model+"/"+p.MinID]
		require(okA && okB, "paired traces exist")
		want := compare(a, b)
		require(want == p && p.DifferentInputs && p.OnlyBranchIntentDiffers && p.DisjointAcceptableSets, "paired facts")
		if p.MaxActive == 0 && p.MinActive == 0 {
			require(p.SameBranchHidden && p.SameBranchScores, "inactive branch invariance")
			collapsed[p.Model]++
		}
	}
	return map[string]any{"status": "PASS", "producer": producer, "traces": 64, "pairs": 32, "all_zero_branch_pairs": collapsed, "new_audit_forward_calls": 0, "new_audit_candidate_executions": 0}
}
func main() {
	require(len(os.Args) == 3, "usage: audit RESULT_DIRECTORY STUDIES_DIRECTORY")
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	must(e.Encode(audit(os.Args[1], os.Args[2])))
}
