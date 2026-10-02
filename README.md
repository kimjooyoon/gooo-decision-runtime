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

## Direct typed source features (v0.2.11-experimental)

Explicit `semantic_context_intent_v3` path models read 64 structural values plus
192 positioned Korean/English byte features. `PreparedPlan.SourceFeatures(id)`
projects a private, validated, fallback-normalized typed plan into a fixed array.
It describes variable references, assignments, operands, branches and root order.
It excludes literal magnitudes, name spellings, intention text and test outcomes.
Unique-local renaming and integer changes preserve the structural array. Ambiguous
duplicate declarations or input shadowing decline this optional representation.

`decision.EncodeSemanticContextInput(fields, intent)` serializes the complete
canonical header and natural intent within 512 UTF-8 bytes. Source projection and
valid feature encoding allocate no heap objects in the contract tests. The feature
array, 1,248-byte workspace and eight-label model dimensions remain unchanged.
Feedback preserves the source header and appends only subsequent observations;
oversized feedback leaves the frontier unchanged and makes zero new predictions.

This corrective release also preserves the previous prediction and workspace
when semantic-v3 input is invalid. V0.2.10's common kernel cleared prediction
output before validating the new codec; this contradicted the frozen v3 error
contract. New actual-kernel tests cover malformed/empty headers, byte overflow,
invalid UTF-8 and invalid feature flags, plus zero-allocation valid prediction.
Old feature versions retain their existing failed-output clearing behavior.

