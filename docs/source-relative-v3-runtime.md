# Source-relative V3: declared output relationships

V3 adds source-relative output information to the existing small decision model.
The V2 consumer already reads every numeric output requirement through its
separate 32-cell case stream. V3 supplies a more direct relationship between those
goals, the predicate's operands and the resolved source constants.

The source input remains 528 float32 cells. Cells208..255 now contain three
16-cell groups for predicate operands less/equal/greater. Cells0..207 and256..527
retain the V2 representation. The numerical model still owns19,034 FP32 weights
(76,136 tensor bytes). A new artifact schema and feature identity distinguish V3;
V2 files and their original weights keep their existing interpretation.

## What each group records

| Cell within group | Authored input/output relationship |
| --- | --- |
| 0 | Fraction of output rows whose predicate operands fall in this group |
| 1..3 | Expected output equals/is less than/is greater than input |
| 4 | Expected output equals a resolved source integer literal |
| 5..7 | Expected equals input+literal, input-literal or literal-input |
| 8 | Expected equals input*literal |
| 9..10 | Expected equals input/literal or literal/input, with nonzero divisor |
| 11..13 | Addition, subtraction or multiplication wraps outside signed64 range |
| 14..15 | Literal is zero or input is zero |

All1..128 declared output rows and all distinct resolved predicate/return Int
atoms are counted. At most six atoms occur in the supported canonical shape;
equal literals are combined and sorted. Cells0..3 are divided by the total output
row count; the other cells by row-count times literal-count. Empty literal sets
leave those latter cells zero. Exact int64 comparisons and arithmetic precede
conversion of bounded counts to float32. Original numeric case bytes remain in
their separate stream.

Arithmetic matches the body's Go int64 behavior: division truncates toward zero,
MinInt64/-1 returns MinInt64, and add/subtract/multiply wrap. Wrap and zero flags
describe mathematical relationships of declared values; they do not assert an
executed program fault. No candidate body, probe input or native command is
evaluated while forming this view. Future finite search still checks the complete
original source-owned requirements.

## Go APIs and the example CLI

`ContractInput.SourceRelativeOutputs` exposes counters, sorted literals and
source/case identities. `SourceRelativeV3FeaturesInto` writes the full array.
`contractdecision.NewSourceRelativeV3`, `DecodeSourceRelativeV3`, and
`FitSourceRelativeV3` use the explicit V3 artifact. The path-plan session predicts
once before candidate execution; later advances make zero new predictions.
Omitting a model preserves deterministic finite traversal. Unsupported model
shapes retain their reason and deterministic fallback.

```sh
go run ./examples/contract-model inspect -source-relative-v3-source document.json
go run ./examples/contract-model search -model fresh-v3-model.json document.json
go run ./examples/contract-model search document.json
```

Inspection emits schema`gooo/contract-input-inspection/v7`, all original output
and Boolean rows, and the new output relationship view. Its model, candidate and
training counters remain zero. Select one inspection ABI at a time.

This SDK route uses an authored/source-derived JSON plan. The installed public
Gooo0.6.33 compiler consumes SDK0.2.42 and its V2 adapter. V3 compiler integration
and fitted public V3 weights require subsequent source and publication work.

## Current evidence and next fit

Fresh synthetic zero-weight tests cover inspection, strict artifact/version
separation, one initial prediction, finite original-body checks, deterministic
omission, exact large Integer handling, zero division, wrapping and caller-owned
immutable/concurrent reads. The original fitted studies and weights are retained.
V3 learned quality and speed have not been measured.

The [new V2 bilingual observation](https://huggingface.co/asketeddy/gooo-canonical-decision-tiny-v1/tree/168ea144404ef2719be1b529e877f606ecd2025d/research/bilingual-public033-20261012/result)
has5/8 first-mask pair agreement,5/16 first complete model documents and41/16
candidate checks versus18/8 fixed checks. All routes reach their finite checks.
Those eight evaluated contracts inform the next design; a future learned V3
study must predeclare fresh training/evaluation groups and retain first failures.
