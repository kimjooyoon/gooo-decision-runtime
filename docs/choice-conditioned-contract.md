# Let each source choice read the declared cases

`ChoiceModel` changes where source structure meets the required examples.
The existing model pools the cases once and gives the same summary to every
choice. This model gives each choice its own case summary, using all original
384 source cells and 32 declared-case cells without replacing any of them.

For choice `c`, case `i` and eight-cell hidden row `h`:

```text
source_offset[c] = context_weights * source[c]
case_hidden[c,i] = leaky_relu(case_weights * case[i] + bias + source_offset[c])
pool[c] = mean or signed_max_abs over every case_hidden[c,i]
hidden[c] = leaky_relu(source_weights * source[c] + pool_weights * pool[c] + bias)
option_scores[c] = output_weights * hidden[c] + bias
candidate_score[mask] = sum of selected option scores
```

The source offset enters before the case nonlinearity. The trained model can
therefore interpret the same goal differently at different source choices.
Source features already describe local structure, option orientation, intent
and available value-flow facts. Their existing representation limits remain.
This computation does not evaluate a candidate body, produce tokens or create
new operations. Gooo still owns the available choices and checks every declared
output and condition after selection.

## Size and bounded work

There are **12,818 FP32 parameters / 51,272 weight bytes**, including 3,072 new
source-to-case weights. This is 12,288 bytes more than the earlier model.
Parameter count is independent of the number of cases and choices.

On the measured Darwin arm64 layout, `ChoiceModel` occupies 51,276 bytes and
the caller-owned `ChoiceWorkspace` occupies 58,296 bytes. Prediction uses another
temporary workspace of that size to keep error outputs atomic; these numbers
exclude stack/runtime overhead and are not process RSS. Workspace arrays include
a 16KiB snapshot of all 128 case rows. Each caller row is read exactly once per
prediction, then reused for each of up to 16 choices. No case prefix is sampled.

The model kernel's allocation regression test observes zero heap allocations
with pre-existing inputs, reader and destinations. The first implementation
allocated one temporary workspace per call through an interface; a direct read
of the owned snapshot removed that allocation. Source preparation and artifact
loading have separate costs. A fresh 388-document study measured 37.584 µs
median initial prediction, 1.725 s CPU fitting and 72.27 MiB maximum RSS for the
whole two-model experiment. See the [recorded comparison](../studies/choice-context-learning-20261010/README.md)
for timing scope, parameter differences and regressions.

Training captures each sample once per epoch, scores all choices and recomputes
each choice once for backpropagation. It uses fixed gradient arrays and retains
no whole-dataset case tensor. The objective is ordinary candidate NLL plus the
existing separate L2 update. The paired-goal objective and source-literal case
profile are not combined with this model in this version.

## Local use

Use compiler-exported `pathplan.Document` JSON files containing a typed plan and
declared test cases. Training labels come from complete candidate checks.

```sh
go run ./examples/contract-model fit -choice-context -pooling signed_max_abs \
  -out choice-model.json training-a.json training-b.json
go run ./examples/contract-model search -model choice-model.json evaluation.json
```

Search recognizes the artifact and immediately makes one initial prediction
before finite construction. There are no later model calls in this session.
Omit `-model` for the deterministic frontier. An explicitly provided empty or
invalid model is an error. Public APIs are `NewChoiceConditioned`,
`FitChoiceConditioned`, `DecodeChoiceConditioned` and
`PreparedPlan.NewChoiceContractSession`.

The artifact `gooo/choice-conditioned-contract/v1` binds the interaction rule,
pooling, v6 source features, original declared-case v1, activation and both
weight arrays. Legacy loaders reject it. Existing v1/v2/v3 contract models keep
their formats and defaults. The ordinary released compiler does not yet load
this new artifact; the Go example command and SDK session support it.

## Inspect one decision

The development API adds `ChoiceModel.ExplainChoicesInto` and
`ChoiceModel.ExplainInto`. They record the numerical states in the same pass as
the ordinary prediction. Each caller case is captured once and shared by the
choice computations. The first method returns all choice logits without
enumerating combinations; the second scores the explicitly supplied masks.

```go
var workspace contractdecision.ChoiceWorkspace
var prediction contractdecision.ChoicePrediction
var trace contractdecision.ChoiceExplanation
err := model.ExplainChoicesInto(sources, cases, &workspace, &prediction, &trace)
```

The trace includes each choice's source-conditioned case bias, eight-cell case
summary, pooling indices, source prefix, joint values, hidden activations and
option scores. Candidate scores are present when masks were supplied. Active
signed-pooling indices are zero-based rows in the original case list. Mean
pooling uses `-1` because it combines all rows. Unused storage is zeroed on every
successful call, and errors preserve all caller destinations.

A pooling index identifies which case supplied one learned coordinate. Multiple
coordinates and downstream weights jointly determine the scores. The arithmetic
states support diagnosis; source outputs and condition checks establish whether
the resulting Gooo program meets the authored contract.

The local example command accepts an existing choice-conditioned artifact and a
Gooo-derived document:

```sh
GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./examples/contract-model explain \
  -model studies/choice-context-learning-20261010/result/model-choice.json \
  studies/choice-context-learning-20261010/examples/fresh-bound-k9-r1-w0-alias-g0.json
```

Its JSON links the exact declared int64 cases and choice IDs/labels to their
source arrays and recorded computation, with model/plan/case hashes. It makes
one prediction, performs zero candidate executions and training updates, and
enumerates zero candidate masks. `predict_ns` includes case capture, prediction
and recording; file loading and JSON output happen outside that interval.
Contradictory authored cases remain visible in the output for inspection.

This diagnostic API and command are development additions after SDK0.2.37.
The published weights and original study observations are unchanged. Unit
fixtures exercise both pooling modes, all 16 choices and 128 cases, exact integer
goals, ordinary-prediction equivalence, error atomicity and concurrent readers.

## What is established

- Separate scalar equations agree with inference for both pooling modes.
- Numerical derivatives exercise the old layers and every new context neuron.
- Zero context weights reproduce the old model's logits and probabilities.
- All 128 cases and 16 choices contribute; reader errors, overflow and invalid
  artifacts preserve destinations. Separate workspaces support concurrent use.
- Local fitting updates both weight sets. Serialization, command routing,
  immediate finite construction, cancellation and model absence are exercised.
- A two-goal learning regression uses identical source arrays and opposite
  compiler-validated labels. After fitting, each goal selects its valid path and
  completes construction in one attempt. This is a training fixture.
- Constructed outputs retain exact large integers, including values above 2^53.

The [fresh frozen comparison](../studies/choice-context-learning-20261010/README.md)
then measured 197/388 → 221/388 valid first choices against a separately fitted
global-pooling model: 87 improvements, 63 regressions, 238 unchanged. Both models
used the same 32 training rows; 356 documents stayed outside training. All modes
completed 386 satisfiable contracts and exhausted two contradictions. The new
model still chose the same path for 163/193 opposite-goal pairs, and unseen
multiplication fell from 32/64 to 31/64. Its additional parameters and compute
are disclosed. The previous published default model remains available unchanged.
