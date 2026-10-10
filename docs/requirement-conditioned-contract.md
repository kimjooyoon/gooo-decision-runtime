# Learn from output and intermediate-condition requirements

## Development: retain calculation order during learning

`fit -ordered-requirements` selects a new explicit model that consumes the
ordered-expression adjunct from [source inspection](declared-condition-input.md#inspect-ordered-calculations).
Each choice reads 528 source cells: the original 384 followed by 144 ordered
predicate/then-return/else-return cells. Both the eight-cell query projection
and the 24-cell joint hidden layer learn from the full source input. All authored
output and condition rows retain their existing exact integer encodings.

```sh
go run ./examples/contract-model fit -ordered-requirements -out ordered-model.json \
  studies/requirement-learning-20261010/examples/req-bound-k6-r1-w0-o0-alias-g0-c0.json \
  studies/requirement-learning-20261010/examples/req-bound-k6-r1-w0-o0-alias-g0-c1.json

go run ./examples/contract-model search -model ordered-model.json \
  studies/requirement-learning-20261010/examples/req-bound-k6-r1-w0-o0-alias-g0-c0.json
```

Documents passed to `fit` become training data. The new artifact is
`gooo/ordered-requirement-contract/v1`, with 17,890 FP32 parameters and 71,560 weight
bytes (about 69.9 KiB). Training and inference run in Go on the local CPU. The
artifact binds its 528-cell source version, both goal versions, architecture,
pooling and activation. Existing artifacts retain their original computation.

The API is `PreparedPlan.NewOrderedRequirementContractSession(ctx, model, cases)`.
It makes one prediction before constructing candidates, immediately checks that
proposal, then retains the original finite search without further predictions.
A source shape outside the bounded ordered representation is reported as
declined with zero model calls, and ordinary deterministic search remains
available. A nil model preserves the original deterministic receipt. Fitting
reports unsupported source shapes before enumerating their training labels.

`AuditOrderedRequirements` compares all 528 source cells, every output/condition
row and candidate ordering. The fit report includes this complete input audit.
Two typed-plan training fixtures with identical old inputs and reversed
subtraction order learn distinct paths and each complete all declared cases on
their first proposal. Independent float64 equations, numerical gradients for
both new source connections, full 128-row readers, concurrent inference and CLI
artifact loading have regression coverage. These are implementation and
training-fixture checks. The [separate 48-document study](../studies/ordered-requirement-learning-20261010/README.md)
records 11/40 first-candidate evaluation successes versus 10/40 for the original
requirement model, with all 40 completed through finite checking. Training remains
2/8 and loss stays near uniform; preserving source distinctions has not yet led
to strong joint learning. Both saved models, all regressions and local CPU/RSS
measurements are included. Native compiler loading of this schema is subsequent
integration work.

## Original requirement model

The requirement-conditioned model reads three source-bound inputs: Gooo's
available choices, every declared integer output case and every declared Boolean
condition. Its prediction orders the finite candidates. Gooo then compiles each
candidate and checks the original output and condition requirements.

This follows the [input collision diagnosis](decision-input-audit.md): two bodies
can return the same values while requiring opposite intermediate conditions.
The [declared-condition channel](declared-condition-input.md) now participates
in learning and prediction, alongside the existing source and output arrays.

## Local use

From this SDK checkout, a small command example uses the two included documents:

```sh
go run ./examples/contract-model fit -requirements -out requirement-model.json \
  studies/choice-context-learning-20261010/examples/fresh-bound-k9-r1-w0-alias-g0.json \
  studies/choice-context-learning-20261010/examples/fresh-bound-k9-r1-w0-alias-g1.json

go run ./examples/contract-model search -model requirement-model.json \
  studies/choice-context-learning-20261010/examples/fresh-bound-k9-r1-w0-alias-g0.json
```

Every document supplied to `fit` is training data. Use separate documents for an
evaluation. The earlier published study keeps its original training set and
measurements. The command creates a new model file and reports its fingerprint,
training history, feature versions and full input audit. `-pooling signed_max_abs`
selects the alternative pooling rule; the default is arithmetic mean.

`-requirements` selects a distinct architecture. `-choice-context`, `-goal-pairs`
and the source-literal output profile select other computations and are supplied
separately. Default fitting still uses the original model. Model-free `search`
retains the original deterministic ordering.

The SDK API is `PreparedPlan.NewRequirementContractSession(ctx, model, cases)`.
Initialization makes one local prediction before any candidate executes.
`Advance` uses that ranking immediately, preserves failed attempts, and makes
zero later predictions. A condition rejection at a short attempt budget can
return `ErrNoConditionCandidate` with a partial record; the same session can
continue with its remaining frontier. Output-correct bodies with a failed
condition remain rejected.

This implementation is available through the SDK command and API. Native Gooo
CLI artifact loading is follow-up work. The [fresh model study](../studies/requirement-learning-20261010/README.md)
provides recorded weights, full observations and explicit learning limitations.

## Small joint computation

The model has **13,282 FP32 parameters: 53,128 weight bytes (about 51.9KiB)**.
These are weight bytes, distinct from process memory. Fitting and prediction run
locally in Go on the CPU. Every choice uses the complete 384-cell source array,
1–128 output rows of 32 cells, and 0–128 condition rows of 32 cells.

For source choice `c`, first compute an eight-value source projection `q[c]`.
Two separate learned encoders then read the goals:

```text
q[c]        = source_context_weights × source[c]
output[c,i] = leaky_relu(output_weights × output_case[i] + output_bias + q[c])
condition[c,j] = leaky_relu(condition_weights ×
                   [condition_case[j], targets_choice(j,c)] + condition_bias + q[c])
hidden[c]   = leaky_relu(joint_weights ×
                   [source[c], pool(output[c,*]), pool(condition[c,*])] + joint_bias)
logits[c]   = option_weights × hidden[c] + option_bias
```

Both goal pools have eight cells and the joint hidden layer has 24 cells.
`targets_choice(j,c)` copies the corresponding source-target one-hot cell from
the original condition row. All 32 original cells remain present. The extra
query-relative cell allows a condition to be read in relation to the source
choice currently being ranked. The two pools meet in a nonlinear joint layer,
so output and condition requirements can influence the same decision.

Each pool uses either the arithmetic mean or the signed value with greatest
magnitude per neuron, with positive values winning opposite-sign ties. An empty
condition suite gives an exact zero condition pool. The output suite stays
required. Every activation uses a fixed 0.01 negative slope.

For a complete candidate, add the logits for its chosen binary options.
Training minimizes the negative log probability of the complete acceptable
candidate set, as labelled by Gooo's output and condition checks. All encoders,
source context and joint-layer weights learn together. Candidate probabilities
describe that supplied pool; finite source checks establish candidate validity.

Prediction snapshots each output and condition row once, then reuses the two
bounded arrays for all choices. Training uses the same per-sample snapshot for
forward and backward passes and recomputes case activations as needed. It keeps
no feature tensor for the whole dataset. Caller readers remain immutable during
a call, and concurrent predictions use separate workspaces.

## Input identity and measured scope

The artifact schema `gooo/requirement-conditioned-contract/v1` binds both goal
feature versions, source version, dimensions, bounds, pooling, empty-condition
rule, interaction, activation and weights. The new ranking receipt additionally
binds the exact condition feature bytes and row count. The ordinary source plan
digest continues to bind the authored condition IDs and values.

`AuditRequirements` includes all three input channels. In the opposite-condition
fixture, the old two-channel audit retains a 1/2 first-choice upper bound. The
three-channel audit finds distinct inputs and an upper bound of 2/2. That bound
describes the available information; trained-model validity is measured by
actually selecting and checking bodies.

The regression fixture learns from two Gooo-labelled opposite-condition goals,
loads the saved model, and constructs each valid body in one attempt. Each
construction checks two exact integer output cases and one intermediate
condition, with one initial prediction. This is a training-fixture regression.
The [172-document study](../studies/requirement-learning-20261010/README.md) separately
measures held-out source variants, rare conditions, empty suites and conflicting
goals. First-choice learning remains weak there: 8/32 training documents, with
40 full-input collision groups remaining. Larger choice counts and general
function coverage remain outside that study's two-choice scope.

Tests also cover independent scalar equations, numerical gradient comparisons,
both pooling rules, all 128 rows in both streams, 16 source choices, byte-bound
artifacts, atomic errors, concurrent readers, unchanged legacy inputs and
deterministic absence. A zero-weight model deliberately proposes a condition
failure; continuation checks the next candidate and completes without another
prediction. No general natural-language accuracy follows from these fixtures.
