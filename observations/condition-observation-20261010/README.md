# Actual selected Gooo bodies: condition observation

The SDK API producer is `503659bcd8f28ee4b2cbc2e350323e54640e4d99`.
The source program, context and model selections come from workbench commit
[`b63ea6d74603a89e1a74af2b3dc36c1596742935`](https://github.com/kimjooyoon/gooo-ecosystem-workbench/tree/b63ea6d74603a89e1a74af2b3dc36c1596742935).

This observation reuses the earlier posthoc predicate probes: -1 should be true,
0 and 1 false for “input is negative.” All three stored programs actually use
`0 < input`. Each therefore matches **1/3** intermediate expectations. Their
published final-output results remain 6/11, 6/11 and 11/11 for the previous,
control and joint models. Those 11 inputs and these three predicate probes have
different scopes and denominators.

`results.json` records every Boolean observation, exact integer, selected choice,
source/plan/document identity and consumed archive hash. The audit checks the
selected Gooo body against the recompiled typed plan and compares each condition
with the original Go AST audit, including the original generated-Go hashes.
There are zero new model calls, no training and no new native generation.
This observation does not make existing compiler search enforce these predicates.

With the matching workbench files already available locally:

```sh
GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./observations/condition-observation-20261010 /path/to/gooo-ecosystem-workbench > observed.json
cmp observations/condition-observation-20261010/results.json observed.json
```

`main.go` is the formatted reproducible audit; `original-audit.go.gz` preserves
the exact initial runner. The original successful local race, vet and benchmark
logs are retained. A simple body-plan condition observation measured **82.52
ns/op, 0 B/op, 0 heap allocations/op** on this Apple M4 run. That microbenchmark
excludes compilation, model prediction, native process execution and JSON.
It is a single local observation, not an end-to-end speed or CPU-utilization claim.
