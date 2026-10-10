# Gooo Decision Runtime

This is the Go inference and program-assembly component of
[Gooo](https://github.com/kimjooyoon/meta-ontology-go). The compiler provides a
source-bound plan; this library ranks its permitted choices and continues finite
search using observed failures. Think of it as the small assembly mechanism
inside the larger language workshop.

**Latest published models: v0.2.33-experimental.** The
[release](https://github.com/kimjooyoon/gooo-decision-runtime/releases/tag/v0.2.33-experimental)
contains two384-input value-flow FP32 models and checksums. The flow-on model
has9,290 parameters/37,160 weight bytes; its JSON is108,446 bytes. Compiler
[PR1442](https://github.com/kimjooyoon/meta-ontology-go/pull/1442) connects these
weights to Gooo construction. The installed compiler0.6.25 still uses SDK0.2.28.

The earlier shared judge uses 2,072
parameters across three binary decisions. Its compact FP32 weights occupy 8,288
bytes; ternary files occupy 446 bytes and decode into 2,096 tensor bytes plus
eight scale bytes. A caller owns a 3,200-byte workspace. These array sizes are
one component of application RAM.

Development source adds an explicit [v5 semantic-flow input](docs/semantic-value-flow.md).
It normalizes eligible direct/copy/assignment forms while preserving the v4
input for other shapes. Published v4 weights keep their original input contract;
the new input requires its own training. This change does not relabel old weights.

### Condition candidate model

[`conditiondecision`](conditiondecision/README.md) trains a shared 256 → 24 → 2
network directly in Go. Its 6,218 FP32 parameters occupy 24,872 weight bytes.
Learning uses the probability mass of complete source-valid candidate sets;
the selected combination is still checked by the Gooo compiler or interpreter.

In the [first fixed study](studies/condition-candidate-20261010/README.md), CPU
training took 0.172s. Ranking four candidates over two choices had a 7.667µs
median. New wording scored 2/4 on initial judgments and 12/16 after observations;
local-variable versions of the same absolute-value problem scored 8/8 and32/32.
The study retains all120 selected bodies and finite results, including failures.
This package has its own artifact ABI. The v1 compiler CLI connection is in
[PR1441](https://github.com/kimjooyoon/meta-ontology-go/pull/1441); compression of
these new weights remains work.

The opt-in [v2 branch return inputs](docs/branch-return-features.md) distinguish
which branch returns an input, literal, local or composed expression. They fill
twenty reserved cells without increasing the model's dimensions. In the
[paired 60-source study](studies/branch-role-learning-20261010/README.md), v2
reached 8/8 on the training sources and 6/16 on new wording; v1 scored 4/8 and
8/16 respectively. Both weights and every selected body/case are published.
All five finite-search modes completed the authored cases. Condition-only
feedback supplied no extra neural calls when output values were wrong but the
declared comparison was right. The v3 implementation below adds this missing input.

### Output feedback model

[`executiondecision`](executiondecision/README.md) adds a 320 → 24 → 2 model
with 7,754 FP32 parameters (31,016 weight bytes). Its input preserves all v2
channels and adds the observed input, expected output and actual output as exact
integer bytes. `NewExecutionSession` and `SearchExecutionBatches` feed committed
output failures into the next ranking, then compile and evaluate the selected body.

In the [fixed 128-source study](studies/execution-feedback-learning-20261010/README.md),
Gooo-based CPU training took 0.825s for the output-on model. Output feedback
reduced four-region search attempts from 62 to 54; first-choice validity stayed
at 2/16. Across all sources, attempts fell from 220 to 212 while elapsed search
time increased. All models, successful and failed choices, and finite searches
are published. The current compiler release does not load this schema yet.
[Input layout, APIs and current limits](docs/execution-feedback.md).

### Values carried through assignments

The proposed [384-cell value-flow input](docs/branch-value-flow.md) follows
assignments and copies through a selected branch to the eventual return. This
lets the input describe the difference between two branches that both return
the same local name after giving it different values.

In the [fixed static projection](studies/flow-feature-projection-20261010/README.md),
the original 128-source collection had 72 pairs with identical v3 inputs but
disjoint acceptable paths. The additional facts distinguished all 72, with zero
excluded sources. No new model was trained in that measurement. The separate
[`flowdecision`](flowdecision/README.md) package supplies 384-cell CPU training
and finite-search integration. In the
[paired fresh learning study](studies/value-flow-learning-20261010/README.md),
first-choice validity outside training rose from 54/96 with flow disabled to
73/96 with it enabled. Assignment forms stayed at 8/16. All four search modes
completed the authored cases; flow-on feedback added 23 calls without saving
attempts. Both trained models and the individual regressions are published.

### Connecting the condition model to finite search

`PreparedPlan.NewConditionSession` now uses the model's additive choice scores
inside the existing frontier. It supports 1–16 binary choices and source-bound
feedback, with one neural call per changed context. Gooo's typed checks, declared
conditions and output cases still decide which complete body can be selected.
A nil model preserves declared-fallback search. Calls, skipped calls, rejected
candidates and cancellation are recorded separately.

See [the API and its limits](docs/condition-search.md). `SearchConditionBatches`
provides a 64-attempt, 16-feedback-round adapter for local document consumers.
It selects the input version from the frozen model for initial and later calls.

In the [fixed 24-contract search study](studies/condition-search-20261010/README.md),
deterministic search tried 60 paths, initial ranking tried 35, and condition
feedback tried 28. All three completed the same authored cases. These are reused
absolute-value variants; fewer paths did not make every subgroup faster.

### Separate inputs for the condition model

`ConditionFeaturesInto` separates source structure, full intent and an observed
condition into a fixed 1,024-byte array. Expected/actual Boolean roles, unreached
conditions, candidate choices and exact int64 bytes have dedicated fields.
Appending feedback leaves source and intent features unchanged. The new
`conditiondecision` artifact uses separately trained weights. Prior path-model
loaders retain their original feature contracts. See
[the layout and evaluation criteria](docs/condition-feature-channels.md).

### Condition-aware model feedback

An output-correct body can still fail a declared intermediate condition. After a
committed batch, `Reconsider`, `ReconsiderUnfixed`, `ReconsiderJoint` and
`ReconsiderThree` now include that reason in the frozen model's next input.
`FeedbackReceipt` adds two optional fields: `prior_condition_rejections` and
`first_condition_failure`. The latter holds the first rejected candidate's mask
and first failing `ConditionResult`, including the exact int64 input, expected
Boolean, actual observation and final output.

For example, the appended context can contain:

```text
condition_rejected=1 condition_mask=0 condition_choice="comparison" condition_input=-9007199254740995 condition_expected=false condition_actual=true condition_status=MISMATCH
```

A skipped condition carries `condition_actual=UNOBSERVED` and
`condition_status=NOT_REACHED`. It cannot silently become an observed false.
Only completed candidate evaluations supply this evidence. The session retains
one counterexample, while callers own the full attempt log. Returned copies
cannot change later feedback. The model keeps its original weights; the complete
source input and intent remain present. Feedback enters the existing hashed
natural-language feature channel, so reliable interpretation still depends on
the model's training and that channel's capacity.

The existing input bounds apply to the full augmented input. Oversized inputs
decline inference and leave the frontier unchanged. With one remaining path,
no ranking call is needed. With no rejected conditions, the previous feedback
text and JSON fields stay unchanged. Deterministic sessions continue without a
model; passing the declared output and condition cases still decides acceptance.

Regression coverage checks per-choice, two-choice and three-choice input/feature
delivery, cancellation, copied receipt ownership, exact large integers,
unreached predicates and oversized inputs. This establishes evidence delivery;
new trained accuracy or fewer search attempts require a separate measurement.
`source-provenance-v0.2.27.json` preserves the previous manifest byte for byte;
`source-evolution-condition-feedback.json` binds the three evolved runtime files.

### Source condition cases (v0.2.27)

`bodyplan.Program.ObserveCondition(input, statement)` observes an existing `if`
condition while running the complete selected body. It uses the ordinary local
slots and expression evaluator. Skipped branches return `reached: false`.

`pathplan.PreparedPlan.CheckConditions` connects these observations to declared
choice IDs. For example, after preparing a source-bound plan and choosing all its
paths:

```go
rows, err := prepared.CheckConditions(ctx, selectedChoices, []pathplan.ConditionCase{
    {ChoiceID: "comparison", Input: -1, Expected: true},
    {ChoiceID: "comparison", Input: 0, Expected: false},
    {ChoiceID: "comparison", Input: 1, Expected: false},
})
```

Each row records the final output, observed condition and `MATCH`, `MISMATCH` or
`NOT_REACHED`. Only `MATCH` passes. A `branch_layout` choice identifies its own
`if`; an `operand_order` choice must identify the whole condition of exactly one
`if`. Ambiguous references and other expression/choice kinds return an error.
There are at most 128 cases, with exact int64 inputs and explicit Boolean answers.
No model calls occur in either API.

`Plan.ConditionCases` makes these examples part of the immutable plan contract.
Bounded search, incremental batches, feedback, two/three-choice model sessions
and probe-based resolution retain condition observations for every evaluated
candidate. A candidate failing a declared condition cannot become the selected
program, even when its final outputs pass. Its final-case results remain visible
with `CONDITION_REJECTED`, separately from type rejection. Budgets and initial
model calls are retained; source constraints do not restart a search. Direct
`Choose` reports a condition failure for an ineligible one-shot selection.

Probe caches retain only candidates satisfying the original source conditions.
Subsequent snapshots reference the initial condition observations and report
reused condition evaluations, with zero new condition executions. The two-update
whole-candidate profile supports straight-line assignments; `if` conditions are
outside that existing profile.

These APIs address a gap seen in the
[Gooo joint-path model study](https://github.com/kimjooyoon/gooo-ecosystem-workbench/tree/b63ea6d74603a89e1a74af2b3dc36c1596742935/models/joint-path-20261010):
opposite predicates can produce the same final answers after swapping branches.
The caller supplies the expected predicate values. Counts cover only those
authored cases. Gooo source syntax and saved-replay enforcement were merged in
[compiler PR1439](https://github.com/kimjooyoon/meta-ontology-go/pull/1439), with
publication being prepared in PR1440. Keep final-output scores and intermediate-condition scores
separate. Empty condition suites carry no predicate evidence. The previous
extraction manifest is preserved in `source-provenance-v0.2.26.json`;
`source-evolution-condition-cases.json` records the exact SDK-owned changes.

### Ordered source value graph (v0.2.26)

`RecordGraphSharedFeatureVersion` (`triple_record_value_graph_v3_shared_v1`)
retains source operators, canonical literals, root parameter positions/types and
ordered value edges. It addresses distinctions removed by the v1 expression
summary and v2 ancestor counts, including AND/OR and subtraction operand order.

`RecordGraphInput` carries exactly three complete field choices and one shared
graph of at most 512 nodes. Choice roots and all ordered parent, condition and
execution-guard edges refer to one-based node positions. Parents precede their
child, so cyclic or forward relations are rejected. The full canonical source
input is bounded to 64 KiB; each string is bounded to 1,024 UTF-8 bytes. A caller
owns source provenance, type validity and expression/root correspondence. The
SDK validates the structural contract without executing code or test cases.
Each input node declares `bool`, `int64` or `string`; record fields retain their
stable field IDs and the corresponding primitive type.

Each field still occupies 256 floats: 96 per ordered alternative and 64 for the
complete intent's byte ngrams. An alternative has 32 explicit root-operation
slots, 32 reachable-operation counts and 32 hashed value-relation slots. Ordered
recursive fingerprints retain operand positions, conditional arms, literal
values and input identity. Plain copies share their value fingerprint; a newly
attached execution guard remains represented. Local names, source spans and
graph node numbers do not enter those fingerprints. Only reachable nodes enter
the features. The seven channels normalize independently.

The finite relation and intent hashes can collide. This is a compact learned
input, not a lossless semantic comparison. The intent channel has fewer slots
than v2; bilingual and unseen-program quality need measurement. The shared judge
still scores fields independently. Eight synthetic alternative arrangements are
distinct in the regression fixture, but that is not a prediction-accuracy result.

`EncodeRecordGraphThree`, `DecodeRecordGraphThree` and
`FeaturesIntoRecordGraphThree` expose transport and feature preparation.
`LoadRecordGraphSharedThree`, `PredictRecordGraphSharedInto` and
`PredictRecordGraphSharedFeaturesInto` require this explicit contract and its byte
bound. The 256/8/2 judge, 768-float input, FP32/ternary layouts and caller-owned
prediction workspace are unchanged. Prepared prediction retains zero warmed
heap allocations; JSON decoding and source feature preparation use additional
temporary memory. Tests use synthetic ABI weights only. New trained weights,
compiler integration and native program evaluation remain separate work.

The published v1 and v2 contracts and their feature arrays remain unchanged.
Their model files are rejected by the graph loader. The new graph files are
SDK-local additions; the extracted source provenance manifest remains intact.

### Unread candidate locals (v0.2.25)

Typed body plans can retain local values that a selected candidate never reads.
Their initialization, assignments, scope and type checks are preserved. Go
emission adds an adjacent `_ = local` only for an unread binding; Gooo bodies and
the typed arena keep their original statements. Existing bodies with all locals
read emit the same source. Two fixed 128-slot scratch arrays track reads and
emission markers by binding identity, including same-name locals in separate branches. The interpreter's
runtime frame and model contracts are unchanged.

The original extraction receipt remains in `source-provenance-v0.2.24.json`.
The current receipt preserves the research origin and records the subsequent
SDK-local renderer change against the extracted baseline.

`jointdecision.RecordFieldFeatureVersion` describes three source-owned
string-field choices directly. `EncodeRecordThree` keeps each complete field
name, ordered alternative expression and Korean/English intent; supplied inputs
and expected outputs are excluded. `LoadRecordThree` requires a separately
trained feature contract and the explicit `PredictRecordInto` entry point. Its dense 768/24/8 layout uses the existing fixed
3,200-byte workspace; JSON and expression parsing currently allocate temporary
storage. The feature map distinguishes same-field copying, constants, and
prefix/suffix concatenation, with finite hashed literal/intent channels. It
does not encode enclosing control flow or resolve local variable bindings.
The [paired-intent study](https://github.com/kimjooyoon/gooo-neural-decision-experiments/tree/main/publication/paired-field-intents-20261005)
trained the dense contract and executed its generated record bodies. Its FP32
first complete mask matched 424/1,536 source views; finite continuation completed
every full-budget native graph. These are separate measurements.

### Source value origins (v0.2.24)

`RecordOriginSharedFeatureVersion` adds a separate input contract for three
ordered record choices. Each choice carries its complete expressions, intent,
and two fixed 16-slot arrays of source ancestor counts. Think of a copied record
as a saved photograph: later writes change the working record, while the saved
photograph keeps its earlier values. The compiler supplies that distinction.

`EncodeRecordOriginThree` and `FeaturesIntoRecordOriginThree` use 64 expression
slots, 32 origin slots and 160 intent slots per field. Each channel normalizes
independently; three 256-slot parts keep the existing 768-slot workspace. The
origin slots count input fields, literals, expressions, copies, writes, earlier
choices, conditional joins, guards and reads. Input values and test answers
are absent. Each alternative has 1..512 ancestors, each canonical part is at
most 1,024 UTF-8 bytes, and the complete input bound is 4,096 bytes.

`LoadRecordOriginSharedThree` requires that exact feature version, byte bound,
and `float32_separate_v1` arithmetic. `PredictRecordOriginSharedInto` performs
projection and prediction; `PredictRecordOriginSharedFeaturesInto` consumes a
prepared array. The shared 256/8/2 layout and request-owned workspace stay the
same. Tests check text/prepared parity, concurrent calls, atomic errors, and
zero warmed allocations for prepared prediction. Source JSON/AST projection
still allocates temporary storage.

This release implements the runtime contract with synthetic regression weights.
The published v1 field model retains its original feature contract and weights.
New origin weights and their quality require a separate training study. Counts
give a bounded summary of the dependency graph; distinct graphs can still share
counts. The shared judge also retains its independent-field scoring assumption.
Finite execution remains the measurement of observed functionality.

```go
model, err := jointdecision.LoadRecordOriginSharedThree("origin/model.json")
if err != nil { return err }
var workspace jointdecision.ThreeWorkspace
var prediction jointdecision.ThreePrediction
err = model.PredictRecordOriginSharedInto(sourceOriginText, &workspace, &prediction)
```

### Shared record field judge (v0.2.23)

`RecordSharedFeatureVersion` uses the same complete expression/intent projection
with one 256/8/2 judge shared across three fields. It returns eight composed
path scores in one call. The FP32 weight file is 8,288 bytes; PTQ/QAT ternary
files are 446 bytes. This release supplies the execution contract and regression
fixtures. The [frozen field study](https://github.com/kimjooyoon/gooo-neural-decision-experiments/tree/172d366a45cbf591273823832a367ce3c66b8cdb/publication/record-shared-field-20261005)
now measures independently trained weights under this contract. The same3,840
MPS updates produce first-mask agreement of1,152/1,536 FP32 and1,344/1,536 QAT
on the registered source-body axis; different bodies with new wording reach
120/512 and102/512. Across24 native source views, budget-one active fields reach
126/144, while the eight new-wording views regress from the earlier model's32/48
to30/48. These are three authored field roles and eight authored body families;
all eight required masks occur in training. [Public weights and finite observations](https://huggingface.co/asketeddy/gooo-record-shared-field-tiny-v1).

The fixed prepared-array prediction measured3.04µs FP32/3.13µs QAT with zero
warmed heap allocations. Source parsing and full generation/build/execution
have separate costs. Model probabilities order attempts; supplied finite cases
measure matching fields and remaining work. Omitting a model keeps the compiler's
deterministic order. The immutable v0.2.23 tag retains its original release source;
this main documentation links the later measured study.

```go
model, err := jointdecision.LoadRecordSharedThree("fp32/model.json")
if err != nil { return err }
var scratch jointdecision.ThreeWorkspace
var prediction jointdecision.ThreePrediction
err = model.PredictRecordSharedInto(completeRecordText, &scratch, &prediction)
if err != nil { return err }
fieldProbabilities, err := jointdecision.RecordChoiceMarginals(prediction)
```

`PredictRecordSharedFeaturesInto` accepts a caller-prepared `[768]float32` array,
using `FeaturesIntoRecordThree` unchanged. Text/Go-expression parsing can then
be measured separately from prediction. Each concurrent request owns its scratch
and output; loaded weights are shared. Model metadata explicitly carries
`triple_record_field_context_v1_shared_v1` and `float32_separate_v1`; the integer
and dense record APIs reject this different contract.

Each field's score depends on its own ordered expressions and intent. Combining
the scores assumes fields can be judged independently. Cross-field requirements
still need finite execution or a model representing that dependency. Marginals
are ordering probabilities; supplied cases determine observed completeness.
Literal hashing, wording sensitivity and the current three-field bound remain
properties of the input representation.

| Read next | Contents |
| --- | --- |
| [Gooo Wiki, 한국어](https://github.com/kimjooyoon/meta-ontology-go/wiki) | Language/model walkthrough, current support and metric definitions |
| [Public model](https://huggingface.co/asketeddy/gooo-shared-judgment-tiny-v1) | FP32/PTQ/QAT artifacts, quality table and intended use |
| [Research and current results](https://github.com/kimjooyoon/gooo-neural-decision-experiments) | Training, parity, finite execution and resource measurements |
| [Language direction, 한국어](https://github.com/kimjooyoon/meta-ontology-go/blob/dev/docs/language-direction.ko.md) | Intent, construction, provenance and next language work |

The latest native study used **v0.2.15** in 400 Gooo generations and 800 compiled
runs; all 9,600 supplied finite expectations and 192 representation pairs matched.
Actual predictions totaled 816. The model-free arm completed deterministically.
Model first-choice quality and Korean/English agreement remain separate tasks.
[Full measurement scope](https://github.com/kimjooyoon/gooo-neural-decision-experiments/blob/main/docs/full-input-native-results-20261003.md).

The earlier [input-sensitivity study](https://github.com/kimjooyoon/gooo-neural-decision-experiments/blob/main/docs/bilingual-wrapper-audit-results-20261003.md)
keeps all six models fixed and varies authored instruction phrasing. Shared FP32
completes 113/512 original development views and 480/512 bare instructions that
match the training format. These conditions expose a wording weakness in the
current model. The runtime continues to preserve complete caller input; phrasing
robustness motivated the next model/feature comparison.

That [full-input study](https://github.com/kimjooyoon/gooo-neural-decision-experiments/blob/main/docs/full-input-judgment-initial-results-20261003.md)
has now published twelve exports from four training conditions. On the same
observed development tasks, whole-text fragment FP32 completed 368/512 first
paths, compared with 113/512 for the positioned control. It also exposed an
operation-order feature alias and quantization regressions. Its V4 feature
contract and the explicit arithmetic rule are now extracted into this SDK.
Use the [pinned compact V3 bundle](https://huggingface.co/asketeddy/gooo-shared-judgment-tiny-v1/tree/985999a89caba6a31cc7147f66ba29a5ce76a1d9/research/compact-runtime-20261003)
with SDK v0.2.14 and later; the v0.2.15 compiler integration also loads the
explicit V3/V4 artifacts described below. The [Linux follow-up](https://github.com/kimjooyoon/gooo-neural-decision-experiments/blob/main/docs/full-input-numerical-portability-followup-20261003.md)
records 272 differing complete candidate rankings. The subsequent
[explicit arithmetic comparison](https://github.com/kimjooyoon/gooo-neural-decision-experiments/blob/main/docs/full-input-separate-arithmetic-results-20261003.md)
reproduces every intermediate value, probability and ranking across 18,432
arm64/Linux input pairs, using unchanged weights and 147,456 actual calls.
Both runtime implementations carry the arithmetic identity in model metadata
and initial/feedback receipts. The SDK CI replays every explicit model against
the frozen observations. Native V4 generation and execution completed on compiler
`e461c1d`. The integration merged to dev as `75b2b7d` in
[PR 1158](https://github.com/kimjooyoon/meta-ontology-go/pull/1158); main promotion
completed as `fc0e99c4` in [PR 1159](https://github.com/kimjooyoon/meta-ontology-go/pull/1159).

### Reading model artifacts (v0.2.21)

On Unix, operation, structural, two-choice, expanded three-choice and shared
three-choice loaders open metadata and weights without waiting for a FIFO writer.
They reject symlinks and validate the opened regular-file identity and extent
before reading at most the declared bound plus one byte. Existing model schemas,
weight digests, tensor layouts and finite-value checks still apply. This changes
artifact loading; prediction arithmetic and model weights keep their identities.

Missing, replaced or malformed artifacts return an error. Callers can omit the
model to use their deterministic path order; a failed requested model remains a
visible setup failure. The non-Unix implementation has Windows arm64 compilation
evidence and retains descriptor checks. Runtime nonblocking behavior there has
not been measured.

The loader source and regressions come from research revision
[`ddcd1002`](https://github.com/kimjooyoon/gooo-neural-decision-experiments/commit/ddcd1002b14996f4e6b5577a227e065441a407c5).
The current 69-file extraction records both original and relocated byte digests;
the prior extraction is retained in
[source-provenance-v0.2.20.json](source-provenance-v0.2.20.json).

### Whole-candidate source-conditioned judge (v0.2.19)

The additive `orderfacts` and `orderjudge` packages support a separately trained
[16 KiB model](https://huggingface.co/asketeddy/gooo-order-judge-tiny-v1). It scores
all eight complete candidate bodies against the original Korean/English intent.
The admitted shape is `let v=input; two integer updates; return v`, with one root
order and two operand choices. Constants in the learned feature projection are
bounded to -16..16. Existing V3/V4 APIs and models retain their contracts.

```go
model, err := orderjudge.Load(metadataBytes, weightBytes)
if err != nil { return err }
search, body, ranking, err := orderjudge.Search(ctx, plan, model, cases, 8, true)
```

`ctx` must have a deadline. The caller supplies a validated source-bound plan;
this library independently checks its typed candidates. One whole-candidate
prediction precedes finite candidate tests. The final Boolean flag enables reuse
of already evaluated equal 48-byte descriptors. The budget counts actual body
evaluations; the ranking receipt records skipped masks and representatives.
A nil model keeps deterministic fallback-distance order and makes zero predictions.
Rank probabilities are uncalibrated, and results describe the finite input cases.

This package owns 4,096 FP32 weights. A caller's prediction workspace is 128 bytes;
features, typed programs and receipts use additional memory. Search currently
prepares all candidates each call. The optional Go `Fit` API implements the fixed
bounded experiment; runtime callers normally load the public artifact.

The [source manifest](source-provenance-order-judge-v1.json) binds the additive
packages and [complete replay example](examples/order-replay/main.go) to their
research origin. Run the example against decoded `initial.zip` from the
[original evidence](https://github.com/kimjooyoon/gooo-neural-decision-experiments/tree/8a41bb0f825dfd3f950101b33491581492c740f6/publication/order-judge-initial-20261003):

```sh
go run ./examples/order-replay --input /tmp/gooo-order-initial --output /tmp/gooo-order-sdk
```

It makes 480 real predictions, executes 960 searches and checks 320 baseline
searches, plus every frozen feature and candidate result. Outputs must be fresh.
This SDK example uses the typed interpreter; compiler native integration is a
separate stage.

### Constant bodies with a declared input (v0.2.18)

A typed body still declares exactly one Integer input named `input`. Its body
may leave that parameter unread, as in `func Constant(input int64) int64` returning
a literal or a constant expression. The input node belongs to the function
signature; this release permits it to be absent from the expression dependencies.
Other unused expressions, missing/duplicate input nodes, unreachable statements,
scope errors and type errors remain rejected. The existing plan schemas and
already accepted bodies keep their representation.

This allows source-derived Gooo recipes for constant results to use ordinary
typed search and observation. Model inference and training are optional. Tests
cover Integer and Boolean constants, integer endpoints, ordinary structural
selection and rejection of unrelated unused nodes.

The [research change](https://github.com/kimjooyoon/gooo-neural-decision-experiments/commit/5cda16c301b32b21c0d87093aff03c37c7471fd7)
is extracted with its regression test and exact byte digests in the current
64-file manifest. The preceding extraction is retained in
[source-provenance-v0.2.17.json](source-provenance-v0.2.17.json).

### Next useful observation (v0.2.16)

`prepared.RankProbes(ctx, cases, inputs, maxCandidates)` compares declared
integer bodies that match the supplied cases. It proposes the input separating
the most surviving candidate pairs, with deterministic tie handling. An existing
test oracle supplies that input's expected value; candidate outputs remain
observations. See [the executable example](examples/probe-ranking/main.go):

```sh
go run ./examples/probe-ranking
```

The example resolves `input-2` versus `2-input` using one new oracle observation
after both pass at input 2. Bounds: 64 candidates, 128 cases, 32 probes; the fixed
output matrix is 16 KiB. Model calls and training updates are zero. Partial space,
unresolved agreement, cancellation and source/case/probe identities are explicit.
This additive API is introduced in v0.2.16; the current compiler CLI integration
is tracked in [compiler PR 1160](https://github.com/kimjooyoon/meta-ontology-go/pull/1160).
The [paired native pilot](https://github.com/kimjooyoon/gooo-neural-decision-experiments/tree/main/publication/path-observation-loop-20261003)
retains its exact compiler and model versions. The API follows the practical
information-value question in [LAVOIR](https://arxiv.org/abs/2609.30706), using a
finite output-partition count as its score.

### Reusing observations (v0.2.17)

`prepared.StartProbeSession(ctx, cases, inputs, maxCandidates)` performs that
initial ranking and retains its outputs. `session.AppendObservation(ctx, testCase)`
then filters the already observed rows using an expected value supplied by an
existing oracle. It does zero new compilation, evaluation or model calls.
`session.Snapshot(ctx)` returns an owned copy of the current evidence.

```go
session, first, err := prepared.StartProbeSession(ctx, cases, inputs, 64)
if err != nil { return err }
// Obtain expected from the separately declared specification.
next, err := session.AppendObservation(ctx, pathplan.TestCase{
    Input: chosenInput, Expected: expected,
})
if err != nil { return err }
fmt.Println(first.TotalEvaluationAttempts, next.Ranking.SurvivingMasks)
```

The input must have appeared in the initial probes and must not already belong
to the finite suite. Initial cases remain unchanged. A partial candidate budget
stays partial after filtering. An oracle value can leave no surviving candidate;
that remains visible. The session owns fixed arrays for 64×32 output values,
128 cases, 32 inputs and 64 masks. Source plans and returned receipts are separate
memory costs. Methods are serial per session; independent sessions may share a
prepared plan. Invalid additions and cancellation before commit keep the prior
revision intact.

Each snapshot records its original ranking hash, current case hash, revision,
cached comparisons, reused output values and cumulative evaluation count.
`ranking.evaluation_attempts` counts new evaluations in that operation, which is
zero on append/snapshot. The first operation records the initial actual work.
No serialized receipt can be imported as a trusted session.

The [small local kernel comparison](benchmarks/probe-session-20261003.md) compares
fresh ranking with cached continuation. This additive API is introduced in
v0.2.17. The compiler's first observation-loop integration uses v0.2.16. Full codegen
latency and skipping unnecessary model ranking require separate integration.

The sections below document each API and the release in which it was introduced.
Historical compiler-version statements describe that release's observation.
Source extraction manifests retain the exact origin of the runtime files.

## Module and original operator interface

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

## Full input and explicit arithmetic (v0.2.15-experimental)

Three-choice artifacts can declare `arithmetic_version: float32_separate_v1`.
The runtime rounds each product before adding it and records this rule through
`ThreeModel.ArithmeticVersion()` and path receipts. Omitted metadata retains the
previous arithmetic implementation; unknown versions are rejected. Compaction
preserves the declared rule.

`jointdecision.LoadThreeBag` loads expanded
`triple_semantic_context_bag_v4_joint_v1` artifacts. `LoadThree` retains its V3
contract, and `LoadSharedThree` accepts either declared feature version for the
compact layout. V4 retains complete text and counts 2/3-byte intent fragments.
It has a documented operation-order alias, so typed alternatives, finite cases
and deterministic continuation remain part of its intended use.

Source and tests are extracted from research revision
`daecfea3583614e006c960de263448a1645a9190`: 63 pinned files, manifest SHA-256
`608238066589da0cb6065a2f77b6254c8102891fb598dc2d41c64d5ee7d630f2`.
The previous manifest is retained in `source-provenance-v0.2.14.json`.

[v0.2.15 is published](https://github.com/kimjooyoon/gooo-decision-runtime/releases/tag/v0.2.15-experimental)
at source `59c8d342da4475506b90954469aa201f85cadeb3`. Local darwin/arm64 and
[Linux CI](https://github.com/kimjooyoon/gooo-decision-runtime/actions/runs/37070241916)
each completed all 18,432 inputs with 36,864 actual predictions. Both matched
the complete explicit-arithmetic reference, including intermediate bits and
full rankings. These repeated numerical observations use frozen models and
zero optimizer updates. The native observation linked above then exercised this
contract through generation, immediate build and two executions per request.

The [registered integration protocol](https://github.com/kimjooyoon/gooo-neural-decision-experiments/blob/daecfea3583614e006c960de263448a1645a9190/docs/full-input-sdk-native-protocol-20261003.md)
requires 36,864 actual SDK predictions over all 18,432 frozen inputs, comparing
both layouts' full intermediate values and rankings to the public explicit
lanes. The replay emits a result even on a collection failure, with actual call
counts and the last condition/view. Run it from a clean SDK checkout:

```sh
go run ./examples/three-replay --bundle /path/to/full-input-separate-arithmetic-20261003 \
  --source-revision "$(git rev-parse HEAD)" --output /tmp/sdk-full-input-replay.json
```

The output path must be fresh and outside tracked source. Model artifacts come
from the [public arithmetic appendix](https://huggingface.co/asketeddy/gooo-shared-judgment-tiny-v1/tree/7c4501789b34d885c14ab54aee1d40995eb8b1e6/research/full-input-separate-20261003).
CI retrieves the pinned reference and publishes its replay report. Local and
CI observations have separate prediction counts. Native compiler integration
and model-default selection have their own subsequent evidence.

## Compact shared three-choice judgments (introduced in v0.2.14-experimental)

`jointdecision.LoadSharedThree` explicitly loads
`gooo/tiny-shared-three-choice-path-model/v1`. The same 768-value, full-input
feature contract reuses an 8-hidden-unit judge for each of three decisions.
`hidden_dim: 8` describes the stored local judge; the caller still provides a
3,200-byte `ThreeWorkspace` containing 24 computed hidden values and eight mask
scores. All existing three-choice path/session/feedback APIs accept this model.
Initial and feedback receipts name its actual compact schema and file hashes.

| Layout | FP32 | PTQ/QAT ternary |
| --- | ---: | ---: |
| Weight file | 8,288 bytes | 446 bytes |
| Resident tensors | 8,288 bytes | 2,096 bytes |
| Separate matrix scales | 0 bytes | 8 bytes |

`jointdecision.CompactThree(expanded)` returns closed metadata and weight bytes
only when every removed zero and tied copy is exact. It performs no training or
requantization. The loader rejects malformed dimensions, tensor layouts,
nonfinite values, invalid trits/padding, digest mismatches and symlinked files.
Inference uses caller-owned arrays and preserves them on invalid input/numerical
failure. Valid warmed calls allocate zero heap objects in the contract tests.

The expanded and compact public shared models matched bit-for-bit on all
10,739 previously frozen initial/feedback inputs for each of three variants.
This is representation parity, not a new accuracy result. Local kernel medians
were 24.5–27.7 microseconds including features; these exclude native codegen and
are not a cross-machine performance guarantee. The
[full report](https://github.com/kimjooyoon/gooo-neural-decision-experiments/blob/main/publication/compact-shared-runtime-20261003/report.json)
records measurements, identities, caller workspace, allocation probes and scope.

Sampling seeds remain bound to the actual metadata/weight hashes. Same compact
artifact and seed reproduce the same choice; the same seed across expanded and
compact representations may choose differently. Missing models and unsupported
full input retain deterministic continuation with zero predictions.

The v0.2.14 release's 56 extracted files are pinned to research revision
`3fb6699e3b2ddf937a409cb415e1eb32d8e015fb`, manifest SHA256
`ccfbfdf1b4de51f2cf444369709fd8316767e8e3c731d5b5938b5f024c31aa99`.
The previous extraction is preserved in `source-provenance-v0.2.13.json`.
Model weights remain separate; this SDK release alone does not deploy a new
native compiler or select a default model.

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
the research repository. At that release, native compiler main
`1e01c96c54f2f8dd43334b8f580af93ffaea24df` used SDK 0.2.8 and rejected v2
before inference. Current compiler integration uses SDK v0.2.14-experimental;
see the compact native study linked at the top for its measured scope.

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

The CI workflow uses Go 1.27.2 for formatting, vet, unit tests, and race tests.
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
are separate from release provenance. At the time of this API's release, native
compiler main used SDK v0.2.7-experimental. Later compiler adoption is documented
in the current integration links at the top of this README.

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
their existing behavior. At that release, compiler main used SDK v0.2.5.

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
The session retains counters, the latest digest and, when applicable, one
condition counterexample, with caller-owned logs.

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
