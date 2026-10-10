package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/bits"
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/canonical-role-learning-20261011/record"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func require(ok bool, reason string) {
	if !ok {
		panic(reason)
	}
}
func payload(root, name string) []byte {
	raw, err := os.ReadFile(filepath.Join(root, name))
	if err == nil {
		return raw
	}
	require(errors.Is(err, os.ErrNotExist), "cannot read retained payload")
	packed, err := os.ReadFile(filepath.Join(root, name+".gz"))
	must(err)
	z, err := gzip.NewReader(bytes.NewReader(packed))
	must(err)
	defer z.Close()
	raw, err = io.ReadAll(z)
	must(err)
	return raw
}
func read[T any](root, name string) T {
	raw := payload(root, name)
	var out T
	must(json.Unmarshal(raw, &out))
	return out
}
func lines[T any](root, name string) []T {
	d := json.NewDecoder(bytes.NewReader(payload(root, name)))
	var out []T
	for {
		var row T
		err := d.Decode(&row)
		if err == io.EOF {
			break
		}
		must(err)
		out = append(out, row)
	}
	return out
}
func hash(root, name string) string {
	return fmt.Sprintf("%x", sha256.Sum256(payload(root, name)))
}
func order(scores [2][2]float32) [4]uint16 {
	x := [4]uint16{0, 1, 2, 3}
	value := func(m uint16) float64 { return float64(scores[0][m&1]) + float64(scores[1][m>>1&1]) }
	sort.SliceStable(x[:], func(i, j int) bool { return value(x[i]) > value(x[j]) })
	return x
}
func observed(o r.Observations) bool {
	require(len(o.Outputs) > 0 && len(o.Conditions) > 0, "empty observation")
	good := true
	for _, x := range o.Outputs {
		require(x.Passed == (x.Actual == x.Expected), "output pass flag differs")
		good = good && x.Passed
	}
	for _, x := range o.Conditions {
		want := x.Observation.Reached && x.Observation.Value == x.Case.Expected
		require(x.Passed == want, "condition pass flag differs")
		good = good && x.Passed
	}
	return good
}

type change struct {
	Documents                 int
	Improvements, Regressions []string
}
type pairGroup struct {
	Pairs                                          int
	RequiredChanges, ProposedChanges, LogitChanges [2]int
	BothFirstCorrect, EitherFirstCorrect           int
}
type latency struct {
	Observations int
	MedianNS     float64
	P95NS        int64
}

