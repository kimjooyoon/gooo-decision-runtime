# V4: keep each return branch's output relationships

V4 is an optional SDK model/input contract added after SDK 0.2.45. It gives the
small decision model a separate view of the literals owned by each return branch.
The Gooo source still supplies the available choices and required behavior.

The [frozen-input diagnosis](../studies/factorized-input-diagnosis-20261012/README.md)
found a layout-specific gap in the earlier neutral model: one required raw mask
had 0/128 complete first choices. Exact raw inputs had no conflicting accepted
masks, so that analysis did not establish an unavoidable input collision. V4
makes one relationship more explicit for a future fit. Its decision quality has
not yet been measured and no fitted V4 weights are published with this change.

## A small example

Consider a body with `input / 5` in its then branch and `31 - input` in its else
branch. V3's output relation channel combines the source literals `{5, 31}`.
V4 retains that channel and also records `{5}` for then and `{31}` for else.
It is like keeping the labels on two trays of parts while assembling a function.

For each declared input/output pair, the new view compares the expected output
with both branches' literal sets. It calculates all the existing arithmetic
relationships; it does not choose an operator, select a mask, assemble a body or
read any observed candidate result. The model still has to combine source
operators, condition goals and these relations when choosing the first path.
Held-out probes and candidate successes never enter feature preparation.

## Array contract

The source input stays at 528 float32 values per choice. Existing V3 prose-neutral
channels stay at their original offsets, including the exact ordered-expression
suffix. Previously reserved cells `[256,352)` now contain 96 values:

```
offset = 256 + branch*48 + predicate_region*16 + relationship_slot
branch:          0 = then, 1 = else, in the canonical fallback-oriented source
predicate_region: 0 = left<right, 1 = left==right, 2 = left>right
```

Each branch's literal set contains at most two distinct exact int64 atoms from
its resolved return expression. Predicate-only literals do not join that set.
An input-only return has an empty set and zero literal-relation counts.

Slots 0..3 are the case count and counts of expected output equal to, less than,
or greater than input. Slots 4..15 are the existing V3 relations, in order:
literal equality, input+literal, input-literal, literal-input, product,
input/literal, literal/input, addition overflow, either subtraction overflow,
multiplication overflow, literal zero, and input zero. Arithmetic comparisons use
the existing int64 wrap/division semantics. Undefined division contributes no
quotient match. Counts 0..3 divide by the total declared case count; counts 4..15
divide by that count times the owning branch's distinct literal count.

Preparation retains all 1..128 declared output rows. Arrays hold counts/features;
the model remains at 19,034 FP32 parameters and 76,136 tensor bytes. This states
layout sizes only; no new latency, allocation, CPU or process-memory result has
been collected.

## Inspect and construct

From this checkout, inspect the bundled authored SDK fixture without a model:

```sh
GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./examples/contract-model inspect \
  -source-relative-v4-source examples/contract-model/testdata/source-relative-v2-division.json
```

The `gooo/contract-input-inspection/v9` report includes each branch's literals,
counts, source/case identities and the exact array. It reports zero model calls,
candidate executions and training updates. Select this format on its own.

The Go input API is `ContractInput.BranchOutputs` for the explained view and
`ContractInput.SourceRelativeV4FeaturesInto` for the array. The model APIs are
`contractdecision.NewSourceRelativeV4`, `DecodeSourceRelativeV4` and
`FitSourceRelativeV4`. Training requires separately prepared V4 arrays and
source-owned labels; existing V3 tensors are not converted to fitted V4 weights.

Once a separately trained artifact is available, pass it to the ordinary SDK
`search -model <artifact> <document>` command or
`PreparedPlan.NewSourceRelativeV4ContractSession`. The artifact schema is
`gooo/source-relative-branch-output-model/v4`, with an explicit branch relation
version and declared-requirements-only intent policy. Older decoders reject it.

The session predicts once before any candidate execution, records the arrays'
digests, and advances the original finite candidates without more predictions.
Omitting the model retains deterministic search; unsupported source shapes keep
that route with an explicit decline reason. Invalid supplied artifacts return an
error. The public compiler 0.6.34 does not load this new SDK profile yet.

## Evidence in this change

Synthetic regressions cover branch swaps, literal-free returns, every declared
row, exact source identities, concurrent readers, invalid-destination preservation,
artifact identity, and one-prediction search through the original bodies. These
exercise new zero/synthetic weights. They are implementation checks, separate
from a held-out model-quality study. A new quality study must publish its tasks,
split, fit settings and evaluation scope before fitting or predicting.
