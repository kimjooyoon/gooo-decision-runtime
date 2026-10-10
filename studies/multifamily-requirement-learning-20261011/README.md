# Broader training support for the same small decision model

Status: source corpus and producer prepared; no labels, fitting, predictions or
construction results have been collected for this study yet.

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
