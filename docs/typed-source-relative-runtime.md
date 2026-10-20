# Small decision heads for Gooo choice kinds

This checkout adds an experimental model with separate weights for comparison
operand order and return-branch placement. It consumes the complete
[V4 source/requirement input](source-relative-v4-runtime.md) and ranks finite
choices. Its output is two option scores per choice. It produces no source tokens.
SDK 0.2.46 and the public Gooo 0.6.34 compiler do not include this new profile.

The [retained V4 diagnosis](../studies/branch-owned-choice-diagnosis-20261012)
motivates this implementation. Comparison selection fell from 216/384 to 164/384
on the same held-out rows, while branch placement reached 192/384. This change
lets the two choice kinds learn separate weights within a small memory budget.
Its quality needs a fresh preregistered study; the earlier cohort is kept frozen.

## Input and layout

Each choice receives the same 528 source values and all declared output and
Boolean rows as V4. Source ownership, exact integer relations and zero intent
cells remain explicit. Canonical source-kind flags choose the head, independently
of the choice's position in the array. The condition-relation kind must agree.
Neither source IDs nor branch labels are learned as authority over the program.

| Layout | Shared V4 head | Two typed heads |
| --- | ---: | ---: |
| Learned case/context width per head | 8 | 4 |
| Hidden width per head | 24 | 12 |
| Total FP32 parameters | 19,034 | 18,364 |
| Raw tensor bytes | 76,136 | 73,456 |

Each typed head has its own case encoders, source context, interaction layer and
two option scores. No parameter is shared between the heads. The full mask score
still adds the selected choice scores; this change does not add pairwise mask
potentials. All four source combinations remain available to finite checking.

Weights and working arrays have fixed bounds. The workspace retains at most 128
output rows, 128 Boolean rows, two choice states and 64 candidate scores. Callers
own their workspace; model weights are immutable during prediction. Tensor size
does not establish process RAM, CPU usage, speed or a quality improvement.

## Use the runtime

The Go package `contractdecision` provides:

- `NewTypedSourceRelative`: construct from two exact 9,182-value arrays.
- `DecodeTypedSourceRelative` and `Marshal`: load/save the distinct
  `gooo/typed-source-relative-branch-model/v1` artifact contract.
- `PredictChoicesInto` and `PredictInto`: fill caller-owned score destinations.
- `FitTypedSourceRelative`: fit bounded, source-labelled samples from a fresh
  initialization; it does not collect or execute candidate programs.

Prepare source rows with `ContractInput.SourceRelativeV4FeaturesInto`, and retain
every declared case through the same input object. The artifact records the V4
input versions, the declared-requirements-only intent policy, typed routing and
both layer widths. Its decoder checks exact array lengths and numerical values.
The typed artifact cannot be substituted for an older V4 artifact.

`PreparedPlan.NewTypedSourceRelativeContractSession(ctx, model, cases)` performs
one initial prediction before any candidate executes. Each `Advance` checks more
source-owned candidates without additional inference. A partial step can return
`ErrNoConditionCandidate` when every candidate tried so far violates the Boolean
requirements; inspect the retained progress and continue while candidates remain.
No body is accepted on that error. A malformed artifact or invalid input remains
an error. Omitting the model uses the ordinary deterministic traversal.

From this checkout, a separately fitted typed artifact can be selected with:

```sh
GOWORK=off GOTOOLCHAIN=go1.27.3 go run ./examples/contract-model search \
  -model /path/to/typed-model.json /path/to/source-owned-path-document.json
```

The existing `inspect -source-relative-v4-source` command shows the complete
input arrays without inference. Typed fitting is currently a Go API; there is no
typed `fit` CLI flag or published fitted typed artifact in this change.

## What has been checked

Synthetic tests check independent scalar forward equations, finite-difference
gradients, kind-based routing after choice reordering, parameter isolation,
concurrent caller workspaces and transactional errors. One synthetic fit step
with zero regularization updates only the visited head. Artifact tests preserve
weight identity and reject mismatched shapes/contracts.

A fresh source fixture deliberately starts with the wrong comparison, retains
its rejected Boolean observations, then completes four output and three Boolean
requirements within its four-candidate palette without another prediction. A
separate CLI fixture checks typed dispatch and deterministic omission. These are
implementation tests with synthetic weights, not model quality measurements.

The next study should compare shared and typed heads on fresh published tasks,
with explicit training settings, model sizes and a deterministic baseline. It
should keep first-choice errors separate from eventual finite completion and
retain every failed case. The change in layer widths is part of the architecture
comparison and must be reported when interpreting results.
