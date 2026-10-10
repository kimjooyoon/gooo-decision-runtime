# Condition candidate decision model

This experimental Go model consumes the separate source, authored intent and
condition-observation channels introduced by `ConditionFeaturesInto`. A shared
256 → 24 → 2 ReLU network scores each declared choice. A complete candidate mask
receives the sum of its selected option scores. Softmax ranks 1–64 supplied masks
over 1–16 choices, with a deterministic smaller-mask tie break.

`Fit` trains on the probability mass of a complete acceptable candidate set.
For example, if only `01` and `10` satisfy the source, they remain the two targets
of one training sample. Labels are not flattened into independently permissible
bits. Additive choice scores still restrict which joint distributions the model
can represent; source evaluation must check the selected complete candidate.

## Local cost and contract

- 6,218 FP32 parameters: 24,872 bytes of resident weight values.
- Fixed inference arrays, caller-owned workspace and output; valid inference
  allocates zero heap objects in the contract test.
- CPU full-batch training with explicit seed, epochs, learning rate and L2.
- One closed JSON artifact schema, `gooo/condition-candidate-decision/v1`, bound
  to `source_intent_condition_channels_v1` and the exact architecture.
- Artifacts in this first implementation use FP32. Earlier ternary path models
  retain their own schema and feature version.

Relative candidate scores are uncalibrated. Changing the supplied candidate pool
changes the distribution. The caller owns candidate enumeration, source checks,
output tests and the decision to accept or continue searching.

## Use

Prepare source-bound feature arrays with `PreparedPlan.InitialConditionInput` or
`ObserveConditionInput`, then call `FeaturesInto` for each declared choice, in
plan order. Each mask's bit `i` selects option 0 or 1 at that same choice index.

```go
var work conditiondecision.Workspace
var prediction conditiondecision.Prediction
err := model.PredictInto(features, candidateMasks, &work, &prediction)
// Handle err and check prediction.Selected with the Gooo source contract.
```

The module currently provides training, serialization and finite-pool ranking.
Compiler CLI provider integration and a compressed version are subsequent work.
The fixed [source-based study](../studies/condition-candidate-20261010/protocol.txt)
separates initial judgments, observations, new wording and local-variable bodies.

## Reproduce a new observation

From this SDK checkout, choose a fresh output directory:

```sh
GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./examples/condition-fit \
  studies/condition-candidate-20261010/dataset.json /tmp/my-condition-model-run
```

This performs a new fit and 120 rankings. It does not reproduce historical timing.
The dataset contains each original `.gooo` source, its digest and the compiler's
typed document. The study's nested exporter module imports a sibling
`meta-ontology-go` checkout; its protocol names the exact lowering revision.
