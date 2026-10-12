# Explicit V3 profile with prose excluded

This optional profile makes decisions from the original source structure,
source/requirement relationships, numeric outputs and Boolean expectations.
It is useful when examples already specify the desired behavior and changes in
the accompanying description should leave the initial decision stable.

Think of a form with a description and concrete examples. This profile keeps
the description in the source record, and gives the small model the examples
and the available switches. It is published in SDK0.2.45; public
Gooo0.6.34 has the ordinary V3 adapter and requires a subsequent update to load
this explicit profile.

## Input and model identity

The input has528 float32 cells. Exactly128 cells in the half-open range[64,192)
are cleared. The remaining400 values, every numeric case and every Boolean case
retain their ordinary V3 interpretation. Preparing this view performs zero model
calls, candidate executions or training updates. The source intent, full plan
identity and declared requirements remain available for inspection and receipts.

| Field | Value |
| --- | --- |
| Model schema | `gooo/source-relative-v3-prose-neutral-model/v1` |
| Source features | `canonical_branch_condition_output_relations_no_intent_v1` |
| Intent policy | `DECLARED_REQUIREMENTS_ONLY_V1` |
| Numerical model | 19,034 FP32 parameters;76,136 tensor bytes |
| Original weight identity | `source_v3_fingerprint`, canonical V3 serialization of the same tensors |

The decoder retains the existing bounded V3 tensor, output and condition checks.
The prediction API rejects populated intent cells before touching caller output.
The new artifact fingerprint differs from ordinary V3 because its input policy
differs. Weight bytes alone do not describe total process memory.

## Inspect, convert and assemble

```sh
GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./examples/contract-model inspect \
  -source-relative-v3-prose-neutral-source document.json

GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./examples/contract-model repackage-prose-neutral \
  -input original-neutral-v3.json -out explicit-neutral.json

GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./examples/contract-model search \
  -model explicit-neutral.json document.json

GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./examples/contract-model search document.json
```

Inspection uses schema`gooo/contract-input-inspection/v8`, reports the policy and
cleared range, and retains all source contexts and requirement rows. Select one
inspection format at a time.

Conversion is for weights already trained with this exact clearing policy. It
changes metadata and preserves tensors. It records the input file SHA, original
canonical V3 fingerprint and new fingerprint with zero inference or training.
Its output must be a new file. The converter cannot verify how supplied weights
were trained; the published study records that history separately.

Search predicts once before executing a candidate and hashes the neutral arrays
actually consumed. Later advances make zero new predictions and check the
original source-owned bodies. Nil model retains deterministic ordering.
Unsupported source shapes retain a reason and deterministic finite search.

The Go APIs are `ContractInput.SourceRelativeV3ProseNeutralFeaturesInto`,
`PreparedPlan.NewSourceRelativeV3ProseNeutralContractSession`, and
`contractdecision.NewSourceRelativeV3ProseNeutral`,
`DecodeSourceRelativeV3ProseNeutral`, `FitSourceRelativeV3ProseNeutral` and
`RepackageSourceRelativeV3ProseNeutral`. Each concurrent caller owns its
workspace and session.

## Recorded quality and its scope

The [immutable factorized result](https://huggingface.co/asketeddy/gooo-canonical-decision-tiny-v1/tree/0aa1dd8c51e730632f0f4bc590b5498799178492/research/factorized-decision-20261012/result)
contains the original neutral model under`v3-prose-neutral/model.json`, with
SHA256`8d61ec5ab6422a4706792c7ea22aab95876686b74e2d33b0d7bd62943ba969ec`.
Its tensors can be repackaged by the command above. Repackaging supplies a loader
contract; it contributes no new quality measurement.

That model meets every declared requirement on its first body in264/512 held
documents, then reaches512/512 through776 candidate checks. The512 variants
share16 goal contracts from four families. Separate checked probes pass4096/4096
observations, repeating128 unique contract/input pairs. These are interpreter
observations, with zero native executions and no latency or CPU collection.

Korean/English and wording pairs have identical neutral arrays, so their256/256
agreement follows from deterministic inference on identical inputs. The model's
remaining gap is concrete:248 first bodies fail some outputs, although all1536
Boolean requirements match. Return-branch association and layout stability need
further work. First-choice quality and completion after checks remain separate
metrics; the existing defaults remain unchanged.

The [frozen-input diagnosis](../studies/factorized-input-diagnosis-20261012/README.md)
locates a stronger layout-specific gap: the neutral model completes 0/128 first
choices when raw mask 1 is required and 128/128 when mask 3 is required. No held
contract completes every sibling variant on the first choice. Exact raw inputs
have no conflicting accepted-mask groups; internal representation loss remains
unmeasured. The Go reader uses saved arrays and outcomes with zero new model or
candidate calls.

The [V4 SDK input contract](source-relative-v4-runtime.md) preserves separate
then/else literal relationships for a future fit. It has its own artifact schema;
its quality has not yet been measured and the published neutral weights above
retain their original V3 meaning.
