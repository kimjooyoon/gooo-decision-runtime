# Give the decision model the declared requirements

Gooo already knows the input/expected-output cases written by the source author.
Existing v6 models receive source structure and natural-language intent first;
expected and actual output values enter their input only after an execution
failure. Two different output contracts on the same source plan therefore have
identical initial v6 arrays.

`PreparedPlan.InitialContractInput` exposes these declared requirements in a
separate explicit input channel for the next model. It retains the complete
1..128-case suite, its digest, the source plan identity and the unchanged initial
v6 projection. No candidate is executed and no prediction is made.

```go
input, err := prepared.InitialContractInput(document.TestCases)
if err != nil {
    return err
}
var source [decision.ExecutionFlowFeatureDim]float32
if err := input.RelationalSourceFeaturesInto(choiceID, &source); err != nil {
    return err
}
var row [decision.DeclaredCaseFeatureDim]float32
for i := 0; i < input.CaseCount(); i++ {
    if err := input.CaseFeaturesInto(i, &row); err != nil {
        return err
    }
    // A dedicated model can consume each declared case using shared weights.
}
```

The compiler remains responsible for binding the supplied cases to Gooo source.
Declared expectations are goals; an observed failure additionally needs an
executed candidate and its actual output. The two channels have different
meaning and neither can replace the other.

## Small, complete storage

The input owns 128 slots of two `int64` values: 2,048 bytes for the case payload,
plus its count and source/input metadata. It projects one 32-cell FP32 row into
128 bytes of caller-owned scratch. A future model can stream every case through
shared weights. No suite sampling, fixed four-case prefix, hidden truncation or
whole-suite floating-point cache is used. Order, duplicates and contradictory
declarations are preserved for the owning verifier.

`declared_integer_case_v1` uses fixed binary fractions:

| Cells | Meaning |
| --- | --- |
| 0 | Declared case present |
| 1–6 | Input and expected-value signs |
| 7–9 | Expected equals / below / above input |
| 10–11 | Input and expected-value parity |
| 12–19 | Exact input bytes, most significant first |
| 20–27 | Exact expected-value bytes, most significant first |
| 28–30 | Expected is the negation / successor / predecessor of input |
| 31 | Reserved zero |

Flags use 1/8; each byte uses its value divided by 2,048. Integers are converted
to bytes before floating-point encoding. All bits survive for values beyond
2^53 and both int64 endpoints; arithmetic relation flags avoid overflow.

## Current boundary and next model

This is an implemented source/case input API. Existing model weights and
selection APIs retain their original ABI and behavior. The published v6 model
does not consume this new channel. Training and compiler routing for a dedicated
contract-aware model remain work.

The intended local design uses shared case weights and a small fixed-size
aggregate combined with each source choice. Case count can grow to the existing
128-case limit without increasing the number of learned parameters. Evaluation
must include paired contracts with the same source/intent but different expected
behavior, held-out expressions and wording, and comparisons with deterministic
candidate checking. Report initial selection, actual program completeness and
observation cost separately.
