# Broader training support for the same small decision model

Status: one original CPU fit and all 768 construction searches completed.
The protocol and corpus were published at
[`c9b31dad`](https://github.com/kimjooyoon/gooo-decision-runtime/commit/c9b31dad9b45e66b5fa3c39ceb0e48e51b0aaf4c)
before collecting labels or fitting. The anonymous public protocol matched the
local bytes before execution. The original execution has not been repeated.

The [published wording experiment](https://github.com/kimjooyoon/meta-ontology-go/wiki/Wording-and-Variable-Stability)
found that equivalent variable forms kept identical model inputs, while changing
Korean/English descriptions changed scores and sometimes the first chosen path.
The released interaction model had been trained on eight distance-calculation
documents with one Korean branch-intent sentence. It was being used on a new
batch-rule family in that observation.

This follow-up keeps the same19,034-parameter FP32 architecture and76,136-byte
weight array. It adds training support for two rule families and two languages,
then compares a fresh model with that untouched released model and deterministic
ordering. It uses Go and local CPU computation. A higher score is an open
question; model size and training-row counts are not evidence of useful behavior.

## Frozen design

| Cohort | Documents | Constants | Wording and body | Fit use |
| --- | ---: | --- | --- | --- |
| Training | 64 | 29, -17 | Korean/English, direct returns, bound/distance families | All64 |
| In-family evaluation | 128 | 53, -37 | Two new Korean/English wordings, direct returns/assignments | None |
| Family evaluation | 64 | 53, -37 | Same evaluation forms; negative-bound family | None |

Each group includes both initial return orders, both output goals and both
intermediate-condition goals. Comparison and branch descriptions both change
with wording. The original cap24 cohort and its sentences are excluded.
These are related template instances; group counts do not represent independent
business tasks or a general language-understanding benchmark.

The [protocol](protocol.txt) fixes one new fit (2,000 epochs, learning rate0.3,
L2=0.0001, seed23), all256 sources, all1,024 candidate masks and all768 searches.
Only64 rows enter fitting. The remaining192 rows are retained for evaluation and
may enter collision audits; they never select settings or trigger another fit.

The corpus is compressed JSONL. The source-only preflight checks source identity,
language lowering, exact integer goals, the528-cell input format, condition-only
contrasts and the train/evaluation boundary. It performs no candidate execution
or learning. The producer requires a clean pinned compiler checkout, a clean
committed producer and the exact released comparator hash before it creates a
fresh output directory. Existing output directories are rejected.

## What the result must retain

Keep every source, source-derived feature array, candidate result, training row,
epoch loss, saved weight artifact and search progress. Report first-candidate
completeness, outputs, conditions and final bounded completion separately for
training, in-family evaluation, family evaluation and wording/form groups.
Keep CPU fit cost, prediction latency and total search cost separate.

Failures and incomplete observations stay in the record. The interpreter-level
study does not itself establish native CLI behavior, calibrated probabilities,
quantized quality, external adoption or speedup under controlled load.
No original model fit or prior observation is repeated.

## Observed result

First success means that one proposed body satisfies **all eight authored
outputs and all three intermediate conditions**. A correctly shaped return
value alone does not qualify.

| Cohort | Fixed order | Released interaction model | Fresh broader model |
| --- | ---: | ---: | ---: |
| Training | 16/64 | 40/64 | 64/64 |
| New instances in trained families | 32/128 | 72/128 | 117/128 |
| Family excluded from both fits | 16/64 | 33/64 | 34/64 |

The fresh model improved 49 in-family first choices and regressed four relative
to the released model. For the excluded family it improved nine and regressed
eight. All three policies eventually completed all 256 documents within four
candidates each. This finite completion checks the authored requirements; it
does not establish correctness for arbitrary inputs or a business deployment.

The in-family misses all occurred in assignment bodies: Korean 27/32 and
English 26/32 first successes, versus 32/32 in each direct-return group.
The excluded family still had 30/64 first failures. Its first output checks
fell from 309/512 to 302/512, even though complete first successes rose by one
and first condition checks improved from 186/192 to 192/192. These measures
describe different properties and stay separate in the report.

On in-family equivalent-wording pairs, both first proposals passed in 57/64
pairs, compared with 31/64 for the released model. On equivalent-form pairs the
fresh model chose the same mask in 53/64 pairs. The source-feature audit found
32 repeated complete-input groups and zero conflicting-label groups. These
observations do not make every source presentation invariant.

The [read-only audit](audit/) checks exact int64 arithmetic for every retained
candidate, feature encodings, accepted masks, training-only rows, artifact
identities, progress chains and cohort counts. It reports every regression in
[`result/audit.json`](result/audit.json). It does not run the model or compiler.

## Local cost

| Measurement | Observation |
| --- | ---: |
| Fresh CPU fit, 2,000 epochs / 64 rows | 16.116 seconds |
| Fresh model prediction median / p95 | 99.125 / 110.666 microseconds, n=256 |
| Released model prediction median / p95 | 98.417 / 110.458 microseconds, n=256 |
| Whole producer wall / user+system CPU | 17.20 / 16.76 seconds |
| Whole producer maximum RSS | 58.797 MiB |
| Parameters / raw FP32 weights | 19,034 / 76,136 bytes |
| Saved model JSON | 222,984 bytes |
| Candidate counts: fixed / released / fresh | 640 / 409 / 297 |
| Sum of construction durations: fixed / released / fresh | 26.796 / 64.856 / 60.019 ms |

The new model reduced checked candidates, while fixed ordering still used less
total construction time for these cheap arithmetic templates. Source collection
and fitting are outside the construction-duration sums. The whole process used
about 97.4% of one CPU core on average (CPU seconds / wall seconds); this is not
an instantaneous machine-wide utilization increase. There were no GPU or cloud
jobs. Policy order was fixed and host load was uncontrolled. No causal speedup
or customer cost reduction follows from these numbers.

## Inspect and reuse

Run the retained-record checks from the repository root with Go 1.27.2:

```sh
GOWORK=off GOTOOLCHAIN=go1.27.2 go test ./studies/multifamily-requirement-learning-20261011/audit
GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./studies/multifamily-requirement-learning-20261011/audit \
  ./studies/multifamily-requirement-learning-20261011/result
```

The [model artifact](result/model-multifamily.json) uses the existing
`gooo/interaction-requirement-contract/v1` SDK interface. Its SHA256 is
`871a829633e8763d86091d384816f4452353fa96acc10cfa4e57d187299b8d5d`.
It was initialized and trained in Go from this Gooo corpus; it uses no Laya
weights. Loading it does not require a Python runtime or a hosted inference
service. It consumes a bounded source representation and declared examples.
Natural-language descriptions are one input channel; this study measures
neither free-form language generation nor general language understanding.

The comparator is reused by hash from the preceding study's retained artifact.
The current producer requires compiler source `2e0e34f8` and its matching local
SDK checkout; it is a record of the original experiment, not an installation
entry point. Native adoption of these new weights is still unobserved. Public
Gooo 0.6.27 uses SDK 0.2.37; the interaction adapter is a subsequent development
change. HF publication is pending authentication.

For product development, the next useful question is whether a developer can
change a real small rule, understand its changed behavior and reuse the saved
program with less work. The broader fit provides evidence for supported
families, and its remaining assignment/family failures identify concrete gaps.
The original records stay fixed; future corpus or representation changes need
their own evaluation. External adoption, willingness to pay and support cost
remain unmeasured.
