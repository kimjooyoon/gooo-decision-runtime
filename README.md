# Gooo Decision Runtime

This module is a small Go-only inference and typed-operation runtime. It exposes
the closed eight-operation classifier and the typed binary IR builder from the
production implementation pinned in [source-provenance.json](source-provenance.json).
It has no third-party module dependencies and does not include model weights,
training data, Python tooling, or cached runtime files.

The exported operation labels are `add`, `subtract`, `multiply`, `less_than`,
`less_equal`, `equal`, `and`, and `or`. The model emits one of these labels; the
Go package maps that label into validated `TypedBinaryIR`. It accepts bounded
instruction text, not source code or arbitrary expressions. A low-confidence
prediction is reported as abstained.

The published [tiny model bundles](https://huggingface.co/asketeddy/gooo-ir-operator-tiny-v1)
were independently initialized and are not fine-tuned from Laya. This repository
contains runtime code only; callers choose and supply a compatible model bundle.

## Load and predict

Import `github.com/kimjooyoon/gooo-decision-runtime` as package `decision`.
The module includes the runtime source and tests, without the parent research
repository's raw experiment records. Model files are loaded separately.

```go
model, err := decision.Load("model.json")
if err != nil {
	return err
}

var workspace decision.Workspace
var prediction decision.Prediction
if err := model.PredictInto("Add the quantity to the balance", &workspace, &prediction); err != nil {
	return err
}
if prediction.Abstained {
	// Handle low confidence without emitting an operation.
}
fmt.Println(model.PredictLabel(&prediction))
```

For typed IR, use `Model.Decide` with a `DecisionRequest` whose schema is
`DecisionRequestSchema` and whose operands have explicit `Int` or `Bool` types.
The bridge validates the operation against the operand types and only emits a
closed `TypedBinaryIR` value.

`Model.MetadataSHA256()` identifies the exact metadata bytes read by `Load`,
including their formatting. `Model.WeightsSHA256()` identifies the validated
packed weight bytes. Together they let a caller record the model configuration
and weight artifact used for a loaded model.

## Memory ownership

`Load` reads and verifies the model bundle, then keeps decoded weights for the
model's lifetime. Treat a loaded `Model` as shared read-only state. Callers own
the model lifetime and should allocate one `Workspace` per concurrent worker;
workspace storage must not be shared by concurrent predictions. A workspace
uses 1,248 bytes for its fixed arrays. The `Prediction` result is 80 bytes on
the measured 64-bit targets; `PredictionBytes()` reports the current ABI size.
`PredictInto` is a bounded synchronous kernel with no context parameter; a
worker can check cancellation before and after it. Its measured zero-allocation
hot path does not apply to `Decide`, which constructs response values. Supply a
stable caller-owned model directory: loading is not an atomic snapshot of files
being replaced concurrently.

Packed weight-file bytes describe storage on disk. Resident tensor bytes
describe decoded matrices and biases in memory. For the bundled formats, FP32
uses 50,912 packed and resident tensor bytes. Ternary matrices use 2,759 packed
weight-file bytes and 12,896 resident tensor bytes, plus 8 bytes for the two
matrix scales. These figures exclude model metadata, Go object headers, caller
workspaces, output values, allocator/runtime overhead, and process RSS. The
ternary encoding is compact on disk; it is not bit-packed in resident memory,
where matrix values are decoded to `int8`.

## Checks

The CI workflow uses Go 1.27.1 for formatting, vet, unit tests, and race tests.
Tests construct deterministic tiny bundles in temporary directories. They do
not download weights, invoke training, or call a model service. The source files
copied into this SDK are byte-pinned in `source-provenance.json`.

This repository is experimental. The classifier is a closed eight-label
decision component, not a general-purpose natural-language-to-code compiler.
