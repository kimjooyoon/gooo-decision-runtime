// Replay the fixed full-input study using the public SDK, without model training.
package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/jointdecision"
)

const manifestSHA = "ce4ad854c0d7fcb3e7fa049658f40ea4b0393a9bff39a2ef21f64d83f221bc3a"

var arms = []string{"positioned-original", "positioned-varied", "bag-original", "bag-varied"}
var variants = []string{"fp32", "ptq_ternary", "qat_ternary"}
var forms = []string{"original", "task-prefix", "complete-suffix"}

type pin struct {
	SHA   string `json:"sha256"`
	Bytes int64  `json:"bytes"`
}
type observation struct {
	Features   string                        `json:"feature_bits_sha256"`
	HiddenSHA  string                        `json:"hidden_bits_sha256"`
	LogitsSHA  string                        `json:"logit_bits_sha256"`
	ProbsSHA   string                        `json:"probability_bits_sha256"`
	Hidden     [24]float32                   `json:"hidden"`
	Prediction jointdecision.ThreePrediction `json:"prediction"`
	Order      [8]int                        `json:"ranked_mask_order"`
}
type frozenRow struct {
	Original struct {
		View  string `json:"view_id"`
		Input string `json:"input_sha256"`
		Text  string `json:"complete_input"`
	} `json:"original"`
	Lanes [4]observation `json:"lanes"`
}
type report struct {
	Schema        string         `json:"schema"`
	Status        string         `json:"status"`
	Source        string         `json:"sdk_source_revision"`
	Provenance    pin            `json:"sdk_extraction_manifest"`
	Manifest      string         `json:"reference_manifest_sha256"`
	Go            string         `json:"go_version"`
	Platform      string         `json:"platform"`
	Rows          int            `json:"complete_input_rows"`
	Calls         int            `json:"actual_model_predictions"`
	Updates       int            `json:"optimizer_updates"`
	Conditions    map[string]int `json:"matched_predictions_by_condition"`
	Models        map[string]pin `json:"model_files"`
	Journals      map[string]pin `json:"frozen_journals"`
	Phase         string         `json:"phase"`
	LastCondition string         `json:"last_condition"`
	LastView      string         `json:"last_view_id"`
	Wall          float64        `json:"wall_seconds"`
	Scope         string         `json:"scope"`
}

func main() {
	bundle := flag.String("bundle", "", "closed full-input-separate-arithmetic-20261003 bundle")
	source := flag.String("source-revision", "", "exact clean SDK source")
	output := flag.String("output", "", "fresh result JSON")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected arguments")
		os.Exit(2)
	}
	if err := replay(*bundle, *source, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(`{"status":"PASS","rows":18432,"actual_predictions":36864}`)
}

