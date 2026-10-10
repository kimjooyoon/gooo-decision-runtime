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
changing their JSON schema would mislabel their training. No canonical model
quality study or native compiler adapter has been published yet.

## Inspect, fit and use

From this repository checkout with Go 1.27.2, inspect a Gooo-derived typed plan:

```sh
GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./examples/contract-model inspect \
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
GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./examples/contract-model fit \
  -canonical-interactions -epochs 2000 -out my-canonical-model.json \
  path-to-training-plan-a.json path-to-training-plan-b.json

GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./examples/contract-model search \
  -model my-canonical-model.json path-to-evaluation-plan.json

GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./examples/contract-model search \
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
No latency, RSS or accuracy measurement for this new format is claimed here.

Relevant APIs:

- `PreparedPlan.CanonicalOrderedBranchContext`
- `ContractInput.CanonicalOrderedSourceFeaturesInto`
- `FitCanonicalInteractionRequirementConditioned`
- `DecodeCanonicalInteractionRequirementConditioned`
- `PreparedPlan.NewCanonicalInteractionRequirementContractSession`
- `AuditCanonicalRequirements`

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

The next model study needs a separately fixed corpus and evaluation protocol.
For practical adoption, measure how much work a developer needs to write a
rule, change it, understand the changed cases and reuse the saved program. Keep
that effort, repeat use and support burden separate from first-choice accuracy.
