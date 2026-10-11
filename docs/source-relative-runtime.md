# Use the source-relative decision model from Go

The SDK can now read the published source-relative research weights and use them
in its ordinary construction session. Gooo still owns the allowed source choices
and the supplied output/condition requirements. The model ranks the first choice;
the session checks actual candidate bodies and preserves incomplete results.

This API is available in the current repository checkout. The published compiler
0.6.28 has its earlier model loaders. Connecting this artifact directly to that
compiler CLI requires a later compiler integration; the commands below run the
SDK example against an already prepared typed plan.

## Inspect and construct

Use Go 1.27.2 from the SDK repository root. Replace `your-plan.json` with a typed
path document bound to your Gooo source:

```sh
GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./examples/contract-model inspect \
  -source-relative-source your-plan.json

GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./examples/contract-model search \
  -model studies/source-relative-learning-20261011/result/model-relative.json \
  your-plan.json

GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./examples/contract-model search \
  your-plan.json
```

The first command reports inspection schema v5, the complete 528 source cells,
condition-relation counts and availability/reason. It makes zero predictions,
training updates or candidate executions. `-condition-relations` remains useful
when only the 16 relation cells are needed.

The second command loads the original relative weights included with the study.
The same file is on [Hugging Face at the pinned publication](https://huggingface.co/asketeddy/gooo-canonical-decision-tiny-v1/blob/be6d95747d3a498620d0a5a6c42b4284c964cba2/research/source-relative-learning-20261011/result/model-relative.json).
Its SHA256 and model fingerprint are both:

```text
cae32bdad17ab818498c6099066918cd9c9c591eb118afe1bc6a7ef82dc83cc4
```

One prediction happens before any candidate execution. `initial.attempted` is
zero. Subsequent `Advance` calls use the same finite queue, with zero additional
predictions. Omitting the model uses the existing deterministic session.

A `PARTIAL` result retains the checked body and its failures. The example writes
that result and returns a nonzero exit when the budget ends before all supplied
requirements pass. Unsupported source shapes record a decline reason and make
zero predictions while continuing deterministic search. Malformed model files
return an error before search.

## Go API

Import `contractdecision` and `pathplan` from this module. Given a validated plan
`prepared`, its declared `cases`, and a bounded context `ctx`:

```go
raw, err := os.ReadFile(modelPath)
if err != nil {
    return err
}
model, err := contractdecision.DecodeSourceRelativeRequirementConditioned(raw)
if err != nil {
    return err
}
session, err := prepared.NewSourceRelativeRequirementContractSession(ctx, model, cases)
if err != nil {
    return err
}
initial, err := session.Observe() // No candidate executed yet.
if err != nil {
    return err
}
progress, body, err := session.Advance(ctx, 1) // At most one candidate; no reinference.
```

Use `progress` and the named failures to decide how to continue within the chosen
budget. Keep the original source binding when constructing a `pathplan.Document`.
The SDK validates the typed plan; the compiler owns its link to `.gooo` source.

`ContractInput.SourceRelativeOrderedSourceFeaturesInto` exposes the exact array
used by the session. It combines canonical source/intent/expressions with the
16 condition cells at `[192:208]`; `[208:384]` remain zero. Equivalent direct
returns and resolved assignments produce the same input. Failed preparation
preserves the caller-owned destination.

For separately planned future fits, `FitSourceRelativeRequirementConditioned`
accepts explicit bounded training samples and fit options. The public original
study is retained as a one-use observation; compatibility tests only decode and
re-encode its weights. They do not fit or predict on its corpus again.

## Scope and identity

The artifact keeps schema `gooo/source-relative-condition-research-model/v1`
and feature version `canonical_branch_intent_ordered_condition_relations_v1`.
It has the original 19,034 FP32 parameters and arithmetic-mean computation.
The public loader preserves its serialized bytes and fingerprint. Old artifact
formats stay on their own decoders. Source-relative weights require their exact
shape, metadata and numeric cells, including all reserved-channel constraints.

The input reader supports one root branch with integer `less_than`, `less_equal`
or `equal` atoms, and up to two source choices. It uses every declared condition,
including an empty list or the 128-row bound. [The input guide](source-relative-conditions.md)
explains the relation counts and unsupported shapes.

The session uses the SDK's existing lazy finite queue. The original study used
an explicitly sorted four-mask frontier. Their total candidate counts and timing
are different execution scopes; the published study does not measure this new
SDK route's performance or native Go execution. The model's related first-body
result was 64/128, as was the small rule's; current defaults remain unchanged.

Local fixtures cover exported-input/actual-prediction equality, one prediction
before candidates, no reinference, missing/128 conditions, partial contradictions,
concurrent sessions, cancellation, transactional arrays and CLI routing. They
exercise correctness of the connection, not a new model-quality measurement.
