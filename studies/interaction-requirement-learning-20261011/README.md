# Small interaction model: better first choices, with a higher inference cost

This study fits two models locally in Go and uses their saved, reloaded files to
assemble 80 fresh Gooo documents. The interaction model learns a useful relation
between source order, requested outputs and intermediate conditions in this
distance-calculation family. It still misses four supported evaluation documents,
and its additional inference work makes total search time slightly longer than
the previous model in this small workload.

## Fixed before fitting

- Producer commit: `be22d1de09d23e8f68ae6ad6f2237281836b9233`.
- Compiler: `8a759cba557b6c6e22aeea57040e8e1c69e6ec68`, Go 1.27.2.
- Compressed corpus SHA256: `7fadf92cbd8c483e8a84fd8af3e0a6bd8a49dc2d19dfa51755cc26d6c198ea20`.
- Training: eight documents, constant 41, direct statements, Korean branch
  intent, both source return orders, output goals and condition goals.
- Supported evaluation: 64 documents, constant 67, Korean/English branch intent,
  direct statements, aliases, assignments and rebinding, with the same eight
  goal combinations per group. The comparison-choice intent remains bilingual
  in every source; this is a change in one intent sentence.
- Unsupported evaluation: eight two-root-branch sources with 128 conditions.
  Both models decline them before inference and retain deterministic search.
- One fit per model: mean pooling, 2,000 epochs, learning rate 0.3, L2 0.0001,
  seed 17. Both use the same training rows and complete Gooo-labelled candidates.

The [protocol](protocol.txt), corpus, producer and source-only preflight were
committed before collecting labels or fitting. All 320 complete candidate masks
were labelled through Gooo. Both models read the same 528 source fields and all
original output/condition rows. Features, activations, interactions, parameter
count and the numeric rule differ, so the comparison concerns the whole models.
These are fresh instances of a published template; unseen algorithms and broad
natural-language understanding need separate evaluation. No settings changed
after viewing these results, and historical fits and observations remain intact.

## Results

| Measure | Deterministic | Ordered requirement | Interaction requirement |
| --- | ---: | ---: | ---: |
| First complete candidate, training | 2/8 | 4/8 | 8/8 |
| First complete candidate, supported evaluation | 16/64 (25%) | 20/64 (31.25%) | 60/64 (93.75%) |
| First complete candidate, unsupported evaluation | 2/8 | 2/8, declined | 2/8, declined |
| First complete candidate, all evaluation | 18/72 | 22/72 | 62/72 |
| Final complete program, all evaluation | 72/72 | 72/72 | 72/72 |
| First complete candidate, all documents | 20/80 | 26/80 | 70/80 |
| Attempts, supported evaluation | 160 | 151 | 68 |
| Attempts, all documents | 200 | 185 | 96 |
| Model calls | 0 | 72 | 72 |
| Search time, supported evaluation (sum) | 7.318ms | 14.332ms | 14.758ms |
| Search time, all documents (sum) | 13.644ms | 21.153ms | 22.062ms |

Against the ordered model, the interaction model improves 44 first choices,
regresses in zero and leaves 36 unchanged across all 80 documents. Forty of those
improvements occur in the supported evaluation set. Both models have zero full
input collisions among their 72 supported training/evaluation sources.

All first-choice failures in the 64 supported evaluation sources occur in four
English-branch-intent assignment documents. Their comparison choices are correct,
but their selected branch layouts produce the wrong outputs. The other seven
wording/form groups reach 8/8 each; this group reaches 4/8. These failures remain
in the records, including a [readable failed-first example](examples/interaction-distance-k67-r0-w1-assignment-g1-c0.gooo).
They motivate a separate study of sensitivity to wording and equivalent source
forms; this observation alone does not establish the causal input feature.

The interaction model distinguishes opposite condition goals in every supported
pair. Both sides succeed first in 34/36 supported pairs, including 30/32 evaluation
pairs. All 64 supported evaluation comparison choices are correct. Gooo checks
both outputs and conditions after the initial proposal and continues through
the finite candidates. All documents complete their authored cases. This is
case-bounded completeness; behavior on arbitrary integer inputs is unmeasured.

The first-candidate case fractions also remain separate from whole-program
success: the interaction model passes 580/640 outputs (90.625%) and 1,236/1,240
conditions (99.677%), while 70/80 whole candidates meet every requirement.
The records retain per-document completeness and equal-document macro sums, so
the many similar rare conditions cannot silently change the denominator.

## Cost on this PC

| Measure | Ordered requirement | Interaction requirement |
| --- | ---: | ---: |
| FP32 parameters | 17,890 | 19,034 |
| Weight array bytes | 71,560 | 76,136 |
| JSON artifact bytes | 209,663 | 223,039 |
| CPU training | 814.161ms | 2,023.893ms |
| First loss → last loss | 1.413764 → 1.373845 | 1.404026 → 0.001845 |
| Prediction median | 34.334µs | 98.458µs |
| Prediction p95 | 39.083µs | 112.916µs |

Both latency cohorts contain the same 72 supported documents. The complete
process—lowering, labels, two fits, artifact reloads and 240 searches—takes
**3.63s elapsed**, 3.08s user CPU plus 0.12s system CPU, with **35.56MiB maximum
RSS**. This machine has an Apple M4, 10 logical CPUs and 16GiB RAM. GPU and cloud
compute were unused. CPU time divided by elapsed time is about **0.88 CPU cores**
on average, or 88% of one core. Whole-PC utilization was not sampled.

This is one fixed-order pass, with possible warm-up and background-load effects.
The model spends more time predicting while checking fewer candidates. On these
cheap bodies, the smaller search does not recover the extra inference cost.
More expensive candidate work may change that balance; it needs measurement.
Weight bytes are separate from process memory. Native execution, quantized
quality and model-free replay performance are outside this study.

## Try the recorded model

From this SDK checkout:

```sh
GOWORK=off go run ./examples/contract-model search \
  -model studies/interaction-requirement-learning-20261011/result/model-interaction.json \
  studies/interaction-requirement-learning-20261011/examples/interaction-distance-k67-r0-w0-direct-g0-c0.json

# Inspect original records only: zero fits, predictions or program executions.
GOWORK=off go run ./studies/interaction-requirement-learning-20261011/audit \
  studies/interaction-requirement-learning-20261011/result
```

The [model guide](../../docs/interaction-requirement-contract.md) explains the
computation and SDK API. These study weights have their own explicit schema.
Native compiler loading and a versioned model release remain follow-up work.

| File | SHA256 |
| --- | --- |
| [Interaction model](result/model-interaction.json) | `4b61fe8d84df77c8dfac6a880eedcddfd8f5f903fa538d1e80532dd77f74b1ac` |
| [Ordered comparator](result/model-ordered.json) | `9f2901905b0cd663ea61d678a0993e6f6c2abee3aef7689b1d00faa91e3f857d` |

The result directory contains every original source, array, training row,
candidate, selection, progress receipt and failure. Large JSON logs use
deterministic gzip with decompressed-byte comparisons. The independent audit
checks exact-int64 arithmetic, candidate labels, all rows, source and model
identities, training-only inclusion, call counts and metric recounts. It never
fits, predicts, lowers or executes a program. Mutation regressions check that
altered cases, labels, weights and receipts are rejected.
