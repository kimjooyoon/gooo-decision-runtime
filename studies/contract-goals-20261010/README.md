# Learning a path from the declared examples

This study asks whether Gooo's own input/expected-output examples can guide a
small local decision model before a candidate runs. Each pair has the same body,
choices and neutral description, with different required outputs. The model
reads all examples, chooses a permitted path, and Gooo checks the resulting body.

The four families cover bounds, signed distance, offsets and constant tags.
Bodies use direct returns, aliases and assignments. Only 24 direct-return
Korean-description documents enter training. Other forms, constants, English
wording and the tag family remain outside training. Two 128-case contracts hide
their decisive case at the end; two others contain contradictory requirements.
All 196 original `.gooo` sources, lowered documents and outcomes are retained.

## What happened

| First path satisfies every declared output and condition | Deterministic | Model |
| --- | ---: | ---: |
| 24 training documents | 12/24 | 18/24 |
| 48 alias/assignment transfers | 24/48 | 32/48 |
| 72 held-out-constant documents | 36/72 | 50/72 |
| 24 English-wording transfers | 12/24 | 12/24 |
| 24 unseen-family documents | 12/24 | 12/24 |
| 2 rare-tail contracts | 1/2 | 1/2 |
| 2 contradictory contracts | 0/2 | 0/2 |

Among the **170 satisfiable documents outside training**, first-path validity
rose from **85/170 (50%) to 107/170 (62.9%)**. Across the whole collection, 49
documents improved and **21 regressed**; the audit lists every regression.
The training documents themselves reached 18/24 on first selection.

Both modes eventually completed all **194 satisfiable contracts**. Neither
reported either contradictory contract as complete. Complete means all authored
finite examples and conditions passed; behavior on other inputs remains open.
Both modes checked all 128 cases in the tail and contradiction examples.

The full collection needed **396 deterministic candidate attempts versus 271
with the model**, including eight attempts on contradictions in each mode.
The model made exactly 196 forward calls, once before each model session.
There were no post-failure calls. Candidate assembly and checking followed each
selection immediately, through the ordinary finite frontier.

## Where reading the goals still falls short

All 97 satisfiable paired goals had identical initial source arrays and
different model scores after reading the cases. Yet **61 pairs selected the same
path for both goals**. Of the other pairs, 32 selected two correct paths and four
selected two incorrect paths. Every English-wording pair and unseen-family pair
kept the same proposal, as did the rare-tail pair.

This distinguishes receiving the case information from using it successfully.
The eight-value mean is a lossy summary, and one decisive case among 128 has
limited weight in that mean. These observations motivate checking how case
signals interact with source/intent scores. This experiment does not isolate
pooling, training duration, wording or architecture as the sole cause.

## Local cost

| Measurement | Observed |
| --- | ---: |
| One CPU fit, 24 documents, 600 epochs | 356.692 ms |
| Prediction median / p95 | 16.375 / 18.791 µs |
| Sum of all deterministic / model sessions | 18.278 / 27.837 ms |
| Whole process wall / user / system time | 1.47 / 0.63 / 0.13 s |
| Peak process RSS | 51,412,992 bytes (49.03 MiB) |
| Parameters / FP32 weight payload | 9,746 / 38,984 bytes |
| Model JSON | 114,032 bytes |

Fewer attempts did **not** make this workload faster: summed session time grew
about 52%. Sources, labels and weights were already in memory during session
timing; loading the file is additional. Deterministic sessions ran first for
each source, so ordering is also a confound. Prediction time includes projecting
declared cases through the model interface, while session time includes source
projection, receipts and checks. The whole process includes collection and fit.

Rounded CPU times correspond to about 0.52 CPU cores on average. A host CPU
utilization delta was not measured. No GPU, external model or native generated
executable was used; bodies ran in the existing Gooo body evaluator.

## Try the published model locally

Run from the SDK root:

```sh
go run ./examples/contract-model search \
  -model studies/contract-goals-20261010/result/model.json \
  studies/contract-goals-20261010/examples/offset-k11-r1-w0-assignment-g0.json

go run ./examples/contract-model search \
  studies/contract-goals-20261010/examples/offset-k11-r1-w0-assignment-g0.json
```

The example directory includes four original `.gooo` sources beside their exact
lowered JSON documents. They were copied from the recorded collection without
recompiling or executing it. These commands consume the documents; the ordinary
released compiler CLI does not yet load the contract-model schema. The generic
local `fit` command can train new explicitly supplied documents separately.

## Reproducibility and scope

- [Protocol](protocol.txt) and clean producer commit
  `addd45c2b2a5c9a76ab5029174cfe1b9189f2275` were fixed before execution.
- Compiler source: `89b000d046ab706bb7d61b91148aa4fd805821f6`, Go 1.27.2.
- One fresh fit: 600 epochs, learning rate 0.3, L2 0.0001, seed 17.
  No retuning or repeated observation was performed.
- [Model](result/model.json), [report](result/report.json),
  [audit including regressions](result/audit.json), [checksums](result/SHA256SUMS).
- Model SHA256:
  `c53ef0386dfc8ee2422b2b9605b609b0c23315905599ecc2f4555c26de514b5f`.
- Artifact schema: `gooo/contract-candidate-decision/v1`. It is a finite-decision
  model read by `contractdecision.Decode`, with no text tokenizer or decoder.
- All 784 oracle candidates, 24 training inputs/label sets, 600 losses and 392
  searches are preserved. Integer values never pass through float64.
- The read-only audit checks source/case identities, exact mathematical results,
  training membership, score-to-choice mapping, progress chains, finite outcomes,
  timings and all counts. It does not rerun the compiler, model or body evaluator.
  Mutation tests reject one-unit changes above 2^53 and altered cases/receipts.

Templates are shared across splits, and the cases remain small. These are
measured finite-contract results; broad Korean/English understanding and more
complex Gooo programs still need separate evaluation. The model remains an
explicit experiment. Existing published models and deterministic execution
retain their own behavior.
