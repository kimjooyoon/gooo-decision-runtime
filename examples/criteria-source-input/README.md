# A source-only scalar requirement packet

The SDK inspector prepared the retained Gooo source-flow export linked in
[observation.json](observation.json), once. The [result](inspection.json) retains
one candidate choice, one fixed criterion, fourteen graph nodes and the distinct
original-input/candidate-result roles. The literal pool holds the source spelling
`11`, using two bytes. The canonical packet occupies 1,838 bytes.

The observed node layout is 20 bytes. The reserved 4,096-node arena occupies
81,920 bytes and its separate literal pool reserves 65,536 bytes. These explicit
storage figures exclude Go, parsed JSON and compiled candidate storage.

This reads a saved compiler JSON carrier and validates/compiles the source plan.
It makes zero model calls, candidate/predicate evaluations or training runs.
It replays no compiler source experiment. The saved producer is the actual
SDK head and new source overlays, before their future publication. Numerical
features and prediction weights remain unavailable.

See the [input contract](../../docs/scalar-criteria-source-input.md).
