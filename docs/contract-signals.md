# Seeing how declared goals reach a decision

`contractdecision.Model.ExplainInto` returns the actual intermediate values of
one normal prediction. It reads all declared cases once and uses the same FP32
computation as `PredictInto`. It adds no training or body execution.

The caller supplies a workspace, prediction and explanation destination:

```go
var workspace contractdecision.Workspace
var prediction contractdecision.Prediction
var trace contractdecision.Explanation
err := model.ExplainInto(inputs, cases, masks, &workspace, &prediction, &trace)
```

The explanation contains the eight-coordinate case summary, the case index
selected for each signed-pooling coordinate, the hidden value after source and
bias products, the joint value after adding case products, hidden activations,
option scores and candidate scores. Mean pooling reports winner index -1.
Unused array cells are zero; errors preserve all caller destinations. Immutable
weights can be shared across requests that own separate workspaces and outputs.

`SourcePrefix` includes bias. `Joint-SourcePrefix` includes FP32 rounding from
the original addition order. These values describe an observed calculation.
Their magnitude alone does not establish why a semantic decision was wrong.
Neither option scores nor prediction probabilities establish program correctness.
The compiler continues to check the authored finite outputs and conditions.

The [fixed diagnostic protocol](../studies/contract-signal-diagnostics-20261010/protocol.txt)
uses both frozen models and every original source, retaining correct and wrong
choices. It compares opposite goals with identical source arrays at each stage.
The API leaves artifact bytes, model fingerprints and the default model intact.

The recorded producer used Go 1.27.2 on darwin/arm64 with fused FP32
multiply-add instructions. [Go permits operation fusion](https://go.dev/ref/spec#Floating_point_operators),
so exact floating-point intermediates may differ on another architecture.
The record auditor explicitly reconstructs the producer's arithmetic and keeps
bit-exact comparisons. Source integers and authored output checks remain exact.

The [completed diagnostic](../studies/contract-signal-diagnostics-20261010/README.md)
found different case summaries, hidden values and scores for all 97 opposite-goal
pairs in both models. Yet 61 mean-model pairs and 62 signed-pooling pairs kept
the same path. All 392 traced calls exactly retained the original saved scores
and selections. The evidence supports studying how goal-dependent score margins
are learned; it does not establish a unique cause or an accuracy improvement.
