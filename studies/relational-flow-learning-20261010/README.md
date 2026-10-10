# Source relations: a measured regression in first-choice quality

This experiment asks whether a small local Gooo decision model benefits from
explicit equality/order relations among the values in a branch. One new v6 fit
was compared with the already saved v5 observations. The producer committed
its [protocol](protocol.txt) before training. No previous experiment was rerun.

## What happened

| First choice satisfies every declared case | Historical v5 | New v6 |
| --- | ---: | ---: |
| Training:16 direct-return forms | 10/16 (62.5%) | 8/16 (50%) |
| Same tasks in64 other source forms | 40/64 (62.5%) | 32/64 (50%) |
|80 forms with held-out constants | 50/80 (62.5%) | 40/80 (50%) |
|2 unsupported-shape controls | 1/2 | 2/2 |
| Total | 101/162 (62.35%) | 82/162 (50.62%) |

The first v6 candidates passed736/1296 output cases (56.79%) and486/486
condition cases. A condition pass therefore does not mean the returned value
satisfies the source intent. These denominators count repeated cases across
equivalent forms; they are not1296 independent tasks.

All128 paired equivalent forms still had exactly equal prediction distributions.
The model strongly favored the smaller-value task:80/80 min tasks passed and
0/80 max tasks passed on the first choice. Each of the80 opposite-intent pairs
had different input arrays and disjoint acceptable sets. In40 pairs the saved
prediction distributions were nevertheless exactly equal. This localizes a
remaining problem to selection after encoding; it does not identify a unique
cause in the learned weights or optimizer.

Both new search modes completed162/162 authored case suites, using242 candidate
attempts each. Historical v5 used223 attempts in either mode; deterministic
search used324. v6 initial-only made162 model calls. Feedback made242 calls,
including80 extra calls, with no reduction in attempts in this run. Previously
executed candidates were not executed again. Final case completion came from
bounded assembly, checking and continued search as well as the initial choice.

## Local cost

| Measurement | Observed v6 |
| --- | ---: |
| Fresh CPU fit,16 initial samples,400 epochs | 101.66ms |
| Prediction over4 complete candidates, median / p95 | 13.916 / 15.708µs |
| Model parameters / FP32 weight bytes | 9,290 / 37,160 |
| JSON artifact | 108,471 bytes |
| Whole producer: wall / user CPU / system CPU | 0.72 / 0.28 / 0.03s |
| Peak process RSS | 21,594,112 bytes (20.59MiB) |

Whole-producer CPU time corresponds to about0.43 CPU cores averaged over its
short wall interval, using rounded process timings. Host CPU utilization change
was not measured. The process includes loading data, fitting,162 immediate
candidate checks and324 searches; RSS is not inference-only memory. There were
566 total prediction calls and no native subprocess executions. v5 and v6 were
measured at different times, so these are not a controlled speed comparison.

## What changed and what to do next

The384-cell input now exposes six source-only equality/less/greater flags among
then-return, else-return and predicate atoms. Whole integers stay exact; intent,
committed observations and the source atom suffix are preserved. The network
remains384→24→2. Training used only the16 direct initial contexts, the same
400 epochs, rate0.25, L2=0.0001 and seed17 as the prior study. Feedback contexts
were not trained. There was no parameter search or selection of a better seed.

The relation ABI is available explicitly for further research. These weights
are an observed regression and do not replace the existing model by default.
Before increasing training volume, the next useful investigation is how the
learned branch choice responds to opposite Korean/English intent on identical
source structure. Inspecting separate choice scores and hidden activations in
a separately declared diagnostic can distinguish inactive paths from a weak
intent interaction. Any new architecture should face the same opposite-intent
pairs and retain the deterministic completion path.

The existing additive choice scorer cannot express every joint candidate
distribution, but this run alone does not establish that limitation as the cause
of this max/min failure. Separate first-choice quality, case completeness and
search cost instead of treating them as one accuracy percentage.

## Evidence and limits

- Producer: `3dfcb1701bba1020d4002025e2c3dd163b534359`, clean Go1.27.2/darwin-arm64.
- [Original report](result/report.json), [historical report](result/baseline-report.json),
  [v6 artifact](result/model-v6.json), [read-only audit](result/audit.json).
- All162 predictions,324 searches,16 training arrays and400 losses are retained
  in the compressed result files. [SHA256SUMS](result/SHA256SUMS) binds publication.
- Source fixture records are reused from the
  [normalization study](../semantic-flow-normalization-20261010/README.md).
- The audit recounts original outputs using int64, verifies runtime input hashes,
  model fingerprints, nonrepeating attempts and aggregates without fitting,
  predicting or executing new candidates.

These fixtures cover a small finite branch-selection problem. Representation
transfer shares tasks with training; constant transfer shares templates. Neither
is evidence of general Korean/English understanding. The model chooses permitted
paths; Gooo and the runtime assemble their bodies and execute the declared cases.
