package pathplan

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	stdhash "hash"

	"github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
)

// DistinguishingInput compares two candidate outputs. Neither output is an
// expected answer: a witness does not grant authority to change source intent.
type DistinguishingInput struct {
	Input       int64 `json:"input"`
	Reference   int64 `json:"reference_output"`
	Alternative int64 `json:"alternative_output"`
}

type CandidateDiagnosis struct {
	Mask                  uint16               `json:"choice_mask"`
	Status                string               `json:"status"`
	GoooSHA256            string               `json:"gooo_source_sha256,omitempty"`
	Passed                int                  `json:"finite_cases_passed"`
	CaseOutputsSHA256     string               `json:"case_outputs_sha256,omitempty"`
	ProbeOutputsSHA256    string               `json:"probe_outputs_sha256,omitempty"`
	CaseIndistinguishable bool                 `json:"case_indistinguishable"`
	Witness               *DistinguishingInput `json:"distinguishing_input,omitempty"`
}

// Diagnosis describes an observed finite space relative to one selected body.
// Counts include the reference itself. Probe agreement is bounded evidence,
// never an all-input equivalence proof or natural-language correctness claim.
type Diagnosis struct {
	Schema                string               `json:"schema"`
	Status                string               `json:"status"`
	PlanSHA256            string               `json:"plan_sha256"`
	CasesSHA256           string               `json:"cases_sha256"`
	ProbesSHA256          string               `json:"probe_inputs_sha256"`
	ReferenceMask         uint16               `json:"reference_mask"`
	Declared              int                  `json:"declared_combinations"`
	Observed              int                  `json:"observed_combinations"`
	Unobserved            int                  `json:"unobserved_combinations"`
	Typed                 int                  `json:"typed_candidates"`
	TypeRejected          int                  `json:"type_rejected_candidates"`
	CaseIndistinguishable int                  `json:"case_indistinguishable_candidates"`
	ProbeDistinguished    int                  `json:"probe_distinguished_candidates"`
	ProbeUnresolved       int                  `json:"probe_unresolved_candidates"`
	FiniteCases           int                  `json:"finite_cases"`
	ProbeInputs           int                  `json:"probe_inputs"`
	ModelPredictions      int                  `json:"model_predictions"`
	Candidates            []CandidateDiagnosis `json:"candidates"`
}

type diagnosisReference struct {
	cases  [128]int64
	probes [32]int64
}

func diagnosisBounds(ctx context.Context, prepared *PreparedPlan, cases []TestCase, probes []int64, budget int) error {
	if ctx == nil {
		return errors.New("diagnosis context is required")
	}
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("diagnosis context must have a deadline")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if prepared == nil || prepared.fallback == nil || prepared.plan.Base.ResultType != decision.TypeInt {
		return errors.New("prepared integer path plan is required")
	}
	if len(cases) == 0 || len(cases) > 128 || len(probes) == 0 || len(probes) > 32 || budget < 1 || budget > 64 {
		return errors.New("diagnosis finite budgets are invalid")
	}
	return nil
}

func diagnosisMask(plan Plan, choices map[string]string) (uint16, error) {
	if len(choices) != len(plan.Decisions) {
		return 0, errors.New("diagnosis requires every declared reference choice")
	}
	var mask uint16
	for i, choice := range plan.Decisions {
		if choices[choice.ID] == choice.Options[1].Label {
			mask |= 1 << i
		} else if choices[choice.ID] != choice.Options[0].Label {
			return 0, errors.New("diagnosis reference contains an undeclared choice")
		}
	}
	return mask, nil
}

func diagnosisChoices(plan Plan, mask uint16) map[string]string {
	choices := make(map[string]string, len(plan.Decisions))
	for i, choice := range plan.Decisions {
		choices[choice.ID] = choice.Options[int(mask>>i&1)].Label
	}
	return choices
}

func diagnosisValue(ctx context.Context, program *bodyplan.Program, input int64) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	value, err := program.Evaluate(input)
	if err != nil || value.Type != decision.TypeInt {
		return 0, errors.New("diagnosis integer evaluation failed")
	}
	return value.Int, ctx.Err()
}

func diagnosisReferenceValues(ctx context.Context, program *bodyplan.Program, cases []TestCase, probes []int64) (diagnosisReference, error) {
	var reference diagnosisReference
	for i, test := range cases {
		value, err := diagnosisValue(ctx, program, test.Input)
		if err != nil {
			return reference, err
		}
		reference.cases[i] = value
	}
	for i, input := range probes {
		value, err := diagnosisValue(ctx, program, input)
		if err != nil {
			return reference, err
		}
		reference.probes[i] = value
	}
	return reference, nil
}

func diagnosisPairDigest(digest stdhash.Hash, input, output int64) {
	var raw [16]byte
	binary.LittleEndian.PutUint64(raw[:8], uint64(input))
	binary.LittleEndian.PutUint64(raw[8:], uint64(output))
	_, _ = digest.Write(raw[:])
}

