package main

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kimjooyoon/gooo-decision-runtime/conditiondecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
	"github.com/kimjooyoon/meta-ontology-go/internal/bodycodegen"
)

type candidate struct {
	Mask   uint16                `json:"mask"`
	Body   string                `json:"gooo_body"`
	Cases  []pathplan.TestResult `json:"cases"`
	Passed int                   `json:"passed"`
}

type row struct {
	ID               string                       `json:"id"`
	Source           string                       `json:"gooo_source"`
	SourceSHA        string                       `json:"source_sha256"`
	Document         pathplan.Document            `json:"document"`
	Features         [256]float32                 `json:"features"`
	FeatureSHA       string                       `json:"feature_sha256"`
	Candidates       []candidate                  `json:"candidates"`
	Valid            []uint16                     `json:"valid_masks"`
	Prediction       conditiondecision.Prediction `json:"prediction"`
	PredictNS        int64                        `json:"predict_ns"`
	PredictionPassed bool                         `json:"prediction_passed"`
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func hash(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }

func source(reverse bool) string {
	body := "if input < 10 { return 10 } else { return input }"
	if reverse {
		body = "if input < 10 { return input } else { return 10 }"
	}
	var s strings.Builder
	fmt.Fprintf(&s, "package branchstudy\nnamespace branchstudy\nentity Integer id \"branchstudy://integer\"\nactivity Choose(Integer) -> Integer computes %s assembling {\n", strconv.Quote(body))
	s.WriteString(" choice \"branches\" branch_layout at \"0\" intent \"Return the larger of the input and ten. 입력과 10 중 큰 값을 반환한다.\"\n")
	for _, x := range []int64{-9007199254740995, -11, 0, 10, 11, 9007199254740993} {
		fmt.Fprintf(&s, " case \"%d\" -> \"%d\"\n", x, max(x, 10))
	}
	s.WriteString(" attempts \"2\"\n}\n")
	return s.String()
}

func inspect(id string, text string, model *conditiondecision.Model) row {
	r := row{ID: id, Source: text, SourceSHA: hash([]byte(text)), Valid: []uint16{}}
	ctx := context.Background()
	document, err := bodycodegen.DecodeSourcePathDocument(ctx, id+".gooo", []byte(text), "Choose", nil)
	must(err)
	r.Document = document
	prepared, err := document.Prepare()
	must(err)
	input, err := prepared.InitialConditionInput()
	must(err)
	must(input.FeaturesInto("branches", &r.Features))
	var raw [1024]byte
	for i, value := range r.Features {
		binary.LittleEndian.PutUint32(raw[i*4:], math.Float32bits(value))
	}
	r.FeatureSHA = hash(raw[:])
	for mask := range 2 {
		choice := document.Plan.Decisions[0]
		program, err := prepared.Compile(map[string]string{"branches": choice.Options[mask].Label})
		must(err)
		c := candidate{Mask: uint16(mask), Body: program.GoooBody()}
		for _, test := range document.TestCases {
			actual, err := program.Evaluate(test.Input)
			must(err)
			passed := actual.Int == test.Expected
			c.Cases = append(c.Cases, pathplan.TestResult{Input: test.Input, Expected: test.Expected, Actual: actual.Int, Passed: passed})
			if passed {
				c.Passed++
			}
		}
		if c.Passed == len(document.TestCases) {
			r.Valid = append(r.Valid, uint16(mask))
		}
		r.Candidates = append(r.Candidates, c)
	}
	var workspace conditiondecision.Workspace
	start := time.Now()
	must(model.PredictInto([][256]float32{r.Features}, []uint16{0, 1}, &workspace, &r.Prediction))
	r.PredictNS = time.Since(start).Nanoseconds()
	for _, valid := range r.Valid {
		if valid == r.Prediction.Selected {
			r.PredictionPassed = true
		}
	}
	return r
}

func main() {
	if len(os.Args) != 3 {
		panic("model output-file")
	}
	f, err := os.OpenFile(os.Args[2], os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	must(err)
	defer f.Close()
	raw, err := os.ReadFile(os.Args[1])
	must(err)
	if hash(raw) != "a16696ed44c668f38cc2e7ee1dc6ff3f4716649d57e59df7c9490d1c670edc48" {
		panic("model identity")
	}
	model, err := conditiondecision.Decode(raw)
	must(err)
	rows := []row{inspect("max-forward", source(false), model), inspect("max-reversed", source(true), model)}
	same := rows[0].Features == rows[1].Features
	disjoint := len(rows[0].Valid) > 0 && len(rows[1].Valid) > 0
	for _, a := range rows[0].Valid {
		for _, b := range rows[1].Valid {
			if a == b {
				disjoint = false
			}
		}
	}
	out := struct {
		Schema     string `json:"schema"`
		ModelSHA   string `json:"model_sha256"`
		ModelCalls int    `json:"model_calls"`
		NativeRuns int    `json:"native_runs"`
		Rows       []row  `json:"rows"`
		SameInput  bool   `json:"identical_initial_features"`
		Disjoint   bool   `json:"disjoint_valid_masks"`
	}{"gooo/branch-feature-discrimination-study/v1", hash(raw), 2, 0, rows, same, disjoint}
	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	must(e.Encode(out))
}
