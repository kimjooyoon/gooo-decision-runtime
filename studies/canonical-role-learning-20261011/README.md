# Learn the two Gooo decisions separately

Status: **source corpus and protocol prepared; original experiment not run**.

The preceding pooling study found weak responses to changed intermediate goals.
A read of its existing records shows64 related pairs with equal source/output
inputs and opposite conditions: both models change their scores, but neither
changes its first selected path. That is a descriptive observation, not a cause.

This experiment compares a shared canonical model with two independently trained
models: one for comparison operand order, one for branch layout. Both read the
same source and requirement arrays. Role targets are projected from each training
document's Gooo-checked accepted mask. The source still owns the candidate space.

Simply separating two loss terms would be equivalent for the existing additive
scores over four masks. Here the role fits have independent weights. The split
policy therefore has **38,068 parameters /152,272 FP32 weight bytes and two initial
model calls**, versus19,034/76,136 and one for the joint policy. It is a complete
recipe comparison with disclosed extra resources, not a matched-size ablation.

The [frozen protocol](protocol.txt) fixes three fresh fits,64 training documents,
128 related evaluation documents and64 excluded-family documents. Constants are
new; known templates and Korean/English wording are reused. It records original
failures, response to goal changes, per-choice and full-body completion, all
candidate checks and total cost. No model is adopted as a new default.

All policies use one explicit four-mask ranking/compilation/checking loop in the
research producer. It executes typed bodies from Gooo and preserves the emitted
Gooo/Go. The compiler's native model loader and general SDK session frontier are
outside this experiment. The split-policy manifest is research metadata only.

`freeze/` writes source and oracle values. `producer/` is a separate module using
the two existing local checkouts; its replacements name their relative locations.
Preflight only reads/lowers the frozen corpus. Tiny K=5 regressions, absent from
the corpus, check role labels, score selection and prediction-before-execution.
Do not invoke the original producer until this source/protocol is publicly frozen.
