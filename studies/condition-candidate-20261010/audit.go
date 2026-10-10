// Audit re-counts the frozen report and verifies its source/model hashes. It
// performs zero fits, predictions, compilations or program executions.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/kimjooyoon/gooo-decision-runtime/conditiondecision"
)

type group struct{ Rows, Valid, BaselineValid, OutputsPassed, OutputsTotal, ConditionsPassed, ConditionsTotal int }
type outcome struct {
	Mask                                                       uint16
	OutputPassed, OutputTotal, ConditionPassed, ConditionTotal int
}
type row struct {
	ID, Split, Context, SourceSHA, PlanSHA, SelectedGooo, SelectedGoSHA string
	Mask                                                                uint16
	Acceptable                                                          uint64
	Valid, BaselineValid                                                bool
	Passed                                                              outcome
}
type source struct {
	ID     string `json:"id"`
	Split  string `json:"split"`
	SHA    string `json:"source_sha256"`
	Source string `json:"gooo_source"`
}
type report struct {
	Schema     string           `json:"schema"`
	DatasetSHA string           `json:"dataset_sha256"`
	ModelSHA   string           `json:"model_sha256"`
	Calls      int              `json:"model_predictions"`
	Replays    int              `json:"selected_candidate_replays"`
	Native     int              `json:"native_executions"`
	Groups     map[string]group `json:"groups"`
	Rows       []row            `json:"results"`
}

func digest(raw []byte) string { s := sha256.Sum256(raw); return hex.EncodeToString(s[:]) }
func read(path string) []byte {
	raw, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return raw
}
func main() {
	if len(os.Args) != 2 {
		panic("study directory required")
	}
	root := os.Args[1]
	dataset, model := read(filepath.Join(root, "dataset.json")), read(filepath.Join(root, "result/model.json"))
	var r report
	if err := json.Unmarshal(read(filepath.Join(root, "result/report.json")), &r); err != nil {
		panic(err)
	}
	if r.Schema != "gooo/condition-candidate-study/v1" || r.DatasetSHA != digest(dataset) || r.ModelSHA != digest(model) || r.Calls != 120 || r.Replays != 120 || r.Native != 0 || len(r.Rows) != 120 {
		panic("frozen report identity differs")
	}
	if _, err := conditiondecision.Decode(model); err != nil {
		panic(err)
	}
	var sources []source
	if err := json.Unmarshal(dataset, &sources); err != nil {
		panic(err)
	}
	byID := map[string]source{}
	for _, s := range sources {
		if s.SHA != digest([]byte(s.Source)) {
			panic("source hash differs")
		}
		byID[s.ID] = s
	}
	if len(byID) != 24 || len(sources) != 24 {
		panic("source count differs")
	}
	groups := map[string]group{}
	seen := map[string]bool{}
	for _, row := range r.Rows {
		s, ok := byID[row.ID]
		key := row.ID + "/" + row.Context
		if !ok || s.Split != row.Split || s.SHA != row.SourceSHA || seen[key] || row.Mask > 3 || row.Passed.Mask != row.Mask || row.Acceptable == 0 || row.Acceptable > 15 {
			panic("row source or candidate differs")
		}
		seen[key] = true
		if row.Valid != (row.Acceptable>>row.Mask&1 != 0) || row.BaselineValid != (row.Acceptable&1 != 0) || row.Passed.OutputTotal != 7 || row.Passed.ConditionTotal != 3 || row.Passed.OutputPassed < 0 || row.Passed.OutputPassed > 7 || row.Passed.ConditionPassed < 0 || row.Passed.ConditionPassed > 3 || row.Valid != (row.Passed.OutputPassed == 7 && row.Passed.ConditionPassed == 3) || row.SelectedGooo == "" || len(row.SelectedGoSHA) != 64 || len(row.PlanSHA) != 64 {
			panic("row denominators or validity differ")
		}
		kind := "observed"
		if row.Context == "initial" {
			kind = "initial"
		} else if row.Context != "observed-0" && row.Context != "observed-1" && row.Context != "observed-2" && row.Context != "observed-3" {
			panic("unexpected context")
		}
		key = row.Split + "/" + kind
		g := groups[key]
		g.Rows++
		if row.Valid {
			g.Valid++
		}
		if row.BaselineValid {
			g.BaselineValid++
		}
		g.OutputsPassed += row.Passed.OutputPassed
		g.OutputsTotal += row.Passed.OutputTotal
		g.ConditionsPassed += row.Passed.ConditionPassed
		g.ConditionsTotal += row.Passed.ConditionTotal
		groups[key] = g
	}
	if !reflect.DeepEqual(groups, r.Groups) {
		panic("reported groups differ from120 rows")
	}
	result := map[string]any{"status": "PASS", "unique_rows": len(seen), "groups": groups, "new_predictions": 0, "new_fits": 0, "new_program_executions": 0, "model_sha256": r.ModelSHA, "dataset_sha256": r.DatasetSHA}
	raw, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(raw))
}
