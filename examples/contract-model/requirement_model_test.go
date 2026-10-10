package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/contractdecision"
	"github.com/kimjooyoon/gooo-decision-runtime/pathplan"
)

func TestRequirementModelLearnsAndImmediatelyConstructsConditionGoals(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	dir := t.TempDir()
	a := saveDocument(t, dir, "true.json", conditionOnlyGoal(true))
	b := saveDocument(t, dir, "false.json", conditionOnlyGoal(false))
	model := filepath.Join(dir, "requirements.json")
	var out bytes.Buffer
	if err := run(ctx, []string{"fit", "-requirements", "-epochs", "2000", "-out", model, a, b}, &out); err != nil {
		t.Fatal(err)
	}
	var receipt struct {
		Schema            string                       `json:"schema"`
		ConditionFeatures string                       `json:"condition_feature_version"`
		Audit             *contractdecision.InputAudit `json:"input_audit"`
	}
	if err := json.Unmarshal(out.Bytes(), &receipt); err != nil || receipt.Schema != "gooo/requirement-conditioned-local-fit/v1" ||
		receipt.ConditionFeatures != decision.DeclaredConditionFeatureVersion || receipt.Audit == nil ||
		receipt.Audit.Schema != contractdecision.RequirementInputAuditSchema || receipt.Audit.BestFirstPasses != 2 ||
		receipt.Audit.UnavoidableMisses != 0 || len(receipt.Audit.Groups) != 2 {
		t.Fatal("fit described the old incomplete model input", out.String(), err)
	}
	var rankings [2]pathplan.ContractRanking
	for i, path := range []string{a, b} {
		out.Reset()
		if err := run(ctx, []string{"search", "-model", model, path}, &out); err != nil {
			t.Fatal(err, out.String())
		}
		var result struct {
			Ranking pathplan.ContractRanking  `json:"ranking"`
			Initial pathplan.ContractProgress `json:"initial"`
			Result  pathplan.ContractProgress `json:"result"`
			Body    string                    `json:"selected_go_body"`
		}
		if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.Ranking.Schema != "gooo/requirement-path-ranking/v1" ||
			result.Ranking.Calls != 1 || result.Initial.Attempted != 0 || result.Result.Attempted != 1 ||
			result.Result.Status != "TRAINING_COMPLETE" || result.Result.SelectedPassed != 2 || result.Body == "" ||
			result.Ranking.Proposed != uint16(i) || result.Ranking.ConditionCount != 1 ||
			result.Ranking.ConditionFeatures != decision.DeclaredConditionFeatureVersion || result.Ranking.ConditionFeatureSHA == "" {
			t.Fatal("learned condition did not immediately construct a checked body", i, out.String(), err)
		}
		rankings[i] = result.Ranking
	}
	if rankings[0].ConditionFeatureSHA == rankings[1].ConditionFeatureSHA || rankings[0].CaseSHA != rankings[1].CaseSHA ||
		rankings[0].FeatureSHA != rankings[1].FeatureSHA {
		t.Fatal("condition input identity mixed with unchanged channels")
	}
	for _, flags := range [][]string{{"-choice-context"}, {"-goal-pairs"}, {"-case-features", decision.SourceLiteralCaseFeatureVersion}} {
		args := append([]string{"fit", "-requirements", "-out", filepath.Join(dir, "unused.json")}, flags...)
		args = append(args, a, b)
		out.Reset()
		if err := run(ctx, args, &out); err == nil || out.Len() != 0 {
			t.Fatal("incompatible computation options accepted", args, err)
		}
	}
}
