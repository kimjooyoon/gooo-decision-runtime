// Replay frozen observations with fresh feature construction and actual bounded
// interpreter search. Keep the initial fit and the new deduplication arm separate.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"strings"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/orderfacts"
	"github.com/kimjooyoon/gooo-decision-runtime/orderjudge"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

type outcome struct{ Budget, Attempts, SelectedMask, Passed, Total int }
type original struct {
	Task                       struct{ ID, Group string } `json:"task"`
	SourceSHA, PlanSHA, Intent string
	Descriptors                [8]string
	Sample                     orderjudge.Sample
	Inputs, Expected           [8]int64
	Observed                   [8][8]int64
	Deterministic              [2]outcome
}
type observation struct {
	ID                   string
	Prediction           orderjudge.Prediction
	Model, Deterministic [2]outcome
}
type record struct {
	ID, Group, Arm string
	Budget         int
	Deduplicate    bool
	SearchNS       int64
	Search         pathplan.SearchResult
	Ranking        *orderjudge.SearchReceipt
	Outcome        outcome
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func raw(path string) []byte      { b, err := os.ReadFile(path); must(err); return b }
func sha(b []byte) string         { return fmt.Sprintf("%x", sha256.Sum256(b)) }
func decode(path string, out any) { must(json.Unmarshal(raw(path), out)) }
func save(path string, value any) {
	b, err := json.MarshalIndent(value, "", "  ")
	must(err)
	must(os.WriteFile(path, append(b, '\n'), 0644))
}
func require(ok bool, what string) {
	if !ok {
		panic(what)
	}
}

func main() {
	dir := flag.String("input", "", "decoded original fit evidence")
	out := flag.String("output", "", "fresh replay directory")
	flag.Parse()
	require(*dir != "" && *out != "" && flag.NArg() == 0, "input and output required")
	_, err := os.Stat(*out)
	require(os.IsNotExist(err), "output must be fresh")
	must(os.MkdirAll(*out, 0755))
	var metadata struct {
		DatasetSHA string `json:"dataset_sha256"`
	}
	decode(filepath.Join(*dir, "pretraining.json"), &metadata)
	require(sha(raw(filepath.Join(*dir, "dataset.json"))) == metadata.DatasetSHA, "dataset digest differs")
	model, err := orderjudge.Load(raw(filepath.Join(*dir, "model.json")), raw(filepath.Join(*dir, "weights.bin")))
	must(err)
	var rows []original
	var frozen []observation
	decode(filepath.Join(*dir, "dataset.json"), &rows)
	decode(filepath.Join(*dir, "observations.json"), &frozen)
	require(len(rows) == 160 && len(frozen) == len(rows), "cohort count differs")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var records []record
	for index, r := range rows {
		require(r.Task.ID != "" && filepath.Base(r.Task.ID) == r.Task.ID && frozen[index].ID == r.Task.ID, "row identity differs")
		base := filepath.Join(*dir, "sources", r.Task.ID)
		var e struct {
			Plan      pathplan.Plan             `json:"expanded_plan"`
			SourceSHA string                    `json:"original_source_sha256"`
			Binding   struct{ Equivalent bool } `json:"source_binding"`
			Context   struct {
				PlanSHA string `json:"original_plan_sha256"`
			}
		}
		decode(base+"-context.json", &e)
		p, err := json.Marshal(e.Plan)
		must(err)
		require(e.Binding.Equivalent && e.SourceSHA == r.SourceSHA && r.SourceSHA == "sha256:"+sha(raw(base+".gooo")) && e.Context.PlanSHA == r.PlanSHA && r.PlanSHA == sha(p), "export binding differs")
		var intents []string
		for _, c := range e.Plan.Decisions {
			intents = append(intents, c.Intent)
		}
		require(strings.Join(intents, "\n") == r.Intent, "full intent differs")
		var intent [orderjudge.IntentDim]float32
		must(orderjudge.IntentFeatures(r.Intent, &intent))
		require(intent == r.Sample.Intent, "prepared intent features differ")
		prepared, err := pathplan.Prepare(e.Plan)
		must(err)
		var acceptable uint8
		for mask := range 8 {
			encoded, err := hex.DecodeString(r.Descriptors[mask])
			must(err)
			require(len(encoded) == orderfacts.Bytes, "descriptor size differs")
			var descriptor orderfacts.Signature
			copy(descriptor[:], encoded)
			var features [orderjudge.SourceDim]float32
			must(orderjudge.SourceFeatures(descriptor, &features))
			require(features == r.Sample.Candidates[mask], "prepared candidate features differ")
			choices := prepared.Defaults()
			for bit, c := range e.Plan.Decisions {
				choices[c.ID] = c.Options[(mask>>bit)&1].Label
			}
			program, err := prepared.Compile(choices)
			must(err)
			selectionPassed := 0
			for i, x := range r.Inputs {
				v, err := program.Evaluate(x)
				must(err)
				require(v.Int == r.Observed[mask][i], "frozen interpreter output differs")
				if (i == 1 || i == 3 || i == 6) && v.Int == r.Expected[i] {
					selectionPassed++
				}
			}
			if selectionPassed == 3 {
				acceptable |= 1 << mask
			}
		}
		require(acceptable == r.Sample.Acceptable, "training acceptable masks differ")
		var cases []pathplan.TestCase
		for _, i := range []int{1, 3, 6} {
			cases = append(cases, pathplan.TestCase{Input: r.Inputs[i], Expected: r.Expected[i]})
		}
		for _, arm := range []string{"deterministic", "model"} {
			var active *orderjudge.Model
			if arm == "model" {
				active = model
			}
			for _, config := range []struct {
				budget int
				unique bool
			}{{1, false}, {8, false}, {8, true}} {
				start := time.Now()
				search, program, ranking, err := orderjudge.Search(ctx, e.Plan, active, cases, config.budget, config.unique)
				ns := time.Since(start).Nanoseconds()
				must(err)
				require(program != nil && ranking.Descriptors == r.Descriptors, "candidate descriptors differ")
				if arm == "model" {
					require(ranking.Prediction == frozen[index].Prediction, "frozen prediction differs")
				}
				mask := 0
				for bit, c := range e.Plan.Decisions {
					if search.Selection.Choices[c.ID] == c.Options[1].Label {
						mask |= 1 << bit
					}
				}
				value := outcome{Budget: config.budget, Attempts: search.Evaluated, SelectedMask: mask, Total: 8}
				for i, x := range r.Inputs {
					v, err := program.Evaluate(x)
					must(err)
					if v.Int == r.Expected[i] {
						value.Passed++
					}
				}
				if !config.unique {
					n := 0
					if config.budget == 8 {
						n = 1
					}
					expected := frozen[index].Model[n]
					if arm == "deterministic" {
						expected = r.Deterministic[n]
					}
					require(value == expected, "actual search differs from initial observation")
					if arm == "deterministic" {
						sdk, _, err := prepared.Search(ctx, nil, cases, config.budget, "")
						must(err)
						require(reflect.DeepEqual(sdk.Attempts, search.Attempts), "deterministic SDK search differs")
					}
				}
				for _, a := range ranking.Aliases {
					require(ranking.Descriptors[a.Mask] == ranking.Descriptors[a.EvaluatedAs] && r.Observed[a.Mask] == r.Observed[a.EvaluatedAs], "alias evidence differs")
				}
				records = append(records, record{r.Task.ID, r.Task.Group, arm, config.budget, config.unique, ns, search, ranking, value})
			}
		}
	}
	save(filepath.Join(*out, "records.json"), records)
	summary := map[string]map[string]int{}
	for _, r := range records {
		key := fmt.Sprintf("%s/%s/budget%d/unique%t", r.Group, r.Arm, r.Budget, r.Deduplicate)
		if summary[key] == nil {
			summary[key] = map[string]int{}
		}
		s := summary[key]
		s["requests"]++
		s["attempts"] += r.Search.Evaluated
		s["model_calls"] += r.Search.Selection.ModelCalls
		s["aliases_skipped"] += len(r.Ranking.Aliases)
		s["cases_passed"] += r.Outcome.Passed
		s["cases_total"] += r.Outcome.Total
		if r.Outcome.Passed == r.Outcome.Total {
			s["complete"]++
		}
	}
	save(filepath.Join(*out, "summary.json"), summary)
	build, ok := debug.ReadBuildInfo()
	require(ok, "build information required")
	save(filepath.Join(*out, "manifest.json"), map[string]any{"schema": "gooo/order-judge-runtime-replay/v1",
		"go_version": build.GoVersion, "build_settings": build.Settings,
		"requests": len(rows), "searches": len(records), "actual_model_predictions": len(rows) * 3,
		"baseline_sdk_searches": len(rows) * 2, "native_runs": 0, "training_updates": 0,
		"input_dataset_sha256": metadata.DatasetSHA, "weights_sha256": sha(raw(filepath.Join(*dir, "weights.bin"))),
		"scope": "Actual bounded typed-interpreter search and unchanged initial-fit replay. Deduplication is a follow-up on the observed cohort. No native compiler invocation or new holdout."})
	fmt.Printf("replayed %d requests, %d actual searches, %d model calls\n", len(rows), len(records), len(rows)*3)
}
