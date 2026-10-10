# Learn the relationship between a case and its goal

Gooo supplies a body with permitted choices, output examples and intermediate
Boolean requirements. This small Go model learns which complete combination to
try first. Gooo then checks the proposed body against the original cases and
continues its finite search when a proposal fails.

The practical question is simple: if two requirements use the same inputs but
ask for opposite conditions, can the model keep that relationship while
summarizing the cases? This implementation adds input–target associations and
small source/output/condition interaction terms to investigate that question.

## Run it locally

From this SDK checkout with Go 1.27.2:

```sh
GOWORK=off go run ./examples/contract-model fit -interactions \
  -epochs 2000 -out interaction-model.json \
  studies/requirement-learning-20261010/examples/req-bound-k6-r1-w0-o0-alias-g0-c0.json \
  studies/requirement-learning-20261010/examples/req-bound-k6-r1-w0-o0-alias-g0-c1.json

GOWORK=off go run ./examples/contract-model search -model interaction-model.json \
  studies/requirement-learning-20261010/examples/req-bound-k6-r1-w0-o0-alias-g0-c0.json

# Omit the model for deterministic search.
GOWORK=off go run ./examples/contract-model search \
  studies/requirement-learning-20261010/examples/req-bound-k6-r1-w0-o0-alias-g0-c0.json
```

These commands create a new local fit; every supplied document is training data.
The fit report includes the input audit, feature versions, model fingerprint and
loss history. Search reports the proposed choices, prediction count, attempts
and declared-case results. A model score is a ranking score; case checks measure
whether the selected body satisfies the authored requirements.

`-interactions` selects this computation. Supply it separately from
`-ordered-requirements`, `-requirements`, `-choice-context`, `-goal-pairs` and
alternative case feature versions. `-pooling signed_max_abs` selects the second
pooling implementation; the default is arithmetic mean.

The SDK API is
`PreparedPlan.NewInteractionRequirementContractSession(ctx, model, cases)`.
It predicts once during initialization, before any candidate executes. The
proposal enters the search queue immediately. Subsequent `Advance` calls use
that ranking and make zero further predictions. A nil model preserves the
deterministic receipt. Unsupported ordered source shapes return a declined
receipt with a reason and zero predictions, retaining deterministic search.

This schema is available through the SDK API and example command. Native
compiler loading and a versioned model release remain follow-up work. A
[fresh study](../studies/interaction-requirement-learning-20261011/README.md)
provides saved weights, held-out results and cost measurements. Compiler 0.6.26
includes the earlier choice-conditioned schema.

## The small computation

There are **19,034 FP32 parameters: 76,136 weight bytes, about 74.4 KiB**.
This counts weights; process memory includes the Go runtime, plans, workspaces
and training scratch. Training and inference use the local CPU through Go.

Each source choice has 528 fields, including ordered predicate and return
operands. Each output or condition case keeps its original 32 fields, including
the exact byte encoding of its integer input. Eleven additional fields multiply
the input's three sign flags and eight bytes by the authored target polarity:

- Output target: negative −1, zero 0, positive +1.
- Condition target: false −1, true +1.

The original target magnitude/bytes remain in the original fields. The added
fields emphasize polarity; they alone cannot distinguish all integer goals.
Ordinals, case counts and choice IDs remain in the original fields but do not
enter these eleven products. Whole `int64` values are never cast to floats.

The learned output encoder reads 43 fields; the condition encoder reads 44,
including whether that condition targets the choice being ranked. Each produces
eight `tanh` activations, pooled over all its cases. A separate learned source
projection produces eight values. For every coordinate, the joint layer reads:

```text
source, output, condition,
source × output, source × condition, output × condition,
source × output × condition
```

Those 56 values accompany the original 528 source fields in a 24-cell hidden
layer with a 0.01 negative slope. Two final scores rank each binary choice.
Complete candidates add their selected option scores. Training minimizes the
negative log probability of the acceptable candidate set, labelled by Gooo's
checks; it has no benchmark-specific rule for choosing a path.