The SDK does not bind an original Gooo file. Native source binding must precede
this projection. No new trained weights or accuracy improvement is claimed by
this ABI release; previous feature versions retain their semantics. See the
[frozen source feature protocol](https://github.com/kimjooyoon/gooo-neural-decision-experiments/blob/7741b41/docs/semantic-context-v3-protocol-20261002.md).

All 33 extracted files are pinned to research revision
`0c540357794028675011c440f203076359ce85c9`, manifest SHA256
`433b4523d2fc582c312b79c20452a014a42b25fff1b30d311500a230a12ebe49`.
The earlier 29-file manifest remains in `source-provenance-v11.json`.
The first v3 33-file extraction remains in `source-provenance-v12.json` and the
v0.2.10 tag; its malformed-input prediction clearing is corrected by v0.2.11.

## Previous Gooo context and bilingual intent codec

`v0.2.9-experimental` adds the exact `split_context_intent_ngrams_v2` feature
contract for explicit structural `LoadPath` models. It preserves the 256-value
feature array and 1,248-byte per-worker workspace: 64 context values and 192
intent values in four position buckets. The last literal `intent: ` separates
caller-supplied context from intent; absent a marker, all input is intent. Active
channels normalize independently with a fixed presence scale. ASCII byte ngrams
remain bounded features, not a natural-language parser or typed source authority.

This version accepts the [own-model experimental exports](https://huggingface.co/asketeddy/gooo-feedback-path-tiny-v1/tree/ad979c4db936cebaeb996acdd9f48b9b4ff135e3/research/split-context-gooo-judgment-20261001).
The matched training comparison regressed continuation attempts in all three
variants. The SDK adds codec compatibility; it selects no default weights and
does not claim a better model. Existing feature versions retain their semantics.
Callers must supply verified structure and legal alternatives separately; the
codec does not extract or validate Gooo context automatically.

All 29 extracted source/test/fixture files are pinned to research
`3b6b38a21978e6c68cb5f498b18a2c9f593668f9`, manifest SHA256
`0a862ac468e5d230e2d0cd6560296d380f546f39cd14ec281cc54ce3281992be`.
The previous 26-file manifest is retained in `source-provenance-v10.json`.
Tests cover exact channel isolation, zero-allocation feature encoding, boundary
rejection, closed metadata versions and eight Go/Python feature fixtures. Model
weights remain separate; offline training and captured model comparisons are in
the research repository. Native compiler main `1e01c96c54f2f8dd43334b8f580af93ffaea24df`
still uses SDK 0.2.8 and rejects v2 before inference. Native SDK adoption and
compiler-produced context are separate subsequent work.

### Go-only own-model typed path example

`go run ./examples/gooo-path --plan plan.json --model model.json` loads an explicit
local structural model and a bounded typed path document. Omitting `--model`
uses deterministic declared ordering and zero predictions. For split-v2 only,
the example checks the original typed plan, then serializes its activity/result,
choice kind/address and legal alternative facts alongside the original intent.
The combined input uses a fixed 512-byte assembly buffer, with explicit overflow
and reserved-separator rejection. The resulting string allocates during setup;
the inference kernel/workspace contract remains separate.

The output records model calls, finite results, context-input digests and the
selected Gooo body. Its serialized facts differ from the current training text;
compatibility is not an effectiveness claim. This SDK-owned example is additional
source outside the 29-file extraction. It does not bind a full original Gooo file
or invoke the native compiler or emitted Go. Source-bound compiler integration
remains a separate step.

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

The `v0.2.2-experimental` revision extracted exactly 20 files from public research source
`733d67857ce2c7569db54d9b42255184186a90e2`; the previous 18-file manifest remains
in `source-provenance-v3.json`. All import transformations and byte digests are
recorded. Source-level/race tests cover legacy-equivalent ordering and bodies,
call/input/output ownership, cancellation, initialization call accounting,
nonblocking concurrency, 128 unique masks over two calls and the maximum mask.
These are contract fixtures, not trained-model benchmark results.

## Optional observed feedback

### Optional finite ambiguity diagnosis

The additive `v0.2.8-experimental` API is
`prepared.Diagnose(ctx, selectedChoices, cases, probeInputs, maxCandidates)`.
It deterministically observes the reference first, then ascending remaining
masks, with at most 64 candidates, 128 cases and 32 probes. The original deadline
and combined type/scope checks apply. It reports unobserved space, actual finite
output-vector agreement, and the first supplied input distinguishing each
case-indistinguishable alternative. Equal pass counts alone are not agreement.
Cancellation retains completed candidates and leaves interrupted work unobserved.

A distinguishing input contains two candidate outputs; neither is an expected
answer or permission to change source. Probe agreement remains unresolved bounded
evidence. The API makes zero model predictions, mutates no session or source,
and supports concurrent calls on owned prepared state. Fixed reference value
arrays total 1,280 bytes; whole-process RAM is separate. Comparison needs the
reference and current candidate, with bounded receipts rather than retained
bodies or an all-pairs matrix.

This 26-file extraction is pinned to research
`a2b7c0c6960888d93f408f0f45af698c579d68cf`, manifest SHA256
`8da2179716d70269852c24c2bb7ab6c200fc4013679b1f0ea6c9bd18fd64f4de`.
The SDK 2.7 manifest is retained in `source-provenance-v9.json`. The new API is
covered by sparse/contradictory cases, incomplete enumeration including mask
65,535, combined rejection, concurrent ownership and cancellation fixtures.
[Methods and captured own-model study](https://github.com/kimjooyoon/gooo-neural-decision-experiments/blob/main/docs/path-diagnosis.md)
are separate from release provenance. Native compiler main still uses SDK 2.7;
this SDK release does not establish native integration or new model training.

`v0.2.7-experimental` adds `prepared.SearchFeedbackBatchesUnfixed` with the same
arguments and bounded progress/receipt ownership as `SearchFeedbackBatches`.
Both APIs share one adapter, including interrupted prediction accounting,
representation declines, budgets and finite continuation. Only the explicit
opt-in API selects `ReconsiderUnfixed`. A synthetic fixture checks the same
four-mask sequence and partial body with six versus five predictions; it is an
API contract check, not a new trained-model or native timing experiment.
The 24-file source extraction is pinned to research
`6cc7d2c42a3526542c7d45a68b7d6a220e658c55`, manifest SHA256
`9a1b59e46865695912355fe87e9c0d86c02725afb6c76f2e59241dc53d5e2620`.
The SDK 2.6 manifest is retained in `source-provenance-v8.json`. Earlier models,
reports and release tags remain unchanged. Native adoption is checked separately.

`v0.2.6-experimental` adds opt-in `Session.ReconsiderUnfixed(ctx, model, ci)`.
It calls the frozen model only for coordinates varying among all unattempted
declared masks. Fixed coordinates are derived from committed attempts using
`[16][2]uint16` counters (64 bytes per session; each count is at most 32,768).
Interrupted candidates consume no count; type-rejected masks count as attempts.
Hashed `fixed_coordinates` receipts record each remaining label and mask count,
without fabricated predictions. Original model, deadline, CI syntax and budgets
remain validated. Removing common log factors can change floating-point near
ties on other data; candidates still pass ordinary type checks and finite tests.
Default `Reconsider`, disconnected ordering and unused receipt fields retain
their existing behavior. Native compiler main continues using SDK 2.5.

This 24-file extraction is pinned to research
`e5d115f79944765d7a675f2f99664d556333ef2a`, manifest SHA256
`843a5b1ea47969c188e1724b2ef42c7b591c6f0081adb4df90cf7b64e5cf3c49`.
The v0.2.5 manifest remains in `source-provenance-v7.json`.
The [paired SDK study](https://github.com/kimjooyoon/gooo-neural-decision-experiments/blob/main/docs/unfixed-feedback-study.md)
records 576 actual SDK sessions and 1,931 own-model predictions: the opt-in arm
uses 917 versus 1,014 legacy predictions, skipping 97 fixed-coordinate calls.
All 288 pairs retain candidate sequence and selected body/finite outcomes. The
legacy SDK matches 288 frozen native references. Twelve actual Go processes
execute 192 function evaluations against an independent integer-state oracle.
These are reused development views, not untouched language accuracy, native
deployment or causal wall-time improvement. No weights or training are added.

`v0.2.5-experimental` skips reconsideration predictions when exactly one declared
candidate remains. There is no alternative ordering to choose. It records a
hashed `ranking_unnecessary` receipt with zero calls, retaining original model,
case, plan, failure, prior progress and caller CI bindings. The round and batch
are consumed; retrying the same batch is rejected. The remaining candidate still
passes ordinary type checks and finite tests. Model validation, deadlines and
invalid caller context remain enforced before this shortcut.

This 24-file extraction is pinned to research
`99e9a1267c3857933c6ab826efbd3703fdb15342`, manifest SHA256
`508c4b09ad195324a5d87b56e81ff00acbba82f449b5cb8ddb13e1e5c1c64c90`.
The v0.2.4 extraction is preserved in `source-provenance-v6.json`. Receipt scope
now describes the original frozen model without inferring its training history.
The [five-family native study](https://github.com/kimjooyoon/gooo-neural-decision-experiments/blob/main/docs/native-feedback-families-study.md)
recorded 244 extra predictions without changed outcomes on two-option plans.
These are observations on SDK v0.2.4; this source release itself is not a native
deployment or a measurement of saved wall time. Multi-option feedback remains
available. No model weights or training are added by this release.

### Earlier v0.2.4 representation handling

`v0.2.4-experimental` records a recoverable representation decline when formatted
feedback exceeds the 512-byte model bound while the original intention is valid.
`Reconsider` returns `ErrFeedbackContextBound` plus a hashed `context_declined`
receipt with zero new predictions, input/intent hashes and the attempted byte
count. It leaves the frontier and best body unchanged, consumes one bounded
round, and rejects a same-batch retry. `SearchFeedbackBatches` continues evaluating
unattempted candidates with the existing ranking after this specific decline.
Cancellation, invalid model/ABI and other failures still stop the operation.
Default/successful receipts omit the added fields and keep their old bytes.

This 24-file extraction is pinned to research
`a8840b8a6ecd079503b7764aef96356e852a5d88`, manifest SHA256
`8a1a7a07d54cc9427848bc39abcb7068112604a0143ffed65c3b6c2e551066c6`.
The earlier 24-file v0.2.3 manifest is preserved in `source-provenance-v5.json`.
Actual native v0.2.3 probes with 475-byte English and 478-byte Korean intentions
are recorded in the research repository and
[issue 1](https://github.com/kimjooyoon/gooo-decision-runtime/issues/1).
The compiler must pin this SDK in a separate checked integration before native
execution can be claimed to include the decline repair. No weights or training
are added by this source release.

### Earlier v0.2.3 API

`v0.2.3-experimental` adds `Session.Reconsider(ctx, originalModel, ciHint)` and
`prepared.SearchFeedbackBatches(ctx, model, cases, total, step, seed, rounds, ciHint)`.
The same frozen model receives a bounded summary of prior finite failures. It
can re-rank unattempted paths; source, cases, model identity and the best observed
body remain fixed. This is explicit reconsideration, not training or acceptance
of unstated intent. Zero rounds with no CI hint uses unchanged `SearchBatches`.

The compatibility helper retains at most 64 attempts and 16 feedback receipts.
It emits an additional progress snapshot after feedback, including actual call
counts when cancellation prevents committing a new ranking. No automatic retry
occurs. Original intent is not truncated to fit context; all inputs must fit the
512-byte ABI before prediction. Default and disconnected execution are unchanged.

`CIHint` accepts a source SHA and PASS/FAIL/UNKNOWN status. It is caller context;
the SDK does not verify GitHub or grant authority to edit source. Each feedback
receipt links actual failures, contexts, predictions and previous observations.
The session retains only counters and the latest digest, with caller-owned logs.

This extraction pins 24 source/test files to research source
`5ee0ce493adaa56f0f1f9b8f9811d265ff5601d9`. The previous 20-file manifest is retained
as `source-provenance-v4.json`; older manifests and release tags remain intact.
No model weights, private environments, raw studies or Python are included.
The [actual frozen-model pilot](https://github.com/kimjooyoon/gooo-neural-decision-experiments/blob/main/docs/feedback-path-pilot-results.md)
found no aggregate functional gain and 102 additional predictions; it informs
the optional default rather than a claim of improved general language accuracy.

## Joint full-path model (v0.2.12-experimental)

The opt-in `jointdecision` package loads a separate closed 512/24/4 model ABI.
It preserves two complete canonical source-v3 inputs and ranks the four legal
absolute masks together. `NewJointSession` and `SearchJointFeedbackBatches`
make one initial prediction. Later observed failures can re-rank the remaining
full masks with one explicit prediction, while the final sole mask needs none.
Seeded sampling is reproducible and excludes the seed from model text.

The caller owns fixed arrays: 2,160-byte workspace, zero heap allocations in
valid prediction tests. Five-trit matrices occupy 2,590 file bytes and decode
into 12,496 tensor bytes plus eight scale bytes. Runtime arithmetic remains
int8/float32; packed file size is not whole-process RAM. Invalid inputs leave
workspace/prediction unchanged. Representation decline keeps deterministic
continuation; nonblocking session locks and cancellation retain committed work.

This optional head changes no previous model ABI and is not the default model.
The SDK accepts a caller's validated typed plan; source-file binding belongs to
the native compiler. All nine additional runtime/test files and earlier files
are digest-bound in `source-provenance.json`; the previous v0.2.11 manifest is
preserved in `source-provenance-v0.2.11.json`.

## Three-choice full-path model (v0.2.13-experimental)

The separate opt-in `jointdecision.ThreeModel` ABI is 768/24/8. Use
`jointdecision.LoadThree`, `PreparedPlan.NewThreeSession`,
`Session.ReconsiderThree` or `PreparedPlan.SearchThreeFeedbackBatches`.
It preserves three complete source-v3 inputs and ranks all eight masks together.
`FeedbackThreeWithParts` exposes complete attempted inputs and individual parts
so representation declines retain their full byte/hash identity.

One initial prediction precedes finite candidate tests. Explicit actual-failure
feedback can re-rank all remaining masks with one prediction; the final sole
mask needs none. Unsupported arity preserves every intention, including a fourth
choice, and runs deterministic continuation with zero predictions. Disconnected
execution is also deterministic. Context overflow leaves the frontier unchanged,
retains full attempted input and consumes one bounded feedback round.

The caller-owned fixed workspace is 3,200 bytes. FP32 tensors occupy 74,624 bytes;
five-trit packed weights occupy 3,854 file bytes and decode to 18,624 int8 matrix
bytes plus 128 FP32 bias bytes and eight additional scale bytes. Valid warmed
projection and inference allocate zero heap objects in contract tests. These
counts exclude allocator and process overhead. The shared Session gains 32 bytes
for four additional scores. Metadata remains capped at 64 KiB; the new weights
cap is 128 KiB and old four-label models retain their 64 KiB cap.

Tests check nonzero FP32/ternary matrices against separate float64 arithmetic,
all eight probabilities, atomic failed outputs, simultaneous independent sessions,
finite TDD, model pins, full overflow identities, cancellation and nonblocking
locks. Runtime arithmetic uses int8/float32, rather than executing in packed form.

All 51 source/test/fixture files are pinned to research revision
`1dae673495bce8b8a906d8cc4412f4abbe25c8b2`, manifest SHA256
`36207f298a0884aaedb76057e6dc018e8ec198c429b7564df8624085584323fe`.
The previous manifest is retained in `source-provenance-v0.2.12.json`.
This release adds model compatibility and controlled test weights only. It does
not claim new trained quality, native deployment or general language completeness.
See the [frozen three-choice study](https://github.com/kimjooyoon/gooo-neural-decision-experiments/blob/d74a8a455ceed5949fcbad482375405b4704dc9a/docs/own-three-choice-completeness-preregistration-20261002.md)
and its [implementation phase](https://github.com/kimjooyoon/gooo-neural-decision-experiments/blob/1dae673495bce8b8a906d8cc4412f4abbe25c8b2/docs/own-three-choice-sdk-abi-20261002.md).
