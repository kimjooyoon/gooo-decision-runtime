# Prepare scalar requirements as a source packet

An Integer Gooo activity can offer a few structural choices and require a fixed
Boolean activity to accept its result. The requirement may call helpers, so a
decision needs the source computation and the roles of its inputs. This package
prepares that source carrier for development of a small local decision model.

`criteriainput` owns `gooo/scalar-criteria-source-input/v1`. It keeps the typed
candidate plan, declared choices and intent, plus fixed predicate graphs whose
first input is the original Integer and second input is the candidate Integer.
Source/contract/graph digests and activity IDs bind the packet's metadata.
They do not become numerical features or prove the compiler's source provenance.

## Published SDK and compiler scope

The source packet API and inspector are available in
[SDK0.2.49-experimental](https://github.com/kimjooyoon/gooo-decision-runtime/releases/tag/v0.2.49-experimental).
Install the Go library with:

```sh
go get github.com/kimjooyoon/gooo-decision-runtime@v0.2.49-experimental
```

The compiler's criteria source-flow export is development source. Its source
[PR #1509](https://github.com/kimjooyoon/meta-ontology-go/pull/1509) was accepted
on dev; inspect the compiler revision you use before expecting this export in a
released CLI. The packet's release adds source preparation. A subsequent
development [numerical encoder](scalar-criteria-features.md) prepares separate
choice, intent and predicate rows; scalar-criteria weights and a compiler ranking
adapter remain next steps.

## Read a compiler export

Use a development Gooo compiler containing the criteria source-flow export:

```sh
gooo body-context --value-flow --activity Fee source.gooo > context.json
go run ./examples/criteria-source-input --compiler-export context.json > prepared.json
```

`FromCompilerExport` projects the explicit `criteria_flow`. It excludes the
older model context and optional `expanded_plan`, where finite selection/held
inputs belong. The new packet has no expected output, outcome or evaluation
answer fields. `Prepare` also refuses candidate `condition_cases`.

The inspector returns source preparation counts, storage layout and the complete
canonical packet. It compiles the typed fallback and offered options to check
their scope and types. It evaluates no candidate or requirement, makes no model
prediction, and performs no training. Its zero execution/prediction fields
describe this preparation, rather than later use by another caller.

## A bounded carrier with explicit roles

- Candidate profile: Integer result, 1–6 binary structural choices, existing
  128-expression/128-statement typed arenas.
- Requirements: 1–8 fixed predicates, up to 512 backward-linked nodes each;
  both root input nodes have the declared Integer roles. Record fields use a
  different contract.
- Complete packet: at most 128 KiB; compiler export: at most 1 MiB. Literal
  spellings share one 64 KiB pool. Limits produce errors without truncation.
- Prepared graph storage: a contiguous 4,096-node arena and eight graph ranges.
  One-based parent/condition/guard edges are local to their graph. Kind/operator
  codes have public name accessors; a literal retains its complete source bytes.
- Read accessors return value copies, immutable strings or copied JSON bytes.
  The typed candidate snapshot also owns its source. Callers must keep the input
  immutable while preparing it.

`Decode` requires canonical field order/values, with insignificant whitespace
allowed. Unknown fields, duplicate keys, trailing input and oversized fixed arrays
are rejected. `Prepared.Canonical()` supplies that canonical representation.
The packet's SHA binds its full carrier. Graph validation checks source shape and
backward edges; it establishes no independent typing or runtime equivalence of
predicate code.

The inspector's byte counts cover the explicit arenas. Whole-process RAM also
includes Go, JSON, source metadata and compiled candidates, so those counts do
not describe process memory or prediction latency.

## Next model work

The packet is implemented source preparation. Development source adds
`Prepared.FeaturesInto`, which reads the prepared source into bounded numerical
rows on request. `Summary.NumericalEncoder` reports API availability, rather than
asserting that a packet has been encoded or that a choice target can be represented.
Weights, prediction API and compiler ranking adapter remain future work. Existing
three-field record and case-conditioned V4 weights keep their own input contracts.
The compiler's scalar-criteria path continues deterministic selection today.

A future experiment should split source families before fitting, score only the
declared paths, and evaluate fixed predicates with selection inputs and separately
held inputs. Record first-choice satisfaction and eventual finite completion with
their own denominators. A source packet alone establishes neither quantity.

[TypeQL's input/returned-value distinction](https://typedb.com/docs/typeql-reference/functions/functions-vs-rules/)
informed the explicit roles. [Scallop's provenance framework](https://www.scallop-lang.org/doc/probabilistic/provenance.html)
offers a reference for keeping learned scores and reasoning structure readable;
Gooo's source lineage and this packet's binding digests retain their own meaning.

## Recorded source preparation

[The retained-source example](../examples/criteria-source-input/README.md) prepared
one choice and one criterion with fourteen nodes. Both input roles and the literal
`11` are retained. The canonical packet is 1,838 bytes. The observed node layout
is 20 bytes, with 81,920 reserved node bytes and a separate 65,536-byte literal
pool. This is one source-packet inspection of saved compiler JSON; it executes no
Gooo source experiment, candidate, predicate, model or training procedure.
The observation keeps the actual SDK development producer and original compiler
export hashes. It is not rebound to a later commit or release.
