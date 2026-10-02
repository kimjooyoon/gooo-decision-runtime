// A finite order ambiguity, resolved by one observation from a declared oracle.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

func run() error {
	p, err := pathplan.Prepare(pathplan.Plan{Schema: pathplan.Schema,
		Base: bodyplan.Plan{Schema: bodyplan.Schema, ID: "order-probe", Name: "Adjust", ResultType: decision.TypeInt,
			Expressions: []bodyplan.Expr{{Kind: "input", Name: "input"}, {Kind: "int", Int: 2},
				{Kind: "binary", Operation: "subtract", Left: 0, Right: 1}},
			Statements: []bodyplan.Stmt{{Kind: "return", Expr: 2}}, Root: []int{0}},
		Decisions: []pathplan.Choice{{ID: "order", Kind: pathplan.OperandOrder, Target: 2,
			Intent: "Subtract the input from two. / 2에서 입력을 뺀다.", Fallback: "forward",
			Options: []pathplan.Option{{Label: "forward"}, {Label: "reverse", Reverse: true}}}},
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cases, probes := []pathplan.TestCase{{Input: 2, Expected: 0}}, []int64{2, 3, 0}
	before, err := p.RankProbes(ctx, cases, probes, 2)
	if err != nil {
		return err
	}
	if before.RecommendedIndex == nil {
		return fmt.Errorf("expected an informative probe")
	}
	x := before.Probes[*before.RecommendedIndex].Input
	// This explicit reference function supplies the expected value. A model's
	// proposal and the competing candidates cannot supply their own truth label.
	oracle := func(input int64) int64 { return 2 - input }
	observation := pathplan.TestCase{Input: x, Expected: oracle(x)}
	after, err := p.RankProbes(ctx, append(cases, observation), probes, 2)
	if err != nil {
		return err
	}
	if len(after.SurvivingMasks) != 1 || after.SurvivingMasks[0] != 1 {
		return fmt.Errorf("order remains unresolved")
	}
	body, err := p.Compile(map[string]string{"order": "reverse"})
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Before      pathplan.ProbeRanking `json:"before"`
		Observation pathplan.TestCase     `json:"oracle_observation"`
		After       pathplan.ProbeRanking `json:"after"`
		Source      string                `json:"gooo_source"`
	}{before, observation, after, body.GoooSource()})
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
