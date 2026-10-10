# Canonical inputs and examples that leave several programs possible

Status: source corpus and experiment code prepared before original execution.
No results or new trained weights have been collected for this study yet.

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

## What will be reported

- First-candidate declared-case and separate-probe completeness.
- The final selected body's probe results before any extra cases are supplied.
- Completeness, attempts and added cases after each bounded feedback stage.
- Equivalent-form agreement, improvements, regressions and input ambiguity.
- Local fit cost, prediction latency, overall process cost and saved model size.

A model score, a complete supplied case suite, and an independent probe result
describe different things. These measurements will retain their denominators.
The probe suite is finite and written for these templates. Passing it does not
establish correctness for every input or real-world business suitability.

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
