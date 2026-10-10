# Branch return roles for the local decision model

The [saved branch study](../studies/branch-feature-discrimination-20261010/README.md)
found a concrete input collision. These two bodies had identical v1 features:

```text
if input < 10 { return 10 } else { return input }
if input < 10 { return input } else { return 10 }
```

Both carried the same Korean/English intent: return the larger value. Their valid
branch choices differed. A deterministic model receiving identical inputs gives
the same answer, regardless of additional training on that representation.

`source_intent_condition_branch_roles_v2` uses the existing array's twenty
reserved cells to describe what each branch returns. The input remains 256 FP32
values (1,024 bytes), and the model remains 6,218 FP32 parameters (24,872 weight
bytes). This changes the information available to the decision model.

## Layout

Cells 0–235 are bit-for-bit identical to v1. The next ten describe the then arm;
the last ten describe the else arm, after normalizing all choices to their
declared fallback. Nested branches contribute their syntactic return sites.

| Offset within an arm | Meaning |
| --- | --- |
| 0 | A return directly uses `input` |
| 1 | A return directly uses an integer or Boolean literal |
| 2 | A return directly reads a local variable |
| 3 | A return uses a binary expression or operator hole |
| 4 | A directly returned local was declared with an input initializer |
| 5 | A directly returned local was declared with a literal initializer |
| 6–7 | A directly returned local is the first/last declaration in source traversal |
| 8 | A directly returned local has a reachable assignment somewhere in the body |
| 9 | Number of distinct return statements, capped at 16 |

Flags are 0 or 1 in the FP32 input. Counts are divided by 16. Fields are combined
by presence within each arm; their order inside an arm is not encoded. Other
choice kinds receive twenty zeros. The projection uses fixed arrays and reads no
output labels, selected candidate, seed or model weights.

Initializer and assignment fields describe syntax. They do not infer a local's
value after execution. Multiple declarations with the same name still decline
this optional representation. Numeric literal values, arbitrary dataflow and
full lexical binding are further work; this compact representation still loses
information. The exact int64 condition-observation encoding remains unchanged.

## Bind the input version to training and search

```go
input, err := prepared.InitialConditionInput()
// Handle err.
var features [decision.FeatureDim]float32
err = input.FeaturesIntoVersion(choiceID, decision.ConditionBranchFeatureVersion, &features)
// Handle err; build labelled samples from complete, checked candidates.
model, history, err := conditiondecision.FitForFeatures(
    ctx, trainingSamples, options, decision.ConditionBranchFeatureVersion,
)
// Handle err; keep evaluation programs outside trainingSamples.
```

`NewForFeatures` is available when constructing explicit tensors. Model artifacts
record the feature version, and fingerprints bind both that version and exact
weight bits. `New`, `Fit` and `FeaturesInto` retain their v1 defaults. Previously
published v1 fingerprints and feature arrays are checked as regression fixtures.
Renaming a v1 artifact's feature field does not produce trained v2 weights.

`NewConditionSession` and `SearchConditionBatches` dispatch using the model's
version for both initial ranking and later condition feedback. V2 receipts
include `feature_version` and the actual input hashes. A session rejects a model
with the same tensors but a different feature version. Source conditions, typed
checks and output cases still decide whether a proposed body can be accepted.
The nil-model route retains deterministic search.

## Evidence and remaining work

Regression tests load the two original saved documents and verify that v1 still
collides while v2 distinguishes their return roles. They also cover renamed
locals, assignments, reversed fallbacks, nested returns, atomic errors, immutable
feedback, exact large integers, concurrent projection and versioned search.
They do not rerun the historical probe or establish new model accuracy.

The subsequent [paired source study](../studies/branch-role-learning-20261010/README.md)
publishes a trained v2 artifact and a fresh v1 comparison. V2 fits the training
layouts better (8/8 versus 4/8) but scores lower on held-out wording (6/16 versus
8/16). Local and nested challenge scores remain unchanged. The fixed original
protocol, both weights, every finite result and local resource observations are
retained. Compiler CLI consumption of v2 and typed output-mismatch feedback are
still subsequent work.
