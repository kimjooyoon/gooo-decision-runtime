# Selecting a Gooo path after an actual failed attempt

The [observed-context model](../feedback-flow-learning-20261010/README.md) passed
all first judgments in its finite evaluation. That left its behavior after a
failure unmeasured. This study starts each evaluation source from every declared
wrong configuration and asks both frozen models for the next choice.

There are 146 held-out sources and 438 wrong starts. Each start is actually
compiled and evaluated once. Both models receive the same resulting condition
and output observation. Each makes one forward pass, then the highest-scored
untried configuration is immediately compiled and checked on eight output and
three condition cases. A raw recommendation of the already failed configuration
is recorded separately. Ties use ascending mask order. This diagnostic ranks
all three remaining configurations explicitly.

## What the observations show

| Across 438 failed starts | Initial-only trained model | Observation-trained model |
| --- | ---: | ---: |
| Raw recommendation satisfies every case | 327/438 | 438/438 |
| Raw recommendation repeats the failed start | 19/438 | 0/438 |
| Next untried configuration satisfies every case | 346/438 | 438/438 |
| Output cases of that configuration | 2860/3504 | 3504/3504 |
| Condition cases of that configuration | 1314/1314 | 1314/1314 |

The 92 failed next configurations from the old model all passed the condition
cases while returning wrong outputs. Observing the expected branch condition
alone does not establish the resulting program's behavior.

We also applied the identical failed-mask exclusion to each model's **saved
initial scores**. This read-only counterfactual uses previously recorded
candidate outcomes and introduces no new model call or execution.

| Does receiving feedback improve the next choice? | Initial-only trained | Observation-trained |
| --- | ---: | ---: |
| Saved initial ranking, same failed mask excluded | 400/438 | 438/438 |
| Ranking after actual failure observation | 346/438 | 438/438 |
| Improved / regressed / unchanged validity | 0 / 54 / 384 | 0 / 0 / 438 |

Feedback caused 54 next selections to become invalid in the old model: 24 among
equivalent source forms and 30 among held-out constants. The new model preserved
its successful choices after all observed failures. It already reached this
panel's ceiling before feedback, so additional corrective benefit remains
unestablished. These data support stability under these failure observations.

## Coverage and limits

| Evaluation split | Starts | Old next choice | New next choice |
| --- | ---: | ---: | ---: |
| Other forms of the training tasks | 192 | 152/192 | 192/192 |
| Held-out constants | 240 | 190/240 | 240/240 |
| Two controls | 6 | 4/6 | 6/6 |

All 16 training sources are excluded. Other forms still share underlying tasks,
and held-out constants share templates. Deliberately choosing rejected starts
also differs from ordinary first-choice operation. Full search sessions, native
code, unfamiliar tasks and broad Korean/English language understanding remain
outside this diagnostic.

A useful next panel needs tasks where the initial source description permits
multiple plausible behaviors and actual observations distinguish them. That
would leave room to measure whether feedback helps. Gooo should keep the finite
choice set and case checks explicit while model scores remain advisory.

## Local execution cost

- 438 observed candidates: **9.004 ms** total.
- 876 model forwards and 876 immediate selected-body evaluations; **zero fits**.
- Diagnostic median / p95: old **14.542 / 20.792 µs**, new **15.084 / 19.875 µs**.
- Whole producer: **0.58 s** wall, **0.20 s** user CPU, **0.04 s** system CPU.
- Peak RSS: **20,201,472 bytes (19.27 MiB)**; about **0.41 CPU cores** on average
  from rounded process times. Host utilization change was not collected.
- Both models retain 9,290 FP32 parameters and 37,160 weight bytes each.
- Native processes, GPU work and cloud jobs: **zero**.

Each diagnostic timing includes copying the hidden trace. The two models run in
fixed old-then-new order, so small timing differences carry an order confound.

## Original evidence

The [protocol](protocol.txt) and producer were committed before any new model
call: `1e0c6776af92f03cb0c6e63124b664f2ef7a8a4f`, clean Go 1.27.2.
No model was retrained or edited and no previous study was rerun.

The [report](result/report.json), [read-only audit](result/audit.json) and
[manifest](result/SHA256SUMS) bind every source, observed mask, exact int64
failure, feature array, selected body and case outcome. The auditor compares
them with the immutable complete candidate table, reconstructs observed feature
channels and recounts the saved-score comparison. It makes zero predictions,
candidate evaluations or fits. Tests reject altered source/feedback cells,
case identity and one-unit errors in a failure value beyond 2^53.
