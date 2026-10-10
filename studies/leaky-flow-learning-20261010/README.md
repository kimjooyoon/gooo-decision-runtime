# Keeping negative signals in the small Gooo decision model

The preceding diagnostic found that some opposite intents reached the same
all-zero branch representation. This experiment changes the activation to a
fixed negative slope 0.01. The input arrays, parameter count, initialization,
16 training rows and 400-epoch schedule stay the same. One new model was trained;
the previous reports are immutable historical baselines.

## What improved, and what regressed

| First choice satisfies all declared cases | Prior v5 ReLU | Prior v6 ReLU | New v6 leaky |
| --- | ---: | ---: | ---: |
| 16 direct training forms | 10/16 | 8/16 | 14/16 |
| 64 other forms of the same tasks | 40/64 | 32/64 | 56/64 |
| 80 forms with held-out constants | 50/80 | 40/80 | 70/80 |
| 2 unsupported-shape controls | 1/2 | 2/2 | 1/2 |
| Total | 101/162 (62.35%) | 82/162 (50.62%) | 141/162 (87.04%) |

Against the original ReLU v6, 80 sources improved, 21 regressed and 61 kept the
same first-choice validity. All 80 max tasks now passed; min tasks fell from
80/80 to 60/80. The 20 min regressions share the original return-arm order and
wording 0, across four constants and five equivalent source forms. The remaining
regression is the nested-branch control. Every failed selection is retained.

The first candidates passed 1149/1296 output cases (88.66%) and 486/486 condition
cases. Both bounded search modes completed 162/162 declared suites, with 183
candidate attempts each, compared with 242 for historical ReLU v6 and 223 for
historical v5. Feedback added 21 model calls and did not reduce attempts in this
run. Candidate reuse checks found no repeated execution of an attempted mask.

These are related finite fixtures. Representation transfer shares underlying
tasks with training, and constant transfer shares program templates. The
percentages measure declared-case behavior, not general language understanding
or correctness on all possible inputs.

## What the internal observations show

All 32 direct-source rows retained negative branch activations. None of the 16
opposite-intent direct pairs had equal branch hidden vectors or option scores;
none had two all-zero branch vectors. The prior v6 diagnostic had eight such
all-zero pairs. Across all 80 opposite-intent pairs, none now had exactly equal
whole prediction distributions. All 128 equivalent-form pairs still had exactly
equal distributions.

This is consistent with the negative-slope change preserving intent distinctions
that ReLU had removed in those recorded paths. The min regressions show that
retaining a distinction does not guarantee the right selection. This single
seed and schedule do not establish stability over other training runs.

## Local cost

| Measurement | Observed |
| --- | ---: |
| One CPU fit, 16 samples and 400 epochs | 133.21 ms|
| First judgment with diagnostic trace, median / p95 | 14.458 / 16.083 µs|
| Parameters / FP32 weight bytes | 9,290 / 37,160 |
| Saved JSON artifact | 108,562 bytes|
| Whole producer wall / user CPU / system CPU | 0.75 / 0.29 / 0.03 s|
| Peak process RSS | 21,397,504 bytes (20.41 MiB)|

The whole process averaged about 0.43 CPU cores from its rounded process times.
Host utilization change was not measured. There were 162 first diagnostic
forwards and 345 search forwards, 507 calls total. No native subprocess or GPU
was used. Judgment timing includes `ExplainInto`'s trace copy; earlier studies
timed bare `PredictInto`. Historical timings are not a controlled speed comparison.

## Computation identity and integration

The model uses the existing v6 feature ABI and a new explicit computation
contract: `gooo/flow-candidate-decision/v2`, activation `leaky_relu_0.01_v1`.
Negative signals are multiplied by a fixed FP32 0.01. The derivative at zero
uses that slope. Old ReLU files keep their original bytes, fingerprints and
behavior. Schema/activation mismatches are rejected. A feedback session also
rejects substituting the same weights under another computation.

`SearchFlowBatches` consumes this model directly while Gooo plans retain the
choices, intent and cases. The nil-model search remains deterministic. The
compiler CLI needs a later SDK upgrade and schema dispatch change before it can
load this new artifact. This publication does not change the installed compiler.

## Original evidence

- Fixed [protocol](protocol.txt); clean Go 1.27.2 producer
  `91d3f58e4f80f2a71b7812ec489125ee17d8d78c`.
- [Model](result/model-leaky_v6.json), [report](result/report.json),
  [ReLU baseline](result/baseline-report.json), [read-only audit](result/audit.json).
- The 16 training arrays are byte-identical to the original v6 sample file:
  SHA256 `e62fd490c2224aef24c25906e0768e666390d8e4f1b6381258bcb24af64251c4`.
- All 400 losses, 162 predictions/traces/bodies and 324 complete search records are
  published with [SHA256SUMS](result/SHA256SUMS). Compression uses gzip-n and
  decompressed byte comparisons. No previous experiment was rerun.
- The read-only audit checks int64 outputs, source cases, model/input identity,
  traces, every attempt and all aggregates. It makes no model call or candidate
  execution. Unit tests cover negative gradients, finite differences, old-model
  bytes, artifact mismatch, runtime routing and feedback identity.
