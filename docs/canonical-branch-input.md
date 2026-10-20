# Keep equivalent variable forms on the same model input

A developer may write `return x - limit`, or assign `x - limit` to a local
variable and return that variable. For the supported pure branch bodies, the
small decision model should see the same calculation in both forms. Otherwise
the developer has to learn which spelling happens to work with a model.

This optional input format resolves the predicate and each branch's returned
expression from the source. It presents them as a direct-return branch for
ranking. Candidate construction and execution still use the original plan.

The SDK API and example command implement this path. A new trained artifact is
required. The released interaction weights retain their original input format;
changing their JSON schema would mislabel their training. A
[canonical learning study](../studies/canonical-contract-learning-20261011/README.md)
now publishes both fresh fits, their weak first-choice results and bounded source
feedback. Its weights are research artifacts. The native compiler adapter is
included in the public compiler0.6.28 development prerelease with SDK0.2.40.

## Download an existing model

The [public Hub repository](https://huggingface.co/asketeddy/gooo-canonical-decision-tiny-v1)
provides the subsequent pooling study's original mean/extreme fits, MIT license,
frozen protocol, synthetic corpus, full construction records and loss histories.
Download a fixed revision to a new directory with the Hugging Face CLI:

```sh
hf download asketeddy/gooo-canonical-decision-tiny-v1 \
  --revision c3d87b2e378125a2fd78aec384ca7a04590b33bc \
  --local-dir gooo-canonical-model
```

The payload is about 1.4MiB. On macOS, compare its files with `SHA256SUMS`:

```sh
(cd gooo-canonical-model && shasum -a 256 -c SHA256SUMS)
```

Linux users can use `sha256sum -c SHA256SUMS` in the same directory.
Use either `model-mean.json` or `model-extreme.json` under
`gooo-canonical-model/result/` as the model path in SDK `search -model` below.
No new fitting is needed to use the files.
The loader is `contractdecision.DecodeCanonicalInteractionRequirementConditioned`.
The Hub files preserve their original bytes; changing schema fields cannot convert
an older trained model to this input format.

Both models have 19,034 FP32 parameters. On 128 related evaluation documents,
mean accepts its first choice in 32/128 and extreme in 36/128, with 14 improvements
and 10 regressions for extreme. Both accept 16/64 excluded-family first choices. Final finite construction
completes all 256 documents for both models and fixed ordering. The fixed path has
the lowest total construction time in this study. These are synthetic template
variants; results and byte sizes have separate scopes from general language
understanding, process RAM and customer savings. See the
[complete study](../studies/canonical-pooling-20261011/README.md).

## Native compiler support

Use the public [compiler0.6.28 development prerelease](https://github.com/kimjooyoon/meta-ontology-go/releases/tag/v0.6.28-dev),
built from [`ee24a413`](https://github.com/kimjooyoon/meta-ontology-go/tree/ee24a413d311b0be117bb8a7e260bd1ca35994bd)
with SDK0.2.40. Earlier compiler0.6.27 does not accept this schema. Follow the
[matching compiler guide](https://github.com/kimjooyoon/meta-ontology-go/blob/ee24a413d311b0be117bb8a7e260bd1ca35994bd/docs/canonical-contract-model.md)
with the release's source examples and Go1.27.2 for native construction, substituting the downloaded model's
path. `body-context` displays canonical inputs without predicting or running
candidates; `body-construct --model` ranks and checks the source choices.
Omitting the model uses deterministic ordering. Unsupported source shapes include
a reason and retain that ordering without inference; malformed artifacts return
an error. Saved construction replay does not need the model file.

The retained native observation used the earlier SDK0.2.40 release's canonical
weights. The Hub pooling weights have original SDK construction observations;
a separate native end-to-end observation of those two files remains unmeasured.
The actual 0.6.28 macOS ARM64 release file loaded the Hub extreme model through
`body-context` with zero predictions and zero candidate executions. This verifies
loading/input compatibility only. Its saved-report reader also matched six
retained text/Markdown/JSON outputs without new construction or inference.
Check the compiler's [current status](https://github.com/kimjooyoon/meta-ontology-go/wiki/Current-Status)
for later release availability.

## Inspect, fit and use

From this repository checkout with Go 1.27.3, inspect a Gooo-derived typed plan:

```sh
GOWORK=off GOTOOLCHAIN=go1.27.3 go run ./examples/contract-model inspect \
  -canonical-source path-to-your-plan.json
```

`source_choices[].canonical_context` shows the resolved source, availability
and reason. `canonical_features` contains the exact 528 cells consumed by the
model when that context is available. The original source features remain in
their existing field. Inspection performs zero predictions, training updates
and candidate executions. `-ordered-source` can be added for a side-by-side
view of the earlier ordered-expression adjunct.

To create a new local model from explicitly selected training documents:

```sh
GOWORK=off GOTOOLCHAIN=go1.27.3 go run ./examples/contract-model fit \
  -canonical-interactions -epochs 2000 -out my-canonical-model.json \
  path-to-training-plan-a.json path-to-training-plan-b.json

GOWORK=off GOTOOLCHAIN=go1.27.3 go run ./examples/contract-model search \
  -model my-canonical-model.json path-to-evaluation-plan.json

GOWORK=off GOTOOLCHAIN=go1.27.3 go run ./examples/contract-model search \
  path-to-evaluation-plan.json
```

Use your own plan paths. Every file passed to `fit` is training data. Keep
evaluation documents outside that list and choose the corpus and settings before
observing their results. `fit` checks every permitted candidate against the
authored requirements to form labels. Its current CLI limit is six binary
choices and a new output file; it refuses unsatisfiable training documents.
Canonical context currently admits at most the predicate-order and branch-layout
choices of one branch. The general six-choice training cap does not widen that
source support.

The command writes the artifact and reports feature versions, fit options,
history and an input-collision audit. `search` selects the matching input path
from the artifact schema. It predicts once before candidate execution and
queues that proposal immediately. Later search steps make no further model
calls. Omitting `-model` uses deterministic ordering.

The SDK session can return the best partial body together with `PARTIAL` progress.
The command retains that progress and returns an error if the finite search does
not complete. Applications must inspect completion and its checked cases before
using a body. A non-nil body alone does not mean every requirement passed.

## Input and computation

| Cells | Meaning |
| --- | --- |
| `[0:64]` | Source facts for the resolved direct-return branch and each option |
| `[64:192]` | Original intent encoding |
| `[192:384]` | Reserved zeros |
| `[384:528]` | Exact ordered predicate, then-return and else-return expressions |

Output examples and intermediate Boolean conditions remain separate streams.
Integer literals keep their exact byte encoding, including values beyond
floating-point integer precision. Resolution substitutes reaching writes and
normalizes fallback orientation; it does not fold arithmetic, reorder operands
or reassociate expressions. Overflow behavior remains that of the original
program. Repeated atoms occupy separate nodes in the virtual view, so prior
expression-node sharing cannot change its source counts.

The source version is `canonical_branch_intent_ordered_expressions_v1`; the
artifact is `gooo/canonical-interaction-requirement-contract/v1`. A separate
decoder and fingerprint bind that format. The calculation shares the existing
[interaction equations](interaction-requirement-contract.md), with 19,034 FP32
weights / 76,136 tensor bytes and the same bounded workspace. The wrapper owns
one weight array. Process RAM includes additional runtime and source storage.
The study records first-choice quality, prediction latency and whole-process RSS
separately. Tensor bytes alone do not describe process memory, and lower candidate
counts did not imply faster total construction in these small calculations.

Relevant APIs:

- `PreparedPlan.CanonicalOrderedBranchContext`
- `ContractInput.CanonicalOrderedSourceFeaturesInto`
- `FitCanonicalInteractionRequirementConditioned`
- `DecodeCanonicalInteractionRequirementConditioned`
- `PreparedPlan.NewCanonicalInteractionRequirementContractSession`
- `AuditCanonicalRequirements`

The separate experimental [source-relative condition input](source-relative-conditions.md)
exposes exact comparison-relation/Boolean-goal counts through
`ContractInput.SourceRelativeConditions` and `inspect -condition-relations`.
It has its own feature version and no fitted consumer yet; the models described
above continue using their original inputs.

Source resolution uses one root branch, known input/integer/Boolean atoms, and
at most one operation in each predicate or return expression. Nested arithmetic,
nested branches, operation holes and other choice kinds return an explicit
unavailable reason. Search with such a source declines the model with zero
predictions and retains finite deterministic construction. A nil model retains
the original deterministic receipt. An unsupported training source is rejected
before candidate labelling or weight updates.

## Evidence so far

The [retained-input inspection](../studies/canonical-input-inspection-20261011/README.md)
projects 256 existing Gooo documents without rerunning their experiments. Across
96 paired direct-return/assignment bodies, complete source arrays match in
96/96 pairs, versus 32/96 with the earlier format. All stored old input arrays
remain equal to their historical values. The new complete-input audit finds
zero conflicting label groups in this corpus.

This establishes consistency for those inputs. It does not measure learned
accuracy: the inspection performs zero model calls or candidate evaluations.
Regression tests cover both pooling modes, all four fallback orientations,
extreme integers, independent input prediction, cancellation, contradictory
requirements, concurrent snapshots, decoding and example-command routing.
Tiny two-epoch test fits exercise the API and carry no generalization claim.

The subsequent study fixed a separate corpus and protocol before execution. Of
128 new variants in the trained computation families, the first proposal passed
all requirements in 36 with canonical input and 33 with legacy input. Canonical
inputs, logits and proposals matched in all 128 direct/assignment pairs across
the evaluation cohorts, while training performance itself remained weak at 18/64.
These observations support consistent representation; they do not establish a
quality advantage suitable for a new default model.

The sparse cohort also shows why a passing source contract needs context: all
64 initial bodies passed their supplied examples, but only 16 passed separate
probes. Adding up to two failing probes to the source brought both model and
fixed ordering to 64/64 on that same probe suite. That is repair of known cases;
there was no fresh evaluation after feedback. Full records and both fits remain
in the [study](../studies/canonical-contract-learning-20261011/README.md).

For practical adoption, measure how much work a developer needs to write a
rule, change it, understand the changed cases and reuse the saved program. Keep
that effort, repeat use and support burden separate from first-choice accuracy.