func distribution(values []int64) latency {
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	n := len(values)
	if n == 0 {
		return latency{}
	}
	median := float64(values[n/2])
	if n%2 == 0 {
		median = (median + float64(values[n/2-1])) / 2
	}
	return latency{n, median, values[(95*n+99)/100-1]}
}
func main() {
	require(len(os.Args) == 2, "need original result directory")
	root := os.Args[1]
	report := read[r.Report](root, "report.json")
	require(report.Schema == "gooo/canonical-role-study/v1" && report.Compiler == r.Compiler && report.SDKBase == r.SDKBase && report.CorpusSHA == r.CorpusSHA, "producer identities differ")
	require(report.Sources == 256 && report.TrainingRows == 64 && report.Fits == 3 && report.Searches == 768 && report.ModelCalls == 768, "reported populations differ")
	require(report.Options == (contractdecision.FitOptions{Epochs: 2000, LearningRate: .3, L2: .0001, Seed: 37}), "fit recipe differs")
	identities := map[string]string{}
	for _, name := range []string{"joint", "comparison", "branches"} {
		raw, err := os.ReadFile(filepath.Join(root, "model-"+name+".json"))
		must(err)
		model, err := contractdecision.DecodeCanonicalInteractionRequirementConditioned(raw)
		must(err)
		info := read[r.Model](root, "model-info-"+name+".json")
		require(info == report.Models[name] && info.SHA == r.Hash(raw) && info.Fingerprint == model.Fingerprint() && model.Pooling() == contractdecision.MeanPooling, "model identity differs")
		require(info.Parameters == 19034 && info.WeightBytes == 76136 && info.ArtifactBytes == len(raw), "model dimensions differ")
		history := read[[]contractdecision.Epoch](root, "history-"+name+".json")
		require(len(history) == 2000 && history[0].Loss == info.FirstLoss && history[1999].Loss == info.LastLoss, "loss history differs")
		for _, epoch := range history {
			require(!math.IsNaN(epoch.Loss) && !math.IsInf(epoch.Loss, 0), "nonfinite loss")
		}
		identities["model-"+name+".json"] = info.SHA
	}
	sources := lines[r.Source](root, "sources.jsonl")
	searches := lines[r.Search](root, "searches.jsonl")
	training := read[[]r.TrainingRecord](root, "training.json")
	require(len(sources) == 256 && len(searches) == 768 && len(training) == 64, "retained populations differ")
	byID := map[string]r.Source{}
	bySpec := map[r.Spec]r.Source{}
	records := map[string]r.Search{}
	trainingSeen := map[string]bool{}
	for _, s := range sources {
		_, exists := byID[s.Spec.ID]
		require(!exists, "duplicate source")
		require(s.SourceSHA == r.Hash([]byte(s.Gooo)) && len(s.Candidates) == 4, "source identity or space differs")
		require(len(s.CanonicalInputs) == 2 && len(s.OutputRows) == 8 && len(s.ConditionRows) == 3, "input dimensions differ")
		decisions := s.Document.Plan.Decisions
		require(len(decisions) == 2 && decisions[0].ID == "comparison" && string(decisions[0].Kind) == "operand_order" && decisions[1].ID == "branches" && string(decisions[1].Kind) == "branch_layout", "role identities differ")
		var good, probe uint64
		for mask, c := range s.Candidates {
			require(int(c.Mask) == mask, "candidate order differs")
			require(c.Acceptable == observed(c.Declared) && c.ProbeAcceptable == observed(c.Probe), "candidate completeness differs")
			if c.Acceptable {
				good |= 1 << mask
			}
			if c.ProbeAcceptable {
				probe |= 1 << mask
			}
		}
		require(bits.OnesCount64(good) == 1 && good == s.Acceptable && probe == s.ProbeAcceptable, "accepted masks differ")
		byID[s.Spec.ID] = s
		key := s.Spec
		key.ID = ""
		bySpec[key] = s
	}
	for _, tr := range training {
		s, ok := byID[tr.ID]
		require(ok && s.Spec.Split == "train" && !trainingSeen[tr.ID], "training scope differs")
		trainingSeen[tr.ID] = true
		cmp, err := r.ProjectRole(s.Acceptable, 0)
		must(err)
		branch, err := r.ProjectRole(s.Acceptable, 1)
		must(err)
		require(tr.PlanSHA == s.PlanSHA && tr.CasesSHA == s.CasesSHA && tr.Joint == s.Acceptable && tr.Comparison == cmp && tr.Branches == branch, "projected targets differ")
		require(reflect.DeepEqual(tr.CanonicalInputs, s.CanonicalInputs) && reflect.DeepEqual(tr.Outputs, s.OutputRows) && reflect.DeepEqual(tr.Conditions, s.ConditionRows), "training input changed")
	}
	groups := map[string]r.Group{}
	timings := map[string][]int64{}
	for _, row := range searches {
		s, ok := byID[row.ID]
		require(ok && row.Split == s.Spec.Split && row.Family == s.Spec.Family && row.Form == s.Spec.Form, "search identity differs")
		key := row.ID + "/" + row.Mode
		_, exists := records[key]
		require(!exists, "duplicate search")
		records[key] = row
		names := map[string][]string{"fixed": {}, "joint": {"joint"}, "split": {"comparison", "branches"}}[row.Mode]
		require(row.Mode == "fixed" || row.Mode == "joint" || row.Mode == "split", "mode differs")
		require(len(row.Predictions) == len(names), "initial inference count differs")
		var scores [2][2]float32
		var ns int64
		for i, p := range row.Predictions {
			require(p.Model == names[i] && p.Fingerprint == report.Models[p.Model].Fingerprint, "component model differs")
			ns += p.NS
			timings[p.Model] = append(timings[p.Model], p.NS)
			if row.Mode == "joint" {
				scores = p.Logits
			} else {
				scores[i] = p.Logits[i]
			}
		}
		require(scores == row.Logits && row.Order == order(scores), "role extraction or finite ranking differs")
		require(len(row.Attempts) > 0 && len(row.Attempts) <= 4, "invalid attempt count")
		for i, a := range row.Attempts {
			require(a.Mask == row.Order[i], "attempt order differs")
			original := s.Candidates[a.Mask]
			require(a.Gooo == original.Gooo && a.Go == original.Go && reflect.DeepEqual(a.Declared, original.Declared), "body or observations differ from collection")
			require(a.Acceptable == observed(a.Declared), "attempt result differs")
			require(!a.Acceptable || i == len(row.Attempts)-1, "continued after completion")
		}
		last := row.Attempts[len(row.Attempts)-1]
		require((row.Selected != nil) == last.Acceptable, "selection completeness differs")
		finalProbe := false
		if row.Selected != nil {
			require(*row.Selected == last.Mask && reflect.DeepEqual(row.Probe, s.Candidates[last.Mask].Probe), "selected probes differ")
			finalProbe = observed(row.Probe)
		} else {
			require(len(row.Attempts) == 4, "stopped before exhaustion")
		}
		first := row.Attempts[0]
		target := uint16(bits.TrailingZeros64(s.Acceptable))
		for _, name := range []string{row.Mode + "/all", row.Mode + "/" + row.Split} {
			g := groups[name]
			g.Documents++
			if first.Acceptable {
				g.FirstDeclared++
			}
			if s.Candidates[first.Mask].ProbeAcceptable {
				g.FirstProbe++
			}
			if row.Selected != nil {
				g.FinalDeclared++
			}
			if finalProbe {
				g.FinalProbe++
			}
			if first.Mask&1 == target&1 {
				g.ComparisonCorrect++
			}
			if first.Mask>>1&1 == target>>1&1 {
				g.BranchCorrect++
			}
			g.CandidateChecks += len(row.Attempts)
			g.ModelCalls += len(row.Predictions)
			g.SearchNS += row.NS
			g.PredictionNS += ns
			groups[name] = g
		}
	}
	require(reflect.DeepEqual(groups, report.Groups), "group summary differs")
	policy := read[struct {
		Schema     string  `json:"schema"`
		Comparison r.Model `json:"comparison"`
		Branches   r.Model `json:"branches"`
		Calls      int     `json:"model_calls_before_first_candidate"`
		Parameters int     `json:"parameters"`
		Bytes      int     `json:"weight_bytes"`
	}](root, "split-policy.json")
	require(policy.Schema == "gooo/canonical-role-research-policy/v1" && policy.Comparison == report.Models["comparison"] && policy.Branches == report.Models["branches"] && policy.Calls == 2 && policy.Parameters == 38068 && policy.Bytes == 152272, "split policy metadata differs")
	changes := map[string]*change{}
	paired := map[string]*pairGroup{}
	equivalent := map[string]int{}
	for _, s := range sources {
		a, b := records[s.Spec.ID+"/joint"], records[s.Spec.ID+"/split"]
		if changes[s.Spec.Split] == nil {
			changes[s.Spec.Split] = &change{}
		}
		ch := changes[s.Spec.Split]
		ch.Documents++
		if !a.Attempts[0].Acceptable && b.Attempts[0].Acceptable {
			ch.Improvements = append(ch.Improvements, s.Spec.ID)
		}
		if a.Attempts[0].Acceptable && !b.Attempts[0].Acceptable {
			ch.Regressions = append(ch.Regressions, s.Spec.ID)
		}
		if s.Spec.Form == "assignment" {
			key := s.Spec
			key.ID = ""
			key.Form = "direct"
			other, ok := bySpec[key]
			require(ok && reflect.DeepEqual(s.CanonicalInputs, other.CanonicalInputs), "equivalent source input differs")
			for _, mode := range []string{"fixed", "joint", "split"} {
				x, y := records[s.Spec.ID+"/"+mode], records[other.Spec.ID+"/"+mode]
				if x.Logits == y.Logits && x.Order == y.Order {
					equivalent[mode]++
				}
			}
		}
		for _, kind := range []string{"output", "condition"} {
			key := s.Spec
			key.ID = ""
			if kind == "output" {
				if key.OutputGoal != 0 {
					continue
				}
				key.OutputGoal = 1
			} else {
				if key.ConditionGoal != 0 {
					continue
				}
				key.ConditionGoal = 1
			}
			other, ok := bySpec[key]
			require(ok && reflect.DeepEqual(s.CanonicalInputs, other.CanonicalInputs), "goal pair missing or source differs")
			label := kind
			if kind == "output" {
				require(reflect.DeepEqual(s.ConditionRows, other.ConditionRows) && !reflect.DeepEqual(s.OutputRows, other.OutputRows), "output pairing differs")
			} else {
				require(!reflect.DeepEqual(s.ConditionRows, other.ConditionRows), "condition target unchanged")
				if !reflect.DeepEqual(s.OutputRows, other.OutputRows) {
					label = "condition_with_output_change"
				}
			}
			for _, mode := range []string{"fixed", "joint", "split"} {
				x, y := records[s.Spec.ID+"/"+mode], records[other.Spec.ID+"/"+mode]
				name := label + "/" + s.Spec.Split + "/" + mode
				if paired[name] == nil {
					paired[name] = &pairGroup{}
				}
				g := paired[name]
				g.Pairs++
				ta, tb := uint16(bits.TrailingZeros64(s.Acceptable)), uint16(bits.TrailingZeros64(other.Acceptable))
				for bit := range 2 {
					if (ta^tb)>>bit&1 == 1 {
						g.RequiredChanges[bit]++
					}
					if (x.Order[0]^y.Order[0])>>bit&1 == 1 {
						g.ProposedChanges[bit]++
					}
					if x.Logits[bit] != y.Logits[bit] {
						g.LogitChanges[bit]++
					}
				}
				if x.Attempts[0].Acceptable && y.Attempts[0].Acceptable {
					g.BothFirstCorrect++
				}
				if x.Attempts[0].Acceptable || y.Attempts[0].Acceptable {
					g.EitherFirstCorrect++
				}
			}
		}
	}
	eventCount := auditEvents(root, records)
	names := []string{"sources.jsonl", "training.json", "searches.jsonl", "events.jsonl", "report.json", "split-policy.json", "started.json", "completed.json"}
	for _, model := range []string{"joint", "comparison", "branches"} {
		names = append(names, "model-"+model+".json", "model-info-"+model+".json", "history-"+model+".json", model+"-fit-started.json")
	}
	for _, name := range names {
		identities[name] = hash(root, name)
	}
	latencies := map[string]latency{}
	for name, ns := range timings {
		latencies[name] = distribution(ns)
	}
	out := map[string]any{"schema": "gooo/canonical-role-audit/v1", "source_count": len(sources), "search_count": len(searches), "fits": 3, "event_count": eventCount, "new_fits": 0, "new_predictions": 0, "new_candidate_executions": 0, "groups": groups, "split_compared_with_joint": changes, "goal_pairs": paired, "equivalent_form_pairs": equivalent, "prediction": latencies, "files": identities, "scope": "retained-record validation; goal pairings overlap; no independent oracle execution or causal attribution"}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	must(enc.Encode(out))
}
