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

There are no published trained v4 weights or source-level learned quality
measurements yet. The preceding static projection established which information
was missing from the old input. Synthetic training and wired routing tests only
establish numerical connectivity and runtime behavior.

The fixed-dimension mathematical implementation is derived from
`executiondecision` at revision `6df5978f8825c1ad1343ac4b4f766c6f8a489ef4`.
This keeps previous model bytes and training implementations stable during the
representation comparison. The duplicated numerical routines are a maintenance
cost; shared fixes must be reviewed in all three packages. Consolidation needs
separate evidence that the published predictions and model identities remain
unchanged. The prepared search/frontier implementation is shared.
