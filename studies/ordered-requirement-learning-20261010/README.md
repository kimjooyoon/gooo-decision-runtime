# Ordered expressions reach the model; learning is still weak

This study makes two fresh local Go CPU fits and uses the saved, reloaded models
to assemble 48 new Gooo documents. It follows the earlier input-collision finding:
the model must receive the difference between `input - limit` and `limit - input`.
The new 528-cell input preserves that distinction, but this fixed training run
still learns little of the relationship between source, output and conditions.

## What was fixed before fitting

- Producer commit: `74a987d017aecc1bc3f72372c7bd864ffc42c895`.
- Compiler: `8a759cba557b6c6e22aeea57040e8e1c69e6ec68`, Go 1.27.2.
- Compressed corpus SHA256: `49e29239b9381627eddc01aaeb500541c0820eaecd4c607d739e34e5d953a96f`.
- Eight training documents: distance calculation, constant 37, direct statements,
  Korean intent, both source return orders, output goals and condition goals.
- Forty evaluation documents: constant 53 and English intent; 32 use direct,
  alias, assignment or rebinding forms, and eight use two root branches with
  128 condition cases, outside the new ordered representation's support.
- Each model trains once: mean pooling, 2,000 epochs, learning rate 0.3,
  L2 0.0001, seed 17. Results did not trigger another fit or setting change.

The [protocol](protocol.txt), compressed corpus and source-only preflight are
committed before fitting. Both models use the same eight labelled documents.
The sources instantiate a published template generator with fresh constants and
combinations. This is a narrow representation/goal comparison, with wording and
constant shifts occurring together; it does not isolate language understanding.

## Results

| Measure | Deterministic | Original requirement model | Ordered requirement model |
| --- | ---: | ---: | ---: |
| First complete candidate, training | 2/8 | 2/8 | 2/8 |
| First complete candidate, supported evaluation | 8/32 | 8/32 | 9/32 |
| First complete candidate, unsupported evaluation | 2/8 | 2/8 | 2/8, model declined |
| First complete candidate, all evaluation | 10/40 | 10/40 | 11/40 |
| Final complete program, all evaluation | 40/40 | 40/40 | 40/40 |
| First complete candidate, all 48 documents | 12/48 | 12/48 | 13/48 |
| Total candidate attempts | 120 | 120 | 117 |
| Model calls | 0 | 48 | 40 |
| Entire search time across 48 documents | 10.849ms | 17.077ms | 16.703ms |

Against the original requirement model, the new model improves the first result
in 13 documents and regresses in 12; the audit lists every regression. Its four
training condition-contrast pairs all receive the same proposal within each pair.
Across all 24 pairs, 23 receive the same proposal and none have both sides correct.

The original model's loss ends at 1.386295 and the new model's at 1.386295,
approximately `ln(4)`, the uniform four-candidate loss. The new input audit finds
zero conflicting groups among its 40 supported documents. The original input
has 24 conflicting pairs among all 48 documents. Preserving information has
removed one obstacle, while the fixed fit still fails to use that information.
One additional first success is insufficient evidence of a reliable advantage.

Gooo checks every candidate's authored outputs and intermediate conditions;
all three approaches complete every satisfiable document. The ordered model
declines eight unsupported sources before prediction and retains the same
deterministic frontier. Each supported model session makes one initial prediction,
checks its proposed candidate immediately, then performs zero later predictions.
No native binary execution or general correctness outside these cases is measured.

## Which relationship remains difficult?

For this exact distance template, the independent arithmetic audit confirms:

```text
comparison choice = condition goal
branch layout     = source reversal XOR output goal XOR condition goal
```

All 48 acceptable candidate sets agree with this relation. The final branch
layout depends on three things together. The original recorded proposals have
comparison correctness 25/48 and branch-layout correctness 24/48 for the new
model. Its training scores remain nearly tied. These observations motivate a
separate experiment in explicitly learning interactions or composing decisions;
they do not prove that the current nonlinear architecture cannot represent them.
The relation above is an audit of this benchmark, and is not injected into the
model or compiler as a shortcut.

The raw fraction of matching condition cases is also easy to misread: it is
1,102/1,144 (96.3%) on the first ordered-model candidates, because the rare sources
contain many similar conditions. Giving each document equal weight produces
73.5% mean condition completeness, while only 13/48 whole candidates satisfy
every requirement. The audit retains all three views and separate model coverage.

## Local cost

| Measure | Original requirement model | Ordered requirement model |
| --- | ---: | ---: |
| FP32 parameters | 13,282 | 17,890 |
| Weight array bytes | 53,128 | 71,560 |
| Public JSON artifact bytes | 155,896 | 209,728 |
| Training time | 804.743ms | 875.069ms |
| Prediction median | 29.042µs | 38.083µs |
| Prediction p95 | 74.416µs | 44.958µs |

The original prediction cohort includes eight 128-condition sources that the
ordered model declines, so the latency percentiles use different cohorts.
The complete process—source lowering, candidate labels, two fits, artifact
reloads and 144 searches—took 2.41s elapsed, 1.83s user CPU plus 0.14s system CPU,
with maximum RSS **30.59MiB**. That is approximately 0.82 core-equivalents averaged
over elapsed time, not a measurement of whole-PC utilization. Weight bytes are
distinct from process memory. The protocol uses CPU only and no cloud service.

## Use the recorded model and inspect the evidence

From this SDK checkout:

```sh
go run ./examples/contract-model search \
  -model studies/ordered-requirement-learning-20261010/result/model-ordered.json \
  studies/ordered-requirement-learning-20261010/examples/ordered-distance-k53-r0-direct-g0-c0.json

# Recheck the original records without fitting, predicting or executing candidates.
go run ./studies/ordered-requirement-learning-20261010/audit \
  studies/ordered-requirement-learning-20261010/result
```

The included examples contain both original `.gooo` text and its path document.
Omit `-model` for deterministic search. The new artifact uses
`gooo/ordered-requirement-contract/v1`; loading it in the native compiler remains
follow-up integration. Gooo 0.6.26 continues to use its published 0.2.37 choice
model. This result does not replace any default weights.

`result/` contains both weights, all 2,000-epoch histories, 192 complete candidate
labels, 144 search traces and process measurements. JSONL records are compressed
with `gzip -n` and byte-compared with the original outputs. The independent audit
checks exact int64 arithmetic (including values above 2^53), all ordered source
cells, every output/condition row, original model identities, training-only rows,
candidate/progress chains, one initial prediction and deterministic fallback.
Its regression tests deliberately corrupt bytes, outcomes, source expressions,
model identities and receipts. The result audit never re-runs the fits or models.
All published source and examples are synthetic; historical study records remain
unchanged. The study bundle is about 1MiB and excludes the producer executable.
