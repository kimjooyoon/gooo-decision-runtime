# Value-flow decision model

This experimental Go model scores source-defined Gooo choices using the
384-cell source/intent/condition/output/value-flow representation. The network
is 384 → 24 → 2, with 9,290 FP32 parameters (37,160 weight bytes). It emits two
choice scores; it does not produce text or program tokens.

`Fit` learns probability mass over supplied acceptable complete masks with
deterministic full-batch CPU updates. `PredictChoicesInto` uses caller-owned
arrays for up to 16 choices; `PredictInto` ranks up to 64 explicit masks.
The closed artifact schema is `gooo/flow-candidate-decision/v1`. Old 256/320-cell
artifacts cannot be loaded into this model or relabelled as new weights.

`PreparedPlan.NewFlowSession`, `ReconsiderFlow` and `SearchFlowBatches` connect
the model to the existing finite frontier. Each selected candidate is compiled
and checked before its failure can influence another ranking. A nil model
continues deterministically. Scope declines, cancellation, model identity and
the session's nonblocking locks retain their existing contracts.

The [fixed paired learning study](../studies/value-flow-learning-20261010/README.md)
publishes two freshly trained artifacts and every selected body. Adding static
flow increased first-choice validity on the 96 sources outside training from
54 to 73. Assignment-form first choices stayed at 8/16, and their observed-context
judgments regressed. All four finite-search modes completed the authored cases.
The flow-on model's feedback added 23 calls without reducing attempts. These
related source variants give limited evidence, with recorded regressions.

The fixed-dimension mathematical implementation is derived from
`executiondecision` at revision `6df5978f8825c1ad1343ac4b4f766c6f8a489ef4`.
This keeps previous model bytes and training implementations stable during the
representation comparison. The duplicated numerical routines are a maintenance
cost; shared fixes must be reviewed in all three packages. Consolidation needs
separate evidence that the published predictions and model identities remain
unchanged. The prepared search/frontier implementation is shared.

## Inspecting a decision

`Model.ExplainInto` runs one ordinary prediction and returns the hidden
activations, two option scores for each choice and the supplied candidate
scores in caller-owned arrays. The prediction uses exactly the `PredictInto`
path. This optional diagnostic makes inactive hidden paths and identical scores
visible without changing weights or the normal prediction interface.

The arrays report what the model computed. They do not prove why training
produced those weights or whether a body satisfies the source cases. The
explanation is transactional on errors and can be used concurrently with
separate caller storage. Unused array cells are zero.

The fixed [intent diagnostic protocol](../studies/intent-selection-diagnostics-20261010/protocol.txt)
compares frozen v5/v6 models on opposite Korean/English intents in32 direct
Gooo sources, with64 explicitly counted diagnostic forward passes and no fit
or candidate execution.
