# Paired goals: fewer identical choices, no overall validity gain

2026-10-10. One new CPU fit added an explicit opposite-goal objective to the
existing signed-pooling model. Architecture, 9,746 FP32 parameters, 38,984 weight
bytes, original 24 training documents, 600 epochs and seed stayed fixed.
The [protocol](protocol.txt) and producer were committed before execution at
`440b3eeec867b7e70abaa46dd710ae1e4c7545ae`.

The same-source g0/g1 training documents formed 12 pairs. Their sources and
candidate orders match; the compiler-validated acceptable sets are disjoint.
The objective adds weight 0.5 times `softplus(2 - paired_gap)` to the ordinary
candidate loss. [API, equation and command](../../docs/contract-goal-pairs.md).

## Observed results

Comparison uses the frozen [unpaired signed-pooling results](../contract-pooling-20261010/README.md),
without rerunning that model or the original compiler/oracle collection.

| Measure | Saved unpaired model | New paired objective |
| --- | ---: | ---: |
| First valid path, all 196 documents | 132 | 128 |
| First valid path, original training 24 | 20 | 16 |
| First valid path, satisfiable outside training 170 | 112 | 112 |
| Same proposal for opposite goals, 97 pairs | 62 | 58 |
| Both goals valid on first selection | 35 | 35 |
| Both goals invalid on first selection | 0 | 4 |
| First-body outputs passed, out of 2,048 | 1,582 | 1,548 |
| First-body conditions passed, out of 588 | 588 | 588 |
| Eventual finite completion, satisfiable 194 | 194 | 194 |
| Contradictory goals incorrectly accepted, out of 2 | 0 | 0 |
| Total candidate attempts, including contradictions | 264 | 268 |

There were **10 improvements, 14 regressions and 172 unchanged first decisions**.
English wording improved 12/24 to 14/24; representation variants improved 32/48
to 34/48. New constants fell 52/72 to 50/72, and the unseen family fell 14/24 to
12/24. Both 128-case rare-tail goals still passed on the first selection.
All regression identifiers appear in [the read-only summary](result/audit.json).

The decrease in identical proposals did not increase the number of pairs where
both answers were valid. This result makes same-proposal count inadequate as a
standalone improvement target. The paired objective includes direction and the
ordinary candidate loss, yet this single setting still regressed; the records
do not establish a unique optimization cause. This remains an optional
experiment. The published default model is unchanged.

These are known-corpus ablation results. Evaluation rows stayed outside fitting,
but the objective was motivated by diagnostics of this collection. They do not
estimate success on fresh Korean/English requests.

## Local cost and retained records

- One fit: **679,480,041 ns**; 28,800 training forwards, twice the unpaired count.
- 196 new sessions and initial inference calls; no later feedback calls.
- Prediction median **16,416 ns**, p95 **21,375 ns**.
- Entire producer: **1.26 s wall**, **0.81 s user + 0.02 s system**, maximum RSS
  **31,473,664 bytes / 30.02 MiB**. Rounded process times imply about 0.66 CPU
  core on average; host CPU utilization increase was not measured.
- No native subprocesses, external models, GPU jobs or additional label/oracle
  collection. Old/new process timings do not isolate an inference speed change.

The model JSON is 113,970 bytes, SHA-256
`9b8883bbe3d2c55d061c300479d161463b8c79f80c86b7ad1a5664a989740061`.
Its v2 fingerprint is
`e6dc12548759daa3ef77244ae155d88ce25f83d8372a821debafb520c4c260e2`.
The result directory includes every loss, all consumed training rows, pair
indices, every search step, incorrect output, model weights, build identity and
process measurements. All large integer values remain typed int64.

## Read-only audit

```sh
go run ./studies/contract-goal-pairs-20261010/audit \
  studies/contract-goal-pairs-20261010/result \
  studies/contract-goals-20261010/result \
  studies/contract-pooling-20261010/result
```

The auditor checks frozen source/training/baseline digests, original pair
membership, exact candidate outcomes with independent int64 arithmetic,
ranking and progress chains, eventual completion, all split counts and latency
quantiles. It performs no fit, model inference, source lowering or body execution.
Mutation tests reject pair/index/label changes, goal changes and one-unit changes
to positive and negative output values beyond 2^53, including coordinated
actual/expected edits. The original producer ran once; auditing never replaces
its outcomes.