The case and source encoders scale their original fields by eight before their
learned projection. The direct source connection to the joint layer retains its
original scale. An empty condition suite has a zero pool and zero interaction
terms wherever a condition factor appears.

Bounds are 16 choices, 1–128 output cases and 0–128 condition cases. Prediction
snapshots each row once and reuses fixed arrays. Training keeps per-sample
scratch and one reusable gradient array rather than a dataset-sized tensor.
The fitting API accepts at most 64 explicitly supplied candidate masks per
sample. The example CLI labels every combination and therefore caps training
documents at six binary choices; it rejects larger spaces instead of sampling
away possible labels.
Readers must remain immutable during a call; concurrent predictions use separate
workspaces. Intermediate overflow returns an error and preserves caller outputs.

The artifact `gooo/interaction-requirement-contract/v1` records its input
versions, dimensions, association rule, scale, interaction terms, activation,
pooling, numeric rule and weights. Its fingerprint binds the computation and weights. Older
model files keep their original computation.

### A portability failure found by CI

The first revision passed the training regression on macOS arm64 but reached
only 6/8 on Linux amd64, with loss 0.364236. Both original CI runs retain that
failure ([PR](https://github.com/kimjooyoon/gooo-decision-runtime/actions/runs/38062871554),
[push](https://github.com/kimjooyoon/gooo-decision-runtime/actions/runs/38062862480)).
A local amd64 run reproduced it. Disabling implicit multiply-add fusion in the
arm64 decision package also reproduced 6/8 and the same reported loss.

The [Go floating-point specification](https://go.dev/ref/spec#Floating_point_operators)
permits implicit fused operations, which can round differently. This model now
uses an explicit `float64` `math.FMA` and then rounds to `float32` for each affine
accumulation. The artifact binds this as `affine_fma64_round_fp32_v1`. A numerical
regression distinguishes that rule from separately rounding the product.
With the same examples, seed, 2,000 epochs and assertions, both local arm64 and
amd64 tests reach 8/8 and pass all 48 case-order checks. Bit-identical trained
weights across all platforms are outside these assertions.

## Evidence and limits

The [previous frozen study](../studies/ordered-requirement-learning-20261010/README.md)
found only 2/8 training and 11/40 evaluation first-candidate successes after
retaining source order. This new architecture changes case features,
activations, interaction terms and parameter count together. Separate ablations
would be needed to attribute an improvement to one change.

Current regression coverage includes:

- Eight training goals combining source reversal, output polarity and condition
  goal, with exact inputs on both sides of 2^53. All eight finish on the first
  candidate after saving and reloading the model; training loss moves from
  approximately 1.415372 to 0.000445 under the fixed test setup.
- All six simultaneous reorderings of the three output/condition rows for each
  goal: 48/48 first-candidate completions. These are the same eight goals. The
  original fields retain ordinal information, so this is a tested property of
  these fitted examples, not a general invariance guarantee.
- Independent float64 forward equations and numerical gradients, both pooling
  rules, empty conditions, full bounded streams, concurrent inference,
  cancellation, atomic errors and strict artifact decoding.
- CLI fit/save/load/search and deterministic absence/unsupported-source paths.

The subsequent [80-document frozen study](../studies/interaction-requirement-learning-20261011/README.md)
reaches 60/64 first-candidate completions on supported evaluation documents,
versus 20/64 for the ordered comparator; both complete every document through
finite checking. Four English-intent assignment sources still miss. The same
single-template scope, all failures and both saved model files are recorded.
CPU fitting takes 2.024s and prediction median is 98.458µs; total search time is
slightly longer despite fewer attempts. Native generated-code execution and
quantization quality for these weights remain unmeasured.

## Related building blocks

[Deep Sets (Zaheer et al., 2017)](https://arxiv.org/abs/1703.06114) studies learned
per-element transformations and pooled set representations.
[Higher-Order Factorization Machines (Blondel et al., 2016)](https://arxiv.org/abs/1607.07195)
studies compact higher-order feature interactions. They provide useful context
for the pooling and interaction ingredients here. Our current experiment asks
how small learned decisions can work with Gooo's explicit requirements and
finite checker; the regressions and separately frozen study bound its measured
scope.
