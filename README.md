# Gooo Decision Runtime

This module is a small Go-only inference and typed body runtime. It exposes
the operation classifier, a separate structural model contract, typed binary IR
and bounded body/path assembly. Source/tests are pinned in
[source-provenance.json](source-provenance.json), with original and relocated
byte digests. The earlier extraction manifest is preserved.
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

## Structural paths

`LoadPath` accepts the distinct structural model ABI and versioned positioned
intent features. Operation `Load`/`Decide` cannot reinterpret it. The
[typed-path models](https://huggingface.co/asketeddy/gooo-typed-path-tiny-v1)
are supplied separately; no weights are bundled in this module.

`bodyplan` exposes typed expression/statement arenas. `pathplan` supports local
references, assignment targets, operand order, branch layout and execution order,
including interacting choices. Offered alternatives and complete combinations
are type/scope checked. Identifiers and body fragments are compiler-owned.

`pathplan.DecodeDocument` reads strict bounded JSON with explicit integer cases.
The caller must bind the fallback base to authoritative source before accepting
body edits. `Program.GoooBody()` returns only the selected body so existing
packages, declarations and stable activity identity can be retained.

`pathplan.Search` requires a deadline and candidate budget. One prediction per
decision ranks eligible paths before finite tests. Partial results retain passing
cases and unattempted combinations. Model confidence is recorded separately from
TDD acceptance. Omitting the model uses deterministic declared order, zero
prediction calls and no network. Validation/rendering allocate separately from
the fixed-array prediction kernel.

## Reuse validated paths

`pathplan.Prepare(plan)` or `document.Prepare()` returns an owned immutable
snapshot with `Fallback()`, `Defaults()`, `ActivityName()` and `PlanSHA256()`.
Bind its cached fallback to the original source, then call `prepared.Search`
with a deadline, model and finite cases. Each complete candidate still passes
the arena compiler's combined type/scope checks. `prepared.Compile` validates
explicit complete selections without repeating individual-option preparation.

Preparing requires synchronized input; later input mutation cannot change the
snapshot. Concurrent searches share only read-only prepared state, each owning
its workspace, choices and heap. The snapshot caches no model or test outcomes;
returned default maps and receipts are independent. There is no global cache.
The existing `Search` signature remains available and prepares internally.

The previous 16-file extraction is retained as `source-provenance-v2.json`;
that revision's fixed extraction pins 18 original/relocated source and test files.

## Incremental finite path sessions

The additive `v0.2.2-experimental` API starts one model ranking and then advances
only new declared masks. `prepared.NewSession(ctx, model, cases, seed)` owns the
fixed cases; `Observe()` keeps initialization call accounting even after an
interrupted start. `Advance(ctx, 1..64)` returns caller-owned progress and the
current best typed body. `SearchBatches` retains the existing 1..64 total native
contract; direct advances can traverse the full finite 16-bit declared space.

Scheduled masks use a uint64 bitset (8 bytes for 64 paths, at most 8 KiB for
65,536 paths). Old attempt logs are returned to the caller and are not retained
inside a session. Frontier capacity, prepared body, models, maps and runtime
memory are separate from that bitset. Concurrent calls fail immediately with
`ErrSessionBusy`; canceled unfinished candidates return to the frontier. Model
ranking occurs during initialization only; advance makes zero new predictions.
Without a model the same inputs and bounds have deterministic ordering.

Progress/case/plan/model hashes are observational lineage, not permission to
change authoritative source. Callers must bind the original Gooo source to the
prepared fallback and verify final native emission. No serialized session state
is accepted as executable authority. Input/case/model changes start a new
session. The existing `Search` API and eight-label model ABI are unchanged.

This revision extracts exactly 20 files from public research source
`733d67857ce2c7569db54d9b42255184186a90e2`; the previous 18-file manifest remains
in `source-provenance-v3.json`. All import transformations and byte digests are
recorded. Source-level/race tests cover legacy-equivalent ordering and bodies,
call/input/output ownership, cancellation, initialization call accounting,
nonblocking concurrency, 128 unique masks over two calls and the maximum mask.
These are contract fixtures, not trained-model benchmark results.
