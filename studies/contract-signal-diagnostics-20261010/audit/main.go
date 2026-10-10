// Read-only reconstruction of recorded arithmetic. No model API prediction,
// training, compiler lowering, candidate execution or fresh observation occurs.
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"

	c "github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/contract-goals-20261010/record"
	d "github.com/kimjooyoon/gooo-decision-runtime/studies/contract-signal-diagnostics-20261010/record"
)

const producer = "662389ae86b0606a1354e5e73d33d3df48a8bd32"
const traceSHA = "f9c57eb6bfc73b8c54017804c4c298be2b8d69ee1ed4d434ee1477d6a4b8b6a0"

type report struct {
	Schema   string              `json:"schema"`
	Producer string              `json:"producer"`
	Calls    int                 `json:"model_calls"`
	Fits     int                 `json:"fits"`
	Bodies   int                 `json:"body_executions"`
	Sources  int                 `json:"sources"`
	Models   []d.ModelSpec       `json:"models"`
	Times    map[string][2]int64 `json:"diagnostic_median_p95_ns"`
	TraceSHA string              `json:"trace_sha256"`
}

func decode[T any](raw []byte) T { var v T; d.Must(json.Unmarshal(raw, &v)); return v }

func audit(root, studies string) (result d.Summary, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("%v", p)
		}
	}()
	rep := decode[report](d.Read(root, "report.json", ""))
	d.Need(rep.Producer == producer && rep.Calls == 392 && rep.Fits == 0 && rep.Bodies == 0 && rep.Sources == 196 && rep.TraceSHA == traceSHA && reflect.DeepEqual(rep.Models, d.Models), "one fixed diagnostic run")
	sources := d.Sources(studies)
	byID := map[string]r.Source{}
	for _, source := range sources {
		byID[source.Spec.ID] = source
	}
	raw := d.Read(root, "traces.jsonl.gz", "")
	d.Need(r.Hash(raw) == traceSHA, "original trace bytes")
	rows := d.Lines[d.Row](raw)
	d.Need(len(rows) == 392, "392 records")
	seen := map[string]bool{}
	for _, spec := range d.Models {
		modelRaw := d.Read(filepath.Join(studies, spec.Study, "result"), "model.json", spec.SHA)
		model, e := c.Decode(modelRaw)
		d.Must(e)
		weights := model.Weights()
		saved := d.Saved(studies, spec)
		var ns []int64
		for _, row := range rows {
			if row.Mode != spec.Mode {
				continue
			}
			key := row.Mode + "/" + row.ID
			source, ok := byID[row.ID]
			d.Need(ok && !seen[key], "unique original source")
			seen[key] = true
			d.Need(row.ModelSHA == spec.SHA && row.Fingerprint == model.Fingerprint() && row.Trace.Pooling == model.Pooling(), "frozen model binding")
			checkTrace(row, source, weights)
			old, ok := saved[row.ID]
			d.Need(ok && row.Trace.OptionScores == old.Ranking.Logits && row.Prediction.Selected == old.Ranking.Proposed, "exact saved outcomes")
			d.Need(row.NS > 0, "positive observed duration")
			ns = append(ns, row.NS)
		}
		d.Need(len(ns) == 196, "196 calls per model")
		slices.Sort(ns)
		d.Need(rep.Times[spec.Mode] == [2]int64{ns[98], ns[186]}, "observed quantiles")
	}
	d.Need(len(seen) == len(rows), "no unknown model rows")
	result = d.Summarize(rows, sources)
	savedSummary := decode[d.Summary](d.Read(root, "summary.json", ""))
	d.Need(reflect.DeepEqual(result, savedSummary), "complete paired recount")
	d.Need(result.Groups["mean/all"].Pairs == 97 && result.Groups["extreme/all"].Pairs == 97, "all satisfiable opposite goals")
	return result, nil
}

func checkTrace(row d.Row, source r.Source, weights [c.ParameterCount]float32) {
	d.Need(row.ID == source.Spec.ID && row.SourceSHA == source.SourceSHA && row.PlanSHA == source.PlanSHA && row.CaseSHA == source.CaseSHA && row.InputSHA == source.InputSHA, "exact source/goal arrays")
	t := row.Trace
	d.Need(t.ChoiceCount == 2 && t.CandidateCount == 4 && t.CaseCount == len(source.CaseRows), "dimensions")
	want := c.Explanation{ChoiceCount: 2, CandidateCount: 4, CaseCount: len(source.CaseRows), Pooling: t.Pooling}
	pool(&want, source, weights)
	const sourceBase = 32*8 + 8
	const biasBase = sourceBase + (384+8)*24
	const optionBase = biasBase + 24
	const optionBias = optionBase + 24*2
	for choice, input := range source.Inputs {
		for h := range 24 {
			sum := weights[biasBase+h]
			for j, x := range input {
				sum += x * weights[sourceBase+h*392+j]
			}
			want.SourcePrefix[choice][h] = sum
			for j, x := range want.Pool {
				sum += x * weights[sourceBase+h*392+384+j]
			}
			want.Joint[choice][h], want.Hidden[choice][h] = sum, activation(sum)
		}
		for option := range 2 {
			sum := weights[optionBias+option]
			for h, x := range want.Hidden[choice] {
				sum += x * weights[optionBase+option*24+h]
			}
			want.OptionScores[choice][option] = sum
		}
	}
	for mask := range 4 {
		for choice := range 2 {
			want.CandidateScores[mask] += float64(want.OptionScores[choice][mask>>choice&1])
		}
	}
	d.Need(t == want, "actual intermediate arithmetic")
	checkPrediction(row.Prediction, want.CandidateScores)
}

func activation(x float32) float32 {
	if x <= 0 {
		return x * 0.01
	}
	return x
}

func pool(trace *c.Explanation, source r.Source, weights [c.ParameterCount]float32) {
	var sums [8]float64
	for h := range 8 {
		trace.Winner[h] = -1
	}
	for i, row := range source.CaseRows {
		for h := range 8 {
			sum := weights[256+h]
			for j, x := range row {
				sum += x * weights[h*32+j]
			}
			v := activation(sum)
			if trace.Pooling == c.MeanPooling {
				sums[h] += float64(v)
				continue
			}
			d.Need(trace.Pooling == c.ExtremePooling, "known aggregation")
			old := trace.Pool[h]
			if i == 0 || math.Abs(float64(v)) > math.Abs(float64(old)) || math.Abs(float64(v)) == math.Abs(float64(old)) && v > old {
				trace.Pool[h], trace.Winner[h] = v, i
			}
		}
	}
	if trace.Pooling == c.MeanPooling {
		for h, sum := range sums {
			trace.Pool[h] = float32(sum / float64(len(source.CaseRows)))
		}
	}
}

func checkPrediction(p c.Prediction, scores [64]float64) {
	d.Need(p.Count == 4, "finite candidate count")
	best, sum := 0, 0.0
	for i := range 4 {
		if scores[i] > scores[best] {
			best = i
		}
	}
	var want [64]float32
	for i := range 4 {
		sum += math.Exp(scores[i] - scores[best])
	}
	for i := range 4 {
		want[i] = float32(math.Exp(scores[i]-scores[best]) / sum)
	}
	d.Need(p.Selected == uint16(best) && p.Probabilities == want, "finite scores to distribution")
}

func main() {
	d.Need(len(os.Args) == 3, "usage: audit RESULT STUDIES")
	result, err := audit(os.Args[1], os.Args[2])
	d.Must(err)
	d.Must(json.NewEncoder(os.Stdout).Encode(result))
}
