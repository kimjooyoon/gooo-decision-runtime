# An explicit computation for retaining negative signals

The [intent diagnostic](../studies/intent-selection-diagnostics-20261010/README.md)
found eight pairs of opposite intents whose v6 branch activations were all zero.
The new optional `leaky_relu_0.01_v1` activation passes negative values multiplied
by the fixed FP32 value0.01. Positive values retain their original value. The
derivative at zero uses the negative slope. This keeps the384→24→2 dimensions,
9, 290 parameters and 37, 160 FP32 weight bytes.

`flowdecision.NewForActivation` and `FitForActivation` select this computation
explicitly. Existing constructors and training entry points retain `relu_v1`.
`Model.Activation` and `ArtifactSchema` expose the choice. A model remains
immutable after construction and uses caller-owned workspaces.

Old ReLU files retain `gooo/flow-candidate-decision/v1`, omit activation and keep
their exact serialized bytes and fingerprints. New leaky files require
`gooo/flow-candidate-decision/v2` and the exact activation identifier. A missing,
unknown or schema-mismatched identifier is rejected. The model fingerprint
binds both feature and computation contracts, so the same weights under another
activation cannot silently replace a model in an active feedback session.

The path runtime derives its seed domain from the actual artifact schema and
continues to check declared conditions and output cases after selection. The
no-model route remains available. A compiler built against an older SDK requires
separate integration before it can accept the new schema.

The [fixed experiment](../studies/leaky-flow-learning-20261010/protocol.txt) uses
the exact previous 16 training arrays and 400-epoch schedule for one fresh fit.
It records first-choice validity, internal traces and final case completeness.
The original ReLU report is a saved baseline; its fit is not repeated. The
activation addresses an observed inactive path, while its effects on other
failures require measurement.

The [original measurement](../studies/leaky-flow-learning-20261010/README.md)
increased first-choice declared-case validity from 82/162 to 141/162, with 80
improvements and 21 regressions against the saved ReLU v6 results. Both search
modes completed 162/162 suites in 183 attempts each. The model retained the same
size; the single local CPU fit took 133 ms. This finite fixture result leaves
min-selection and nested-control failures to address.
