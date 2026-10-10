# Keeping a rare case visible during path selection

A Gooo program can declare many examples while only one distinguishes two
possible bodies. Averaging all examples can shrink that signal. This experiment
changes how the local decision model combines them: each learned coordinate
keeps its largest absolute value, including its sign. The source, permitted
paths, training examples and parameter count stay fixed.

The model chooses among existing typed paths. Gooo assembles and checks the
chosen body immediately, then continues through the finite frontier if needed.
This uses one initial model call per source and no calls after failure.

## Observed results

The mean and deterministic columns use saved observations from the
[original 196-source study](../contract-goals-20261010/README.md). Only the new
signed pooling model was fitted and evaluated in this experiment.

| First path passes every declared output and condition | Deterministic, saved | Mean, saved | Signed pooling |
| --- | ---: | ---: | ---: |
| Training documents | 12/24 | 18/24 | 20/24 |
| Alias/assignment forms | 24/48 | 32/48 | 32/48 |
| Other constants | 36/72 | 50/72 | 52/72 |
| English descriptions | 12/24 | 12/24 | 12/24 |
| Unseen calculation family | 12/24 | 12/24 | 14/24 |
| Rare decisive case at position 128 | 1/2 | 1/2 | 2/2 |
| Contradictory contracts | 0/2 | 0/2 | 0/2 |

Among 170 satisfiable documents outside the original training set, first-path
validity changed from **107/170 (62.9%) to 112/170 (65.9%)**. Across all 196
documents, 11 improved, four regressed and 181 kept the same first-path validity.
The four regressions are distance-k7/k13/k11/k17-r0-w0-direct-g0; the
[audit](result/audit.json) names every improvement and regression.

Both learned modes completed all **194 satisfiable contracts** through finite
search. Both exhausted all four paths for each contradictory contract and left
it incomplete. Total candidate attempts fell from 271 to 264, including eight
attempts on contradictions in each mode. Saved deterministic search used 396.

Partial completeness is also recorded: the first bodies passed **1,582/2,048
output examples**, compared with the mean model's 1,533/2,048. Both passed
588/588 declared intermediate conditions on their first bodies. The long
128-case contracts have more weight in these per-example totals; per-document
first validity above gives each program equal weight. Passing the authored
examples does not establish behavior on all possible inputs.

## What remains difficult

Among 97 opposite-goal pairs with identical initial source arrays, pairs where
both first choices were valid rose from 32 to 35. However, **pairs receiving the
same proposed path rose from 61 to 62**. All 97 pairs had different scores in
both models; different scores often still lead to the same choice. English
description transfers remained 12/24. The case summary is still lossy, and
extreme pooling can discard frequency and allow an outlier to dominate.

This is a controlled comparison on a **known corpus**. The aggregation change
was motivated by the previous results, and templates are shared across splits.
The numbers show the effect of this particular change, rather than new unseen
language accuracy. No general Korean/English comprehension claim follows.

## Local cost

| Measurement | Observed |
| --- | ---: |
| One CPU fit, 24 documents, 600 epochs | 336.128 ms |
| Prediction median / p95 | 16.500 / 19.041 µs |
| Sum of 196 new-model sessions | 28.125 ms |
| Whole process wall / user / system time | 0.93 / 0.46 / 0.02 s |
| Peak process RSS | 29,687,808 bytes (28.31 MiB) |
| Parameters / FP32 weight payload | 9,746 / 38,984 bytes |
| Model JSON | 114,013 bytes |

Rounded CPU times correspond to about 0.52 cores on average for this process.
The host utilization increase was not measured. There were no GPU, external
model, native generated executable or feedback calls. The original mean study
included collection and additional work, so its process RSS and timings cannot
establish an isolated pooling speedup. Saved mean session time was 27.837 ms.

## Use and provenance

From the SDK root, run a chosen recorded document with this explicit model:

```sh
go run ./examples/contract-model search \
  -model studies/contract-pooling-20261010/result/model.json \
  studies/contract-goals-20261010/examples/offset-k11-r1-w0-assignment-g0.json
```

The API and schema are described in [the pooling guide](../../docs/contract-pooling.md).
The ordinary compiler's local declared-case integration currently loads v1;
this v2 computation is available through the SDK and example command. Default
`New`/`Fit` and the published v1 weights retain mean pooling.

- [Protocol](protocol.txt) and clean producer revision
  `3193c960c61d315c51e7fea27f4b82bc02b34086` were fixed before the one execution.
- Exactly one fit: 600 epochs, learning rate 0.3, L2 0.0001, seed 17; exactly
  196 new-model forwards and searches. The original fit/oracle was not repeated.
- All 24 training rows, 600 losses, new weights and 196 searches are retained in
  [result](result), including wrong outputs and progress chains.
- Model SHA256: `f69944394dbe66b3449c5f87cb5ee51b09a9f34c4a2483e5fc119a005160e8b7`.
  Schema: `gooo/contract-candidate-decision/v2`, pooling: `signed_max_abs`.
- [Checksums](result/SHA256SUMS) bind the published result files. The read-only
  [audit](audit) checks the frozen source corpus and training membership, exact
  integer arithmetic, oracle agreement, score choices, progress and counts.
  It never fits, predicts, lowers sources or executes bodies. Mutation tests
  reject one-unit errors above 2^53 and changed source goals/rankings.

The next useful question is how declared goals influence the final choice when
source and wording scores compete with case signals. This experiment preserves
the rare-case improvement and documents the remaining decision errors.
