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
128 bytes of caller-owned scratch. `CaseFeatures` also returns the same row by
value for streaming model readers. No suite sampling, fixed four-case prefix, hidden truncation or
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

## Model and execution connection

The separate [`contractdecision`](../contractdecision/README.md) package now
trains a shared 32→8 case encoder and a source/pooled-case→24→2 ranking network.
`PreparedPlan.NewContractSession` invokes it before candidate execution, then
uses the existing complete finite checks. Nil preserves fallback search.
The local `examples/contract-model` command fits extracted training documents
and runs new documents with or without the model.

Existing published v6 weights retain their original ABI. The new model requires
fresh training; its 9,746 parameters occupy 38,984 FP32 bytes. Current paired
learning tests establish the connection. A separate
[196-document study](../studies/contract-goals-20261010/README.md) now records
held-out finite results and local costs, including regressions. The ordinary
compiler CLI route remains work. All cases reach the encoder, but pooling is
lossy and cannot replace verification of the original suite.
