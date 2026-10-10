# Learning to choose with value flow — 2026-10-10

The previous static projection distinguished source paths that used to look
identical to the model. This study trains two small models to check whether that
extra information helps actual Gooo body selection. The
[protocol](protocol.txt) was committed before either fit. Both models and every
new successful or failed judgment are in [result](result).

## What ran

Both models have 384 inputs, 24 hidden units and two choice scores: 9,290 FP32
parameters, occupying 37,160 weight bytes. The **flow-off** model receives zeros
in the 64 static-value cells during fitting; their unused incoming weights are
then zeroed for runtime. The **flow-on** model receives the source-derived
assignment/return facts. The other 320 cells are identical. The off model's
original pre-pruning artifact is retained, and the audit verifies that no other
weight changed.

Each model was fitted once on the same 32 Gooo-derived training sources and 160
contexts. Both use seed 17, 400 epochs, learning rate 0.25 and L2 0.0001. The
128 original sources, 704 original contexts and static projection are frozen
inputs. The older v3 fits and experiments were not repeated. There was no
parameter search or adjustment after these results.

For each of 704 contexts per model, prediction was followed immediately by
construction and evaluation of the selected body: **1,408 new judgments**.
Four finite-search modes then ran on all 128 sources: **512 searches**. They
used every declared candidate as their budget (four or eight), one attempt per
batch, no sampling seed and up to 16 feedback rounds. The searches retain
per-attempt outputs, conditions, model receipts and the final selected body.

## First selection

A selection is valid when all authored output cases and declared intermediate
conditions pass. These are related, authored variants of a few small functions.
The percentages measure this collection's cases, not general language ability.

| Source group | Flow off | Flow on |
| --- | ---: | ---: |
| Training sources | 26/32 | 32/32 |
| Changed Korean/English wording | 18/32 | 25/32 |
| Changed constants | 26/32 | 32/32 |
| Assignment form | 8/16 | 8/16 |
| New four-region family | 2/16 | 8/16 |
| All sources outside training | **54/96 (56.25%)** | **73/96 (76.04%)** |

On saved contexts with an earlier candidate observation, the four-region family
improved from 16/128 to 64/128 valid selections. Its output cases passed
304/1,408 versus 960/1,408. Assignment-form judgments regressed from 32/64 to
30/64. Distinguishing the static inputs did not make the model learn every
distinction. In particular, the training set contains no assignment-form sources.

## Finite completion and feedback

All four modes completed all **128/128** sets of authored cases. This establishes
completion within their declared four/eight-path search spaces and budgets.
It does not establish correctness on every possible input or arbitrary Gooo code.

| Mode | Candidate attempts | Model calls | Extra feedback calls | Summed search time |
| --- | ---: | ---: | ---: | ---: |
| Flow off, initial ranking | 224 | 128 | 0 | 21.180 ms |
| Flow off, feedback | 226 | 218 | 90 | 31.344 ms |
| Flow on, initial ranking | 151 | 128 | 0 | 17.252 ms |
| Flow on, feedback | 151 | 151 | 23 | 19.022 ms |

Comparing initial ranking source by source, flow on needed fewer attempts on 31
sources, the same on 95, and more on two. The two regressions were
`bucket-k13-r0` and `bucket-k7-r0`: off selected mask 0 immediately, while on
tried mask 4 and then mask 0. Aggregate improvement therefore includes explicit
individual regressions.

Feedback gave no attempt reduction for the flow-on model: all 128 sources used
the same number of attempts, with 23 additional model calls. For flow off,
feedback increased attempts on two sources and left 126 unchanged. The original
v3 study's feedback benefit does not transfer automatically to these new weights.
The current evidence favors initial ranking for this collection. Feedback stays
an explicit option, and its usefulness needs a separately measured decision rule.

## Local resource observations

| Measurement | Flow off | Flow on |
| --- | ---: | ---: |
| CPU fitting duration | 1.156 s | 1.152 s |
| Prediction median | 13.958 µs | 13.917 µs |
| Prediction p95 | 20.833 µs | 20.792 µs |
| Serialized model artifact | 93,635 bytes | 108,446 bytes |

Both fits, input preparation, all judgments, searches and result writing took
3.11 seconds wall time, 2.65 seconds user CPU and 0.06 seconds system CPU. Peak
RSS was 45,531,136 bytes (about 43.42 MiB). The CPU times correspond to roughly
0.87 cores averaged over the process. They do not measure a whole-computer CPU
utilization increase. Peak RSS includes records and training storage; it is not
the isolated model's inference memory.

This was one macOS arm64 process built with Go 1.27.2. Model order was fixed and
timings are observational. Older v3 timings remain historical measurements,
with different dimensions and producer versions, rather than a controlled
performance baseline. No GPU, native execution or quantization was used here.

## Artifacts and audit

Clean producer: `2db75199ef23c76ab1a368290a403523e2dfc3a4`.

- [Flow-on model](result/model-flow_on.json), SHA-256
  `8380505096525a640a06418526530cbc7f76d3efebc2923ee04b018539c5c13f`.
- [Flow-off model](result/model-flow_off.json), SHA-256
  `29fa39ef7954f2d6bc8a6d0c2b275b5f36e00eeeffb65435701248af7854e6c9`.
- [Full report](result/report.json), [audit](result/audit.json), and
  [checksums](result/SHA256SUMS).

Compressed observations decompress to the original bytes. The report binds
the original dataset, context and projection files by digest. The original Gooo
source text and typed documents remain in the prior public study; the audit
checks their identities and compares each selected body's exact recorded cases.
All integer fields are decoded as `int64`, including values above float64's
exact-integer range.

From the repository root:

```sh
(cd studies/value-flow-learning-20261010/result && shasum -a 256 -c SHA256SUMS)
GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./studies/value-flow-learning-20261010/audit \
  studies/value-flow-learning-20261010/result \
  studies/execution-feedback-learning-20261010/result \
  studies/flow-feature-projection-20261010/result
```

The audit reads saved arrays and observations. It checks training splits,
ablation, model identity, candidate cases, initial runtime input hashes, receipt
digests, unique attempts and all reported totals. It does not fit, predict,
prepare, compile or evaluate. It does not independently reproduce training.

## Next language question

Gooo can already represent a direct return and a return through a local variable
with the same value-flow facts. The learned model still reacts differently to
other source-shape features. A useful next experiment is to make those known
semantic equivalences explicit in the model's input or training contract, then
check new copy/overwrite forms outside the training families. This needs new
records and a new protocol; the two models above remain fixed.
