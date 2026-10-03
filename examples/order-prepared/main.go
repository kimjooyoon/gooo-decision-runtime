// Compare complete old, freshly prepared and retained searches on frozen inputs.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
	"unsafe"

	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/orderjudge"
	"github.com/kimjooyoon/gooo-decision-runtime/orderprepared"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

type input struct {
	Task                       struct{ ID, Group string } `json:"task"`
	SourceSHA, PlanSHA, Intent string
	Inputs, Expected           [8]int64
}

type cost struct {
	NS     int64  `json:"wall_ns"`
	Bytes  uint64 `json:"allocated_bytes"`
	Allocs uint64 `json:"allocations"`
}

type result struct {
	Search   pathplan.SearchResult     `json:"search"`
	Ranking  *orderjudge.SearchReceipt `json:"ranking"`
	Gooo, Go string
}

type record struct {
	ID, Arm, Mode string
	Trial, Budget int
	Cost          cost
	Result        result
}

type preparation struct {
	ID, Arm          string
	Runtime, Prepare cost
	ShallowBytes     uintptr
}

type residency struct {
	Arm                    string
	Trial, PreparedObjects int
	HeapBefore, HeapAfter  uint64
	LiveHeapDelta          int64
}

func resident(ctx context.Context, model *orderjudge.Model, plans []pathplan.Plan, arm string, trial int) residency {
	// Drain prior-cycle pools before the baseline. The first collector version
	// measured after large JSON serialization and mixed pool release into residency.
	runtime.GC()
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	rt, err := orderprepared.NewRuntime(model)
	must(err)
	objects := make([]*orderprepared.Prepared, len(plans))
	for i, plan := range plans {
		objects[i], err = rt.Prepare(ctx, plan)
		must(err)
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(objects)
	runtime.KeepAlive(rt)
	runtime.KeepAlive(plans)
	return residency{arm, trial, len(plans), before.HeapAlloc, after.HeapAlloc, int64(after.HeapAlloc) - int64(before.HeapAlloc)}
}

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
func raw(path string) []byte    { b, err := os.ReadFile(path); must(err); return b }
func digest(b []byte) string    { return fmt.Sprintf("%x", sha256.Sum256(b)) }
func read(path string, out any) { must(json.Unmarshal(raw(path), out)) }
func save(path string, value any) {
	b, err := json.MarshalIndent(value, "", "  ")
	must(err)
	must(os.WriteFile(path, append(b, '\n'), 0644))
}

// Allocation counters describe this single collector process. Their reads and
// output serialization are outside the wall interval. They can still perturb
// the process, so these measurements are not a production latency distribution.
func measure(fn func()) cost {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	fn()
	elapsed := time.Since(start).Nanoseconds()
	runtime.ReadMemStats(&after)
	return cost{elapsed, after.TotalAlloc - before.TotalAlloc, after.Mallocs - before.Mallocs}
}

func outcome(search pathplan.SearchResult, program *bodyplan.Program, receipt *orderjudge.SearchReceipt, err error) result {
	must(err)
	require(program != nil && receipt != nil, "missing selected program")
	return result{search, receipt, program.GoooSource(), program.GoSource()}
}

func equal(a, b result) bool {
	ra, rb := *a.Ranking, *b.Ranking
	ra.PredictNS, rb.PredictNS = 0, 0
	a.Ranking, b.Ranking = &ra, &rb
	return reflect.DeepEqual(a, b)
}

func main() {
	dir := flag.String("input", "", "decoded immutable initial study")
	out := flag.String("output", "", "fresh output directory")
	repeats := flag.Int("repeats", 5, "1..20 alternating paired rounds")
	flag.Parse()
	require(*dir != "" && *out != "" && flag.NArg() == 0 && *repeats >= 1 && *repeats <= 20, "bounded arguments required")
	_, err := os.Stat(*out)
	require(os.IsNotExist(err), "fresh output required")
	must(os.MkdirAll(*out, 0755))
	var metadata struct {
		DatasetSHA string `json:"dataset_sha256"`
	}
	read(filepath.Join(*dir, "pretraining.json"), &metadata)
	require(metadata.DatasetSHA == digest(raw(filepath.Join(*dir, "dataset.json"))), "dataset digest differs")
	weights := raw(filepath.Join(*dir, "weights.bin"))
	require(digest(weights) == "cf00ccc83d17d28ed73fcb869366151a48ffccd3aa8ca8e635aabf19810b9e78", "weights differ")
	model, err := orderjudge.Load(raw(filepath.Join(*dir, "model.json")), weights)
	must(err)
	var rows []input
	read(filepath.Join(*dir, "dataset.json"), &rows)
	require(len(rows) == 160, "initial cohort differs")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var records []record
	var preparations []preparation
	var plans []pathplan.Plan
	requests, predictions, searches := 0, 0, 0
	for rowIndex, row := range rows {
		if row.Task.Group != "new-template" && row.Task.Group != "new-constants" {
			continue
		}
		require(filepath.Base(row.Task.ID) == row.Task.ID && row.Task.ID != "", "invalid request ID")
		var exported struct {
			Plan      pathplan.Plan             `json:"expanded_plan"`
			SourceSHA string                    `json:"original_source_sha256"`
			Binding   struct{ Equivalent bool } `json:"source_binding"`
			Context   struct {
				PlanSHA string `json:"original_plan_sha256"`
			}
		}
		base := filepath.Join(*dir, "sources", row.Task.ID)
		read(base+"-context.json", &exported)
		encoded, err := json.Marshal(exported.Plan)
		must(err)
		require(exported.Binding.Equivalent && exported.SourceSHA == row.SourceSHA && row.SourceSHA == "sha256:"+digest(raw(base+".gooo")) &&
			exported.Context.PlanSHA == row.PlanSHA && digest(encoded) == row.PlanSHA, "source binding differs")
		var intents []string
		for _, c := range exported.Plan.Decisions {
			intents = append(intents, c.Intent)
		}
		require(strings.Join(intents, "\n") == row.Intent, "intent differs")
		plans = append(plans, exported.Plan)
		var cases []pathplan.TestCase
		for _, i := range []int{1, 3, 6} {
			cases = append(cases, pathplan.TestCase{Input: row.Inputs[i], Expected: row.Expected[i]})
		}
		for _, arm := range []string{"deterministic", "model"} {
			var active *orderjudge.Model
			if arm == "model" {
				active = model
			}
			var rt orderprepared.Runtime
			runtimeCost := measure(func() { rt, err = orderprepared.NewRuntime(active); must(err) })
			var prepared *orderprepared.Prepared
			prepareCost := measure(func() { prepared, err = rt.Prepare(ctx, exported.Plan); must(err) })
			require(prepared.PlanSHA256() == row.PlanSHA, "prepared identity differs")
			preparations = append(preparations, preparation{row.Task.ID, arm, runtimeCost, prepareCost, unsafe.Sizeof(*prepared)})
			for _, budget := range []int{1, 8} {
				for trial := 0; trial < *repeats; trial++ {
					var comparison [3]result
					for offset := 0; offset < 3; offset++ {
						mode := (offset + trial + rowIndex) % 3
						var r result
						var program *bodyplan.Program
						measured := measure(func() {
							var search pathplan.SearchResult
							var receipt *orderjudge.SearchReceipt
							switch mode {
							case 0:
								search, program, receipt, err = orderjudge.Search(ctx, exported.Plan, active, cases, budget, true)
							case 1:
								fresh, e := orderprepared.NewRuntime(active)
								must(e)
								p, e := fresh.Prepare(ctx, exported.Plan)
								must(e)
								search, program, receipt, err = p.Search(ctx, row.PlanSHA, cases, budget, true)
							case 2:
								search, program, receipt, err = prepared.Search(ctx, row.PlanSHA, cases, budget, true)
							}
							r = outcome(search, program, receipt, err)
						})
						comparison[mode] = r
						predictions += r.Search.Selection.ModelCalls
						searches++
						records = append(records, record{row.Task.ID, arm, []string{"original", "prepare-per-call", "retained"}[mode], trial, budget, measured, r})
						if budget == 8 {
							for i, x := range row.Inputs {
								value, e := program.Evaluate(x)
								must(e)
								require(value.Int == row.Expected[i], "evaluation expectation differs")
							}
						}
					}
					if !equal(comparison[0], comparison[1]) || !equal(comparison[0], comparison[2]) {
						save(filepath.Join(*out, "first-difference.json"), comparison)
						panic("full semantic search record differs")
					}
				}
			}
		}
		requests++
	}
	require(requests == 64 && searches == 64*2*2*3*(*repeats) && predictions == searches/2, "actual counts differ")
	var residencyRecords []residency
	for trial := 0; trial < 3; trial++ {
		residencyRecords = append(residencyRecords, resident(ctx, nil, plans, "deterministic", trial), resident(ctx, model, plans, "model", trial))
	}
	runtime.KeepAlive(records)
	runtime.KeepAlive(preparations)
	save(filepath.Join(*out, "residency.json"), residencyRecords)
	save(filepath.Join(*out, "records.json"), records)
	save(filepath.Join(*out, "preparations.json"), preparations)
	info, ok := debug.ReadBuildInfo()
	require(ok, "build metadata required")
	save(filepath.Join(*out, "manifest.json"), map[string]any{
		"schema": "gooo/order-prepared-replay/v1", "requests": requests, "repeats": *repeats, "searches": searches, "actual_model_predictions": predictions,
		"training_updates": 0, "native_runs": 0, "dataset_sha256": metadata.DatasetSHA, "weights_sha256": digest(weights),
		"resident_memory_scope": "Three post-GC live-heap observations per arm, before result serialization with two collections before each baseline. 64 prepared requests plus one owned model, including private program objects. Inputs and replay records kept alive across each observation. Process heap deltas may include runtime noise.",
		"go_version":            info.GoVersion, "build_settings": info.Settings, "goos": runtime.GOOS, "goarch": runtime.GOARCH,
		"semantic_comparison": "Complete search, score/ranking, descriptors, aliases and selected Gooo/Go; only Ranking.predict_ns removed.",
		"scope":               "Paired SDK interpreter measurements on already observed development requests. Per-operation process allocation deltas; counter reads and serialization outside wall intervals. No compiler CLI or native timing."})
	fmt.Printf("PASS: %d requests, %d searches, %d predictions, all semantic records identical\n", requests, searches, predictions)
}