func diagnoseCandidate(ctx context.Context, program *bodyplan.Program, mask uint16, reference diagnosisReference, cases []TestCase, probes []int64) (CandidateDiagnosis, error) {
	row := CandidateDiagnosis{Mask: mask, Status: "EVALUATED", GoooSHA256: hashBytes([]byte(program.GoooSource())), CaseIndistinguishable: true}
	caseDigest, probeDigest := sha256.New(), sha256.New()
	for i, test := range cases {
		value, err := diagnosisValue(ctx, program, test.Input)
		if err != nil {
			return CandidateDiagnosis{}, err
		}
		if value == test.Expected {
			row.Passed++
		}
		row.CaseIndistinguishable = row.CaseIndistinguishable && value == reference.cases[i]
		diagnosisPairDigest(caseDigest, test.Input, value)
	}
	for i, input := range probes {
		value, err := diagnosisValue(ctx, program, input)
		if err != nil {
			return CandidateDiagnosis{}, err
		}
		if row.CaseIndistinguishable && row.Witness == nil && value != reference.probes[i] {
			row.Witness = &DistinguishingInput{input, reference.probes[i], value}
		}
		diagnosisPairDigest(probeDigest, input, value)
	}
	row.CaseOutputsSHA256 = hex.EncodeToString(caseDigest.Sum(nil))
	row.ProbeOutputsSHA256 = hex.EncodeToString(probeDigest.Sum(nil))
	return row, nil
}

func hashBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func appendDiagnosis(result *Diagnosis, row CandidateDiagnosis) {
	result.Candidates = append(result.Candidates, row)
	result.Observed++
	result.Unobserved--
	if row.Status == "TYPE_REJECTED" {
		result.TypeRejected++
		return
	}
	result.Typed++
	if row.CaseIndistinguishable {
		result.CaseIndistinguishable++
		if row.Witness == nil {
			result.ProbeUnresolved++
		} else {
			result.ProbeDistinguished++
		}
	}
}

// Diagnose deterministically compares at most 64 candidates with the supplied
// reference. It evaluates the reference first, then ascending masks excluding
// it. Cases, probes and choices are copied after bounds checks; synchronize input
// while calling. It uses no model, modifies no session or source, and retains
// only one reference program plus one candidate, fixed value arrays and receipts.
// An interrupted candidate is unobserved; completed receipts remain available.
func (prepared *PreparedPlan) Diagnose(ctx context.Context, choices map[string]string, cases []TestCase, probes []int64, maxCandidates int) (Diagnosis, error) {
	if err := diagnosisBounds(ctx, prepared, cases, probes, maxCandidates); err != nil {
		return Diagnosis{}, err
	}
	mask, err := diagnosisMask(prepared.plan, choices)
	if err != nil {
		return Diagnosis{}, err
	}
	cases, probes = append([]TestCase(nil), cases...), append([]int64(nil), probes...)
	program, err := prepared.Compile(diagnosisChoices(prepared.plan, mask))
	if err != nil {
		return Diagnosis{}, err
	}
	reference, err := diagnosisReferenceValues(ctx, program, cases, probes)
	if err != nil {
		return Diagnosis{}, err
	}
	caseRaw, _ := json.Marshal(cases)
	probeRaw, _ := json.Marshal(probes)
	result := Diagnosis{Schema: "gooo/typed-path-diagnosis/v1", Status: "PARTIAL", PlanSHA256: prepared.sha,
		CasesSHA256: hashBytes(caseRaw), ProbesSHA256: hashBytes(probeRaw), ReferenceMask: mask,
		Declared: 1 << len(prepared.plan.Decisions), FiniteCases: len(cases), ProbeInputs: len(probes)}
	result.Unobserved = result.Declared
	row, err := diagnoseCandidate(ctx, program, mask, reference, cases, probes)
	if err != nil {
		return result, err
	}
	appendDiagnosis(&result, row)
	for index := 0; index < result.Declared && result.Observed < maxCandidates; index++ {
		if uint16(index) == mask {
			continue
		}
		if err := ctx.Err(); err != nil {
			return result, err
		}
		candidate, compileErr := prepared.Compile(diagnosisChoices(prepared.plan, uint16(index)))
		if err := ctx.Err(); err != nil {
			return result, err
		}
		row := CandidateDiagnosis{Mask: uint16(index), Status: "TYPE_REJECTED"}
		if compileErr == nil {
			row, err = diagnoseCandidate(ctx, candidate, uint16(index), reference, cases, probes)
			if err != nil {
				return result, err
			}
		}
		appendDiagnosis(&result, row)
	}
	if result.Unobserved == 0 {
		result.Status = "COMPLETE"
	}
	return result, nil
}
