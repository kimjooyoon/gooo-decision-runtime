# Read scalar choices and requirements as numerical source features

`Prepared.FeaturesInto` converts the scalar criteria packet into
`gooo/scalar-criteria-features/v1`. This development API prepares inputs for a
small local decision model that selects Gooo's declared structural paths.
It makes no prediction and evaluates no source requirement.

The input packet is published in SDK0.2.49-experimental. The encoder described
here is a subsequent development change; check the SDK revision before using it.
It needs its own fitted weights and compiler ranking adapter. Existing record
and case-conditioned weights retain their existing feature contracts.

## Bounded rows with separate meanings

The caller supplies a reusable `Features` value. Six choice rows and eight
predicate rows each reserve 256 float32 values: 14,336 numeric bytes in total.
Active counts identify the declared rows; unused rows stay zero. Metadata,
prepared source, compiler state and process memory are additional storage.

| Row | Range | Meaning |
| --- | --- | --- |
| Choice | 0–63 | Existing fallback-normalized typed source facts, including the two offered options in declaration order |
| Choice | 64–255 | Ordered Unicode 1/2/3-grams from the declared Korean/English intent |
| Predicate | 0–15 | Node kind counts; vocabulary follows the source graph tables |
| Predicate | 16–47 | Operator counts; zero means no operator |
| Predicate | 48–79 | Original-input/candidate-result flow masks, separately through value parents and conditions/guards; return nodes have separate counts |
| Predicate | 80–207 | Fingerprints of ordered parent, condition and guard structure |
| Predicate | 208–239 | Complete canonical source literal spelling, hashed |
| Predicate | 240–255 | Fingerprints of source return nodes |

Each listed channel is L2-normalized separately; an empty channel stays zero.
Source kind/operator codes are one-based table indices. Input role bits are
`1=original_input`, `2=candidate_result`, `3=both`, `0=neither`. Slots 48–51
count value masks, 52–55 control masks, 56–59 return value masks and 60–63
return control masks; slots 64–79 are reserved zero in this version.

Choice source facts follow `pathplan.SourceFeatures`: scope ambiguity and
unreachable targets can decline that optional representation even when the
typed candidate can compile. On any encoder error the caller's output is
unchanged. The prepared packet and deterministic candidate path remain usable.

## Source structure, text and bindings

The graph fingerprint is computed in backward-edge order. Parent positions,
conditions and guards remain distinct; parent fingerprints are propagated
without embedding raw node positions. Every prepared node contributes, including
source branches whose execution has not been observed. Pure copy/call wrappers
also contribute; this encoder promises no algebraic or wrapper invariance.

Intent uses streaming Unicode code points with source case, punctuation and
spacing retained. It needs no tokenizer or generated token sequence. N-grams
and structural/literal fingerprints are hashed into bounded channels. Collisions
and information loss are possible; these features establish no semantic
equivalence, truth probability or accuracy percentage.

Activity and predicate IDs, option labels, fallback labels and source/plan
digests stay outside numeric rows as binding metadata. A model consumes only
the documented numeric rows and active counts. It must use the new schema and
return choices through the corresponding metadata. Metadata is not a learned
feature. Renames do not authorize a model to invent new structural options.

`ConditionCases`, expected outputs, held inputs and execution outcomes are
excluded by the source packet. The encoder does not infer whether a predicate
passes, or independently prove its code is well typed. Fixed predicate evaluation
and source-bound provenance keep their compiler-owned meaning.

## Use and next model step

```go
var features criteriainput.Features
if err := prepared.FeaturesInto(&features); err != nil {
    // Keep the existing deterministic selection path and report the reason.
    return err
}
```

The [source example](../examples/scalar-criteria-features/README.md) exports one
fresh Gooo source and prepares its features. It runs no candidates or predicates.

A subsequent model can share small weights across choice and predicate rows,
then rank the finite declared candidate masks. Its architecture, fitting and
source-family split need their own evidence. First-choice requirement
satisfaction and eventual finite completion must retain separate denominators.
The presence of numerical features establishes neither result.
