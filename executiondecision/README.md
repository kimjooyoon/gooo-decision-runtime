# Execution decision model

A small CPU network for ranking declared Gooo choices using source, intent,
condition observations and exact output counterexamples. It emits choice scores.

The architecture is 320→24→2 with 7,754 FP32 parameters. `Fit` learns probability
mass over acceptable complete candidate masks, using deterministic full-batch
updates. `PredictChoicesInto` scores up to 16 choices with caller-owned fixed
arrays. `PredictInto` scores a supplied pool of up to 64 complete masks.

See [the input, search API and evidence limits](../docs/execution-feedback.md).

This fixed-dimension implementation is derived from the sibling
`conditiondecision` package at source `e7fb6f0db799e49d91d183b2fef83fd00c3f6148`.
The experiment keeps the original 256-cell model's training code, artifact and
fingerprints unchanged while the 320-cell ABI is evaluated. Shared mathematical
fixes should be reviewed in both packages. This duplication is a maintenance
cost to revisit after measuring the new representation.
