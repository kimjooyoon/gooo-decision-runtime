package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/contract-goals-20261010/record"
	d "github.com/kimjooyoon/gooo-decision-runtime/studies/contract-signal-diagnostics-20261010/record"
)

type cases struct {
	rows  [][32]float32
	reads int
}

func (c *cases) CaseCount() int                          { return len(c.rows) }
func (c *cases) CaseFeatures(i int) ([32]float32, error) { c.reads++; return c.rows[i], nil }

func save(root, name string, value any) {
	f, err := os.OpenFile(filepath.Join(root, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	d.Must(err)
	d.Must(json.NewEncoder(f).Encode(value))
	d.Must(f.Close())
}

func identity() string {
	b, ok := debug.ReadBuildInfo()
	d.Need(ok && b.GoVersion == "go1.27.2", "Go1.27.2 required")
	rev, clean := "", false
	for _, setting := range b.Settings {
		if setting.Key == "vcs.revision" {
			rev = setting.Value
		}
		if setting.Key == "vcs.modified" {
			clean = setting.Value == "false"
		}
	}
	d.Need(clean && len(rev) == 40, "clean committed producer required")
	return rev
}

func main() {
	d.Need(len(os.Args) == 3, "usage: producer STUDIES FRESH_OUTPUT")
	producer := identity()
	studies, out := os.Args[1], os.Args[2]
	sources := d.Sources(studies)
	d.Must(os.Mkdir(out, 0700))
	save(out, "started.json", map[string]string{"producer": producer, "time": time.Now().UTC().Format(time.RFC3339Nano)})
	f, err := os.OpenFile(filepath.Join(out, "traces.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	d.Must(err)
	encoder := json.NewEncoder(f)
	var rows []d.Row
	timings := map[string][2]int64{}
	for _, spec := range d.Models {
		raw := d.Read(filepath.Join(studies, spec.Study, "result"), "model.json", spec.SHA)
		model, err := contractdecision.Decode(raw)
		d.Must(err)
		saved := d.Saved(studies, spec)
		var ns []int64
		for _, source := range sources {
			row := observe(model, source, spec)
			d.Must(encoder.Encode(row))
			rows = append(rows, row)
			ns = append(ns, row.NS)
			old, ok := saved[row.ID]
			d.Need(ok && row.Prediction.Selected == old.Ranking.Proposed && row.Trace.OptionScores == old.Ranking.Logits, "saved exact score/selection parity "+row.ID)
		}
		slices.Sort(ns)
		timings[spec.Mode] = [2]int64{ns[len(ns)/2], ns[(len(ns)*95+99)/100-1]}
	}
	d.Must(f.Close())
	d.Need(len(rows) == 392, "392new diagnostic calls")
	save(out, "summary.json", d.Summarize(rows, sources))
	save(out, "report.json", map[string]any{"schema": "gooo/contract-signal-diagnostic/v1", "producer": producer,
		"model_calls": len(rows), "fits": 0, "body_executions": 0, "sources": len(sources), "models": d.Models,
		"diagnostic_median_p95_ns": timings, "trace_sha256": r.Hash(d.Read(out, "traces.jsonl", ""))})
	save(out, "completed.json", map[string]any{"time": time.Now().UTC().Format(time.RFC3339Nano), "model_calls": len(rows)})
	fmt.Println("392 new diagnostic forwards; all saved scores and selections retained; zero fits/body executions")
}

func observe(model *contractdecision.Model, source r.Source, spec d.ModelSpec) d.Row {
	d.Need(len(source.Inputs) == 2 && len(source.CaseRows) > 0, "fixed two-choice contract")
	row := d.Row{ID: source.Spec.ID, Mode: spec.Mode, SourceSHA: source.SourceSHA, PlanSHA: source.PlanSHA,
		CaseSHA: source.CaseSHA, InputSHA: source.InputSHA, ModelSHA: spec.SHA, Fingerprint: model.Fingerprint()}
	reader := &cases{rows: source.CaseRows}
	var w contractdecision.Workspace
	start := time.Now()
	d.Must(model.ExplainInto(source.Inputs, reader, []uint16{0, 1, 2, 3}, &w, &row.Prediction, &row.Trace))
	row.NS = time.Since(start).Nanoseconds()
	d.Need(reader.reads == len(source.CaseRows), "exactly one read per case")
	return row
}
