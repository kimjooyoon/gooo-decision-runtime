# Source constants as context for a small decision model

`source_literal_integer_case_v1` is an explicit alternative input representation
for declared Gooo examples. It describes how an input and its expected result
relate to the integer literals already present in the prepared source arena.
No candidate body is evaluated to obtain these features.

For example, if the source contains the literal 7, an expected result equal to
`input + 7` produces a direct relation flag. The model can learn from that flag
alongside source structure. Gooo still checks the complete authored suite after
choosing a path.

## Fixed size and exact integers

There are still 32 cells per declared case and 9,746 model weights (38,984 FP32
bytes). Cells12–27 retain every input and expected-value byte, and cells7–9 keep
expected equals/below/above input. Twelve other cells replace the original
sign/parity/negation/successor hints:

| Cell | Relation to sorted unique source literals `k` |
| --- | --- |
| 1 | At least one literal exists |
| 2,3,4 | Input equals / below / above `k` |
| 5,6,10 | Expected equals / below / above `k` |
| 11 | Expected equals `input + k` |
| 28 | Expected equals `input - k` |
| 29 | Expected equals `k - input` |
| 30 | Expected equals `-k` |
| 31 | Expected equals `input * k` |

Presence uses1/8; each relation uses its matching fraction of distinct literals
times1/8. A plan can contain up to128 expressions; every distinct Int literal is
included, also in branches that have not executed. Repeated literal values count
once. Empty literal sets leave the new cells zero. Overflowing integer arithmetic
never counts as an equality. Source integers are never converted wholesale to
floating point.

The optional input owns up to1,024 bytes of literal storage, alongside the
existing2,048-byte declared-case payload. Model weights and 128-byte row scratch
stay fixed. Runtime cost includes these extra integer relations; no speedup is
assumed. The input representation remains lossy after learned pooling.

## Use the explicit profile

```sh
go run ./examples/contract-model fit \
  -case-features source_literal_integer_case_v1 -pooling signed_max_abs \
  -out source-relative.json training-a.json training-b.json
go run ./examples/contract-model search -model source-relative.json evaluation.json
```

Fit labels still come from complete candidate output/condition checks. Search
reads the case profile from the model and projects the matching source-bound
cases before its single initial prediction. It immediately continues finite
construction. The local fit receipt is v3 for this option.

The Go APIs are `PreparedPlan.InitialContractInputFor`,
`ContractInput.SourceLiteralsInto`, `contractdecision.NewForCaseFeatures` and
`FitForCaseFeatures`. A model rejects a case reader carrying a different version;
unversioned readers retain the old declared-case v1 convention. New artifacts
use `gooo/contract-candidate-decision/v3`; old v1/v2 bytes and fingerprints stay
unchanged. Training with the paired-goal option currently uses the old case ABI.
The installed ordinary compiler does not load the new v3 artifact yet.

Tests exercise exact arithmetic against a big-integer reference, all128
literals/cases, source ownership, concurrent readers, atomic version errors,
model serialization and immediate source-version-aware construction with large
integers. The [completed one-fit study](../studies/source-literal-contract-20261010/README.md)
compares the original training rows against saved unpaired results: first-path
validity fell from132/196 to109/196, with23 regressions and no improvements.
The profile remains opt-in; it is not a replacement for the published model.
All194 satisfiable documents still completed through finite validation, and both
contradictory goals remained incomplete.
