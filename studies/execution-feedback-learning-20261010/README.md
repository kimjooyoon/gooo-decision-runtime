# Learning from Gooo output counterexamples

This fixed local study asks whether a small decision model can use a wrong
output to select a better next body. It extends the earlier
[branch-role experiment](../branch-role-learning-20261010/README.md), where
condition-only feedback missed output failures.

The source/protocol producer is
`0c82f25efb08f5880801098cd61283dd28e7bd85`. It was committed before the original
export and three measured CPU fits. Gooo lowering uses compiler
`41f6e4668906c8ccc6798f3d43be34602ec3aa28` with public SDK
`v0.2.32-experimental`. That compiler PR's CI was still running during the study.
The sources are compiler-lowered documents and run through the Go typed
interpreter; this experiment executes no native binaries.

## Fixed comparison

128 source variants cover max/min selection, bounded clamping, negative/zero/
positive codes, branches that assign a result variable, and four-region
classification. There are 576 complete candidate masks and 704 initial/observed
contexts. Only 32 source variants and their 160 contexts enter training.
Wording, constants, assignment structure and four-region tasks are held out from
fitting. These are related authored variants, not independent language tasks.

All three fits use 400 epochs, seed17, learning rate0.25 and L2=0.0001:

- **v2:** 256→24→2, source/intent/condition/branch roles, 6,218 FP32 parameters.
- **v3 output off:** 320→24→2, 7,754 parameters; train with a zero output tail,
  then zero the first-layer weights connected to that unused tail. Both artifacts
  are saved. This predetermined transformation preserves training predictions.
- **v3 output on:** the same 320→24→2 architecture, reading exact observed
  input/expected/actual output bytes and the selected candidate mask.

The output-off model receives ordinary runtime observations but has no active
weights connecting them. It can therefore incur extra calls without changing
its scores. The control makes that cost visible. No existing trained model was
loaded, and no settings changed after reading evaluation results.

The [protocol](protocol.txt) specifies the complete source split and measurements.
Each of the 2,112 predictions is followed immediately by compilation and
evaluation of its selected body. All chosen bodies and cases are retained.

## First choice and later observations

| Source split | v2 first choice | v3 output off | v3 output on |
| --- | ---: | ---: | ---: |
| Training | 26/32 | 24/32 | 26/32 |
| New wording | 18/32 | 18/32 | 18/32 |
| New constants | 26/32 | 24/32 | 26/32 |
| Assignment structure | 8/16 | 8/16 | 8/16 |
| Four-region classification | 2/16 | 2/16 | 2/16 |

On the 96 non-training source variants, first-choice validity is 54/96 for v2,
52/96 for output off and 54/96 for output on. After supplied candidate
observations, the corresponding counts are 220/448, 220/448 and 234/448. These
contexts reuse the same sources and cases; the denominators are correlated.

The four-region group changes from 16/128 to 24/128 valid observed choices when
comparing output off with output on. Its output-case count changes from
304/1408 to 424/1408. The assignment group stays at 32/64 valid observed choices.
Initial features still have 72 pairs with identical input and disjoint valid
candidate sets across all three representations. Output observations cannot
repair information missing from the first source representation.

## Finite search

Each mode runs all 128 source variants, with batch1 and its complete candidate
budget of four or eight paths. Source cases are visible during each search.

| Mode | Completed | Attempts | Model calls | Extra feedback calls | Total search time |
| --- | ---: | ---: | ---: | ---: | ---: |
| Deterministic | 128/128 | 320 | 0 | 0 | 15.602ms |
| v2 initial | 128/128 | 220 | 128 | 0 | 15.425ms |
| v2 feedback | 128/128 | 220 | 128 | 0 | 18.642ms |
| v3 output off, initial | 128/128 | 224 | 128 | 0 | 17.752ms |
| v3 output off, feedback | 128/128 | 224 | 216 | 88 | 21.472ms |
| v3 output on, initial | 128/128 | 220 | 128 | 0 | 16.138ms |
| v3 output on, feedback | 128/128 | 212 | 204 | 76 | 20.855ms |

For the same output-on model, adding feedback reduces attempts in four variants
and leaves them unchanged in 124. Each improvement saves two attempts, all in
four-region classification. That subgroup's total falls from 62 to 54 attempts.
For example, `bucket-k7-r4` changes from masks `[7,6,5,4]` to `[7,4]`.

Feedback increased total elapsed search time in this run. The extra inference
and receipt work costs more than the saved tiny body executions. This gives a
concrete selection benefit in a small subset, with an observed runtime cost.
One pass is insufficient for a general speed conclusion. All modes finishing
their declared finite cases does not establish correctness for every input.

## Local CPU and memory

| Model | Fit time | Prediction median | p95 | Weight bytes | Artifact bytes |
| --- | ---: | ---: | ---: | ---: | ---: |
| v2 | 0.624294s | 10.167µs | 15.084µs | 24,872 | 72,490 |
| v3 output off | 0.836126s | 10.625µs | 15.875µs | 31,016 | 75,650 |
| v3 output on | 0.824680s | 10.750µs | 15.917µs | 31,016 | 90,501 |

Prediction timing includes the fixed-shape adapter. The full fit/prediction/
search command took 3.08s real, 2.73s user CPU and0.06s system CPU, with maximum
RSS35,700,736bytes. That averages about0.91CPU cores over this command. No
whole-machine CPU-utilization delta was measured. Separate source export took
0.61s real with maximum RSS30,752,768bytes. No GPU or external base model was used.

## Models, custody and recounting

- [v2 artifact](result/model-v2.json)
- [v3 output-off artifact](result/model-v3_output_off.json)
- [v3 output-on artifact](result/model-v3_output_on.json)
- [Complete report](result/report.json), [saved-record audit](result/audit-summary.json)
- [Checksums](result/SHA256SUMS)

Original append-only records are stored with deterministic gzip. Decompressed
bytes were compared with the original local files. Build excerpts omit only the
first executable-path line; hashes of complete original build records are kept.
Source/model/artifact/training-input identities and exact int64 values are
preserved. The output-off pre-pruning artifact shows the exact fixed transform.

From the repository root, the following recounts saved observations without new
fitting, prediction, compilation or body execution:

```sh
GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./studies/execution-feedback-learning-20261010/audit ./studies/execution-feedback-learning-20261010/result
```

The audit checks source identities, complete masks, exact cases, feature hashes
and output-channel reconstruction, training-only sample hashes, model fingerprints,
weight pruning, histories, selected body results and search receipts. It checks
record consistency; original producer records supply the execution evidence.
Do not rerun the measured study into its original directories.

The next representation gap is assignment dataflow and literal values. The
current branch-role input describes direct returns, while the held assignment
forms return a value assigned inside branches. Training and compiler integration
can now use these public artifacts, but the installed compiler has not yet gained
the v3 loader. This study does not establish quantized quality or calibrated
correctness probabilities.
