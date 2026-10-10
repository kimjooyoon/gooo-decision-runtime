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
loading have separate costs. End-to-end inference/training latency and process
RSS for a fresh corpus have not yet been measured.

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

These are implementation tests, not measured general decision quality. The
[previous literal experiment](../studies/source-literal-contract-20261010/README.md)
regressed; this new architecture has no corpus improvement claim yet. A next
comparison must fix fresh Gooo evaluation contracts before fitting, keep them
outside training, and report the extra parameters and per-choice work alongside
first-choice validity, output/condition completeness, finite completion and all
regressions. The previous published default model remains available unchanged.
