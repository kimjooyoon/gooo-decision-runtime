# Numerical inputs from one fresh Gooo source

`source.gooo.fixture` declares `Quote`, one operand-order choice with Korean and
English intent, and the fixed `Accepted` predicate. The predicate branches on the
original input and compares the candidate result. Selection and held inputs are
authored in the source; this observation evaluates neither.

The development Gooo CLI exported the source once to [context.json](context.json).
The development SDK prepared the complete source packet and numerical rows once
to [features.json](features.json). These binaries and source identities are
recorded in [observation.json](observation.json); the input source stayed unchanged.

- One choice row and one separate predicate row, each with 256 float32 cells.
- Seventeen predicate nodes, including original-input/candidate-result roles,
  branch conditions, guards, return nodes and the source literals.
- Complete canonical source packet: 2,187 bytes.
- Maximum reserved numeric rows: 14,336 bytes, excluding metadata, source arenas,
  compiled candidates, scratch storage and process memory.
- Nonzero cells: 112 in the choice row and 41 in the predicate row. These counts
  describe this representation; they measure no correctness or completeness.
- Candidate executions, predicate executions, model predictions, training runs
  and local unit tests: zero.

The retained producer is SDK base `8323191cb4c8aa1ebd917a327ae54489fa6c025a`
with the six recorded Go source changes, and compiler source
`74dc01aa1f9842dfbb4530f90f1261cf925e564c` without overlays. Publishing the
implementation leaves this observation attached to those actual producers.

From a checkout containing the development encoder and a Gooo CLI containing the
criteria source-flow export:

```sh
gooo body-context --value-flow --activity Quote source.gooo.fixture > context.json
go run . --compiler-export context.json > features.json
```

`--packet` also accepts a complete canonical scalar source packet. Encoding
requires no tokenizer, network provider or generated source text. The
[feature contract](../../docs/scalar-criteria-features.md) explains its lossy
channels, metadata boundaries and remaining model work. Accuracy, probability
calibration, prediction latency and whole-process RAM remain unobserved.
