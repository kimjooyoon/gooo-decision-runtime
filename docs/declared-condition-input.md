# Read the conditions before constructing a body

A Gooo body may require both a final output and a particular intermediate
Boolean condition. `ContractInput` now exposes these as separate source-bound
streams: `CaseFeatures` for outputs and `ConditionFeatures` for conditions.
The source owns their exact values, targets and order.

This makes the [condition-only input collision](decision-input-audit.md)
distinguishable in an explicit new channel. The two contracts retain identical
legacy source/output arrays while their expected Boolean cells differ. The
published contract models still consume the original source/output channels.
A model that learns from conditions needs a new artifact computation contract,
training path and measured integration; those consumers remain future work.

## Inspect a document locally

```sh
go run ./examples/contract-model inspect \
  studies/choice-context-learning-20261010/examples/fresh-bound-k9-r1-w0-alias-g0.json
```

This included document contains two choices, eight output cases and three
conditions. Supply another strict Gooo-derived typed path document as needed,
using the same format as `fit` and `search`.
The compiler or caller binds that document to authoritative `.gooo` source.
Inspection prepares the typed source and prints its choices, exact output cases,
exact condition cases and all numerical feature rows. It reads every authored
row, including the 128th condition. It requires no model file and records zero
model calls, candidate executions and training updates. It writes JSON to stdout.

The report schema is `gooo/contract-input-inspection/v1`. `plan_sha256` binds
the prepared source including conditions; `output_cases_sha256` binds the output
suite. The three channel versions are explicit. `declared_conditions: []`
means there are no authored Boolean requirements. Correctness is established
later by Gooo's ordinary output and condition checks.

JSON case values are signed 64-bit integers. Consumers should decode them into
an exact integer type. The feature cells below preserve the original bits.

## Condition channel layout

The version is `declared_boolean_condition_v1`, with 32 float32 cells per row.
The target is an index in the same prepared choice order as the source arrays.
Choice IDs are resolved from the immutable plan before encoding.

| Cells | Contents | Scaling |
| --- | --- | --- |
| 0 | Row present | 1/8 |
| 1, 2 | Expected false, expected true | One active cell, 1/8 |
| 3, 4 | Integer input is zero, is negative | 1/8 when true |
| 5–12 | All eight two's-complement input bytes, big endian | Byte/2048 |
| 13 | Zero-based condition row index | Index/1024 |
| 14 | Total declared condition count, 1–128 | Count/1024 |
| 15 | Total source choice count, 1–16 | Count/128 |
| 16–31 | Target source choice | One active cell, 1/8 |

Power-of-two scaling preserves each byte exactly, including adjacent values
above 2^53 and both int64 extrema. The encoder never converts a complete int64
value to floating point. Every cell describes authored input; observed truth,
candidate masks and pass/fail results belong to the separate execution channel.

## Source-bound API

After `prepared.InitialContractInput(outputCases)`, use:

```go
version := input.ConditionFeatureVersion()
count := input.ConditionCount()
condition, err := input.ConditionCase(index) // Exact choice ID, int64 and bool.
row, err := input.ConditionFeatures(index)  // [32]float32 for a model consumer.
```

Check errors from each call. `ConditionFeaturesInto` supports caller-owned
scratch. Readers share the immutable prepared plan and add no case tensor or
mutable cache. Bounds and invalid targets produce errors while preserving the
destination. Concurrent readers use separate output arrays.

The regression suite reconstructs every integer bit, checks all 16 target
positions, retains 128 source-bound rows under caller mutation and concurrent
reads, and exports the opposite-condition pair without changing old inputs.
These checks establish representation fidelity. Model quality will require
training and finite construction observations using this channel.
