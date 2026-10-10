package pathplan

import (
	"sync"
	"testing"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
)

func TestLiteralContractOwnsCanonicalSourceAndAllCases(t *testing.T) {
	plan := executionMaxPlan()
	plan.Base.Expressions = append(plan.Base.Expressions, bodyplan.Expr{Kind: "int", Int: 10}, bodyplan.Expr{Kind: "int", Int: -3})
	plan.Base.Statements = append(plan.Base.Statements, bodyplan.Stmt{Kind: "let", Name: "same", Expr: 3}, bodyplan.Stmt{Kind: "let", Name: "other", Expr: 4})
	plan.Base.Root = []int{3, 4, 2}
	prepared, err := Prepare(plan)
	if err != nil {
		t.Fatal(err)
	}
	cases := make([]TestCase, 128)
	for i := range cases {
		cases[i] = TestCase{Input: int64(i), Expected: 9007199254740993 + int64(i)}
	}
	input, err := prepared.InitialContractInputFor(cases, decision.SourceLiteralCaseFeatureVersion)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := prepared.InitialContractInput(cases)
	if err != nil {
		t.Fatal(err)
	}
	if input.PlanSHA256() != legacy.PlanSHA256() || input.CaseSHA256() != legacy.CaseSHA256() || input.CaseCount() != 128 || input.CaseFeatureVersion() != decision.SourceLiteralCaseFeatureVersion {
		t.Fatal("binding")
	}
	var literals [128]int64
	count, err := input.SourceLiteralsInto(&literals)
	if err != nil || count != 2 || literals[0] != -3 || literals[1] != 10 {
		t.Fatal(count, literals, err)
	}
	for i := range plan.Base.Expressions {
		plan.Base.Expressions[i].Int = 999
	}
	owned := append([]TestCase(nil), cases...)
	clear(cases)
	literals[0] = 999
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for i, c := range owned {
				var want, got [32]float32
				_ = decision.SourceLiteralCaseFeaturesInto(c.Input, c.Expected, []int64{-3, 10}, &want)
				if input.CaseFeaturesInto(i, &got) != nil || got != want {
					t.Error("source/case alias or truncation", i)
				}
			}
		})
	}
	wg.Wait()
	if _, err := prepared.InitialContractInputFor(owned, "unknown"); err == nil {
		t.Fatal("unknown ABI")
	}
	if _, err := legacy.SourceLiteralsInto(&literals); err == nil {
		t.Fatal("legacy input claimed a literal profile")
	}
}

func TestLiteralModelImmediatelyRanksAndChecksBothDeclaredGoals(t *testing.T) {
	ctx := sessionContext(t)
	p, err := Prepare(pairedContractPlan())
	if err != nil {
		t.Fatal(err)
	}
	model, err := contractdecision.NewForCaseFeatures([contractdecision.ParameterCount]float32{}, contractdecision.ExtremePooling, decision.SourceLiteralCaseFeatureVersion)
	if err != nil {
		t.Fatal(err)
	}
	for _, cases := range [][]TestCase{
		{{-9007199254740995, 10}, {9007199254740993, 9007199254740993}},
		{{-9007199254740995, -9007199254740995}, {9007199254740993, 10}},
	} {
		session, err := p.NewContractSession(ctx, model, cases)
		if err != nil {
			t.Fatal(err)
		}
		rank := session.Ranking()
		if rank.Calls != 1 || !rank.Applied || rank.CaseFeatures != decision.SourceLiteralCaseFeatureVersion {
			t.Fatal(rank)
		}
		progress, body, err := session.Advance(ctx, 2)
		if err != nil || body == nil || progress.Status != "TRAINING_COMPLETE" || progress.SelectedPassed != 2 || progress.FeedbackPredictions != 0 {
			t.Fatal(progress, err)
		}
		for _, c := range cases {
			value, err := body.Evaluate(c.Input)
			if err != nil || value.Int != c.Expected {
				t.Fatal(value, err)
			}
		}
	}
}
