# Source relations for the local decision model

Gooo describes the choices available in a body and the cases it must satisfy.
The runtime can ask a small model which combination to assemble first, then
run the declared cases. With no model, the finite deterministic search remains
available through the same runtime.

The explicit `source_intent_condition_output_relational_flow_v6` input adds
six relations among the then-return, else-return and two predicate atoms.
For example, a returned constant may equal a predicate constant. Two integer
constants may have a known order. A runtime input compared with a constant
remains unresolved until execution supplies evidence.

Each eligible single-branch form uses six groups of equality/less/greater
flags in cells236:254. Cell254 is reserved; marker255 is3. The array remains
384 FP32 cells. Source intent, condition/output observations and exact integer
atoms retain their previous cells. Unsupported forms retain their complete
v5 arrays. Comparisons use int64 directly, including values beyond2^53 and
both int64 limits.

`ConditionInput.ExecutionRelationalFlowFeaturesInto` exposes the array.
`flowdecision.FitForFeatures` accepts the explicit v6 version and produces a
separately identified artifact. `SearchFlowBatches` dispatches by that artifact's
feature version. Existing v4/v5 artifacts keep their original representations.
Weights require training against the representation they consume.

The [fixed study protocol](../studies/relational-flow-learning-20261010/protocol.txt)
uses the existing162 Gooo-source fixtures and one fresh16-row CPU fit with9290
parameters. Its purpose is to measure whether explicit source relations improve
candidate selection without enlarging the network. Observed case completeness,
model probabilities and a proof of all possible program behaviors are distinct
quantities. The [original result](../studies/relational-flow-learning-20261010/README.md)
regressed from101/162 to82/162 first-choice valid candidates against historical
v5. Both bounded v6 searches still completed162/162 authored case suites. These
weights remain an explicit experiment; the result motivates investigating how
intent affects branch choice before increasing training volume.

The compiler CLI currently has separate v5 integration work. This SDK addition
does not by itself expose v6 through an installed compiler release.
