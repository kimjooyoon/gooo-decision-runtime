# Canonical inputs and examples that leave several programs possible

Status: both original fits and all original construction sessions completed once.
The source, corpus and protocol were public at
[`46f3c135`](https://github.com/kimjooyoon/gooo-decision-runtime/tree/46f3c1355d43c5ccca4398d3e32ec1b4d1494567/studies/canonical-contract-learning-20261011)
before execution. The retained records, both weight files and read-only audit are
now included below. These weights are research artifacts, not a new default.

The [canonical input](../../docs/canonical-branch-input.md) lets supported
direct-return and assignment bodies expose the same calculation to the model.
The earlier input audit established that consistency without measuring learning.
This study gives a fresh legacy model and a fresh canonical model the same
training documents and compares their first choices and finite construction.

There is a second practical question. A program can satisfy every supplied
example while those examples omit an important distinction. For instance,
several distance formulas return zero when the input equals the threshold.
This study keeps separate probe tests, records that gap, and adds the first
failing probe to the Gooo source before constructing again. Both the original
miss and the repaired result remain visible.

## Fixed scope

| Cohort | Documents | Use |
| --- | ---: | --- |
| Bound/distance, direct returns, Korean/English | 64 | Both fits |
| New constants/wording, direct returns/assignments | 128 | Evaluation only |
| Conditional addition/multiplication | 64 | Excluded-family evaluation |
| Distance with one output and one condition | 64 | Sparse-contract evaluation and bounded feedback |

These are related template variants. Each has two binary choices, so construction
checks at most four candidates per stage. The sparse cohort allows at most two
additional cases. Every probe is fixed before execution. Training uses authored
cases from the 64 training documents, and does not consume probe outcomes or
evaluation documents.

Both models have 19,034 FP32 weights (76,136 tensor bytes). The fixed recipe uses
mean pooling, seed 31, 2,000 epochs, learning rate 0.3 and L2 0.0001. There is one
fit per format and no retry or tuning after the results. See the complete
[protocol](protocol.txt), [source generator](record/source.go) and retained
`corpus.jsonl.gz`.

## First-choice results

Each cell counts bodies whose first proposed candidate passes all declared and
separately fixed probe requirements, except the sparse row described below.

| Cohort | Fixed order | Fresh legacy input | Fresh canonical input |
| --- | ---: | ---: | ---: |
| Training documents | 16/64 | 17/64 | 18/64 |
| New constants/wording in trained families | 32/128 | 33/128 | 36/128 |
| Excluded computation family | 16/64 | 15/64 | 16/64 |
| Sparse: first candidate passes probes | 16/64 | 16/64 | 16/64 |

The canonical input has 18 improvements and 15 regressions against the legacy
input in the 128-document evaluation, and nine improvements and eight regressions
in the excluded family. Every regression ID is in [audit.json](result/audit.json).
The net differences are small and both fits perform poorly even on their training
documents. Their final losses, 1.386064 and 1.385903, are close to the uniform
four-way loss of ln(4). One fixed recipe cannot establish the cause of this weak
learning or a reliable quality advantage.

Equivalent source forms are more consistent. Among 128 direct/assignment pairs
across the three evaluation cohorts, the canonical input, logits and initial
proposal agree in 128/128 pairs. Legacy inputs/logits agree in 32/128 and proposals
in 107/128. This is representation consistency, with the quality limits above.
The earlier broader study used different documents and goal combinations; its
117/128 result is not a paired comparator for this experiment.

## When examples leave the intended program unspecified

All 64 sparse documents initially provide only `K -> 0` and the condition at K.
All four possible bodies pass those requirements. Each policy therefore reports
64/64 declared completions while only 16/64 selected bodies pass the separate
probes. This is a concrete gap between supplied-case completeness and the intended
behavior defined by the probes.

The loop takes the first failing output probe, or then the first failing
condition, writes that case into a new Gooo source, lowers it and constructs again.
It adds at most two cases per document per policy, using the same saved weights.

| Sparse cohort, 64 documents per policy | Fixed | Legacy | Canonical |
| --- | ---: | ---: | ---: |
| Initial declared completion | 64/64 | 64/64 | 64/64 |
| Initial selected body passes probes | 16/64 | 16/64 | 16/64 |
| Final body passes those probes after feedback | 64/64 | 64/64 | 64/64 |
| Additional source cases / construction stages | 80 | 79 | 80 |
| Candidate attempts, including feedback | 272 | 240 | 284 |
| Model calls, including feedback | 0 | 143 | 144 |

The fixed policy obtains the same final completeness. The improvement comes from
adding discriminating requirements and checking a finite candidate space; it is
not evidence of online learning. Once probes supply feedback, their final pass
rate measures repair of that suite. No new held-out suite was run after repair.
The loop can obtain expectations here because explicit template functions supply
them. Production use still needs a trusted source of expected behavior.

The source-input audit finds groups with identical visible requirements but four
different hidden probe goals. At most 16/64 can receive the correct initial choice
from those inputs alone. This bound applies to the constructed sparse cohort;
it is not a general ceiling for Gooo or decision models.

All 256 non-sparse documents eventually pass their declared and probe suites
within four candidates, under every policy, without adding requirements.

## Local cost

| Measurement | Legacy | Canonical |
| --- | ---: | ---: |
| One CPU fit | 15.319s | 15.553s |
| First-session prediction median, 320 observations | 95.958µs | 96.166µs |
| First-session prediction p95, nearest rank | 109.208µs | 108.541µs |
| Saved JSON model size | 223,108B | 223,178B |
| FP32 tensor storage | 76,136B | 76,136B |

There are 960 initial searches, 239 additional feedback searches and 799 total
model calls. Each session predicts at most once before its first candidate runs;
later `Advance` calls perform zero predictions. Feedback starts a new session.
The model is not invoked after construction to explain an already chosen body.

Across all documents and feedback, fixed/legacy/canonical construction took
912/876/919 attempts and 37.484/104.383/92.619ms in total. These intervals include
session initialization and checking, and exclude source lowering, fit time,
initial record collection and separate probe execution. The small model did not
reduce total construction time versus fixed order for these cheap bodies.

The original whole process took **32.29s wall**, **31.98s user+system CPU**, and
**96.219MiB maximum RSS**, on an Apple M4 with 10 logical CPUs and 16GiB RAM.
CPU time / wall time is about 99.0% of one core on average. This is not a measured
increase in instantaneous computer-wide CPU utilization. RSS includes the entire
study, both fits, models and retained data; it is not model-only inference RAM.
Policy order and host load were uncontrolled. These are one-run observations,
not an isolated speed comparison or a customer cost saving.

## Practical consequence

Keep the model optional. A useful next interface should show supplied-case
completeness, separate observations and remaining uncertainty together, then make
a recorded failed expectation easy to add as a source case. Preserve the original
failure and the new source identity across that transition. Compare this workflow
with editing an ordinary Go function and tests, including the work of writing
expectations and diagnosing unsupported expressions.

This experiment supplies a bounded research implementation of that loop. It does
not install canonical weights in the native compiler, establish arbitrary-input
correctness, or measure external adoption, repeat use, payments or operating savings.

## Read the evidence without repeating execution

`result/` retains all source documents, both source arrays, four candidate bodies
per document, exact observations, training rows, full loss histories, model files,
all construction stages and augmented source. Larger text files use deterministic
gzip compression. `SHA256SUMS` covers the compressed bytes; the report also binds
the uncompressed source/search records.

From the SDK repository root:

```sh
(cd studies/canonical-contract-learning-20261011 && shasum -a 256 -c SHA256SUMS)
GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./studies/canonical-contract-learning-20261011/audit \
  studies/canonical-contract-learning-20261011/result
```

The audit reads retained evidence and checks exact int64 arithmetic, source and
case identities, feedback selection, model fingerprints and every reported count.
It performs no model fitting, prediction, Gooo lowering or candidate execution.
It is also included in normal SDK tests. The original producer is not rerun.

## Source checks

The nested `producer` module imports the compiler's source lowering from a clean
pinned sibling checkout. Its source preflight reads all frozen documents and
both input formats without evaluating candidates or fitting. A separate small
K=5 regression, absent from this corpus, uses zero untrained weights to exercise
the counterexample loop. It does not contribute study results.

The producer is a one-use research entry point. It requires a clean committed
build, the frozen corpus and a fresh output directory. Historical producers and
experiments remain unchanged. Public result audits will read retained records;
they will not repeat model fitting or original inference.