func replay(root, source, output string) (failure error) {
	if root == "" || output == "" || runtime.Version() != "go1.27.1" {
		return errors.New("bundle, fresh output and Go 1.27.1 required")
	}
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		return errors.New("fresh output required")
	}
	head, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil || len(source) != 40 || strings.TrimSpace(string(head)) != source {
		return errors.New("exact SDK source required")
	}
	dirty, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil || len(dirty) != 0 {
		return errors.New("clean SDK source required")
	}
	m, err := openBundle(root)
	if err != nil {
		return err
	}
	defer m.archive.Close()
	provenance, err := filePin("source-provenance.json")
	if err != nil {
		return err
	}
	r := report{Schema: "gooo/sdk-full-input-replay/v1", Status: "FAILED", Source: source, Provenance: provenance, Manifest: manifestSHA,
		Go: runtime.Version(), Platform: runtime.GOOS + "/" + runtime.GOARCH, Conditions: map[string]int{}, Models: map[string]pin{}, Journals: map[string]pin{},
		Scope: "Complete frozen explicit-arithmetic lanes replayed by the SDK. Exact feature/hidden/logit/probability bits and full rankings; known authored development inputs; zero training or native generation. Synthetic unit predictions are separate."}
	started := time.Now()
	defer func() {
		r.Wall = time.Since(started).Seconds()
		raw, e := json.MarshalIndent(r, "", "  ")
		if e == nil {
			var f *os.File
			f, e = os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
			if e == nil {
				_, e = f.Write(append(raw, '\n'))
				e = errors.Join(e, f.Close())
			}
		}
		failure = errors.Join(failure, e)
	}()
	for _, arm := range arms {
		for _, variant := range variants {
			r.Phase, r.LastCondition = "model_load", arm+"/"+variant
			var models [2]*jointdecision.ThreeModel
			for i, layout := range []string{"expanded", "compact"} {
				dir := filepath.Join("models", layout, arm, variant)
				for _, name := range []string{"model.json", "weights.bin"} {
					rel := filepath.ToSlash(filepath.Join(dir, name))
					want, ok := m.files[rel]
					if !ok || checkPin(filepath.Join(root, rel), want) != nil {
						return errors.New("fixed model bytes differ")
					}
					r.Models[rel] = want
				}
				loader := jointdecision.LoadThree
				if strings.HasPrefix(arm, "bag-") {
					loader = jointdecision.LoadThreeBag
				}
				if i == 1 {
					loader = jointdecision.LoadSharedThree
				}
				models[i], err = loader(filepath.Join(root, dir, "model.json"))
				if err != nil || models[i].ArithmeticVersion() != jointdecision.SeparateArithmeticVersion {
					return errors.New("explicit model load failed")
				}
			}
			for _, form := range forms {
				name := arm + "--" + variant + "--" + form + ".jsonl.gz"
				r.Phase, r.LastCondition = "journal_replay", name
				r.Journals[name] = m.members[name]
				seen := map[string]bool{}
				err = m.visit(name, func(row frozenRow) error {
					r.LastView = row.Original.View
					if seen[row.Original.View] || row.Original.View == "" || sha([]byte(row.Original.Text)) != row.Original.Input {
						return errors.New("duplicate or invalid complete input")
					}
					seen[row.Original.View] = true
					for lane, model := range models {
						var w jointdecision.ThreeWorkspace
						var p jointdecision.ThreePrediction
						r.Calls++
						if err := model.PredictInto(row.Original.Text, &w, &p); err != nil {
							return errors.New("SDK prediction failed")
						}
						if !matches(w, p, row.Lanes[lane+2]) {
							return errors.New("SDK prediction differs from frozen explicit lane")
						}
						r.Conditions[name]++
					}
					r.Rows++
					return nil
				})
				if err != nil {
					return err
				}
			}
		}
	}
	if r.Rows != 18432 || r.Calls != 36864 || len(r.Conditions) != 36 || len(r.Models) != 48 {
		return errors.New("complete replay denominator differs")
	}
	r.Status, r.Phase = "PASS", "complete"
	return nil
}

func bitsSHA(values []float32) string {
	var raw [768 * 4]byte
	for i, v := range values {
		binary.LittleEndian.PutUint32(raw[4*i:], math.Float32bits(v))
	}
	return sha(raw[:4*len(values)])
}
func ranking(p jointdecision.ThreePrediction) [8]int {
	x := [8]int{0, 1, 2, 3, 4, 5, 6, 7}
	sort.Slice(x[:], func(i, j int) bool {
		a, b := x[i], x[j]
		return p.Probabilities[a] > p.Probabilities[b] || p.Probabilities[a] == p.Probabilities[b] && a < b
	})
	return x
}
func matches(w jointdecision.ThreeWorkspace, p jointdecision.ThreePrediction, o observation) bool {
	return bitsSHA(w.Features[:]) == o.Features && bitsSHA(w.Hidden[:]) == o.HiddenSHA && bitsSHA(o.Hidden[:]) == o.HiddenSHA &&
		bitsSHA(p.Logits[:]) == o.LogitsSHA && bitsSHA(o.Prediction.Logits[:]) == o.LogitsSHA &&
		bitsSHA(p.Probabilities[:]) == o.ProbsSHA && bitsSHA(o.Prediction.Probabilities[:]) == o.ProbsSHA &&
		p == o.Prediction && ranking(p) == o.Order && int(p.Mask) == o.Order[0]
}
