# Inspect equivalent branch inputs in retained Gooo records

This is a source projection and label-consistency audit. It reads the saved
plans from the [broader model study](../multifamily-requirement-learning-20261011/README.md).
It performs **zero model fits, predictions and candidate executions**. Original
weights, source documents, labels and observed outcomes remain unchanged.

The broader study's eleven in-family first-choice failures were assignment
bodies. Inspecting those records showed that equivalent ordered expressions
could still be accompanied by different syntax counts and return-role fields.
The new [canonical input](../../docs/canonical-branch-input.md) removes that
presentation difference for its supported branch shapes.

## Recorded result

| Family | Direct/assignment pairs | Earlier complete source input equal | Canonical input equal |
| --- | ---: | ---: | ---: |
| Bound | 32 | 32 | 32 |
| Distance | 32 | 0 | 32 |
| Negative bound | 32 | 0 | 32 |
| Total | 96 | 32 | 96 |

All 256 retained documents yield 512 per-choice projections. The audit checks
that every old 528-cell array still matches its saved value. Paired output cases,
condition cases and acceptable masks also remain identical. The complete-input
audit reports 96 repeated groups and zero groups with conflicting labels.

`best_possible_first_choice_passes = 256` is an information bound from the
stored labels. It means identical inputs do not force conflicting decisions
within this corpus. It is not a measured model score, proof of semantic
equivalence for arbitrary programs, or evidence that training will reach 256.
The corpus consists of related templates, not 256 independent business tasks.

The [result](result.json) includes every paired document ID and input group.
The source reader requires the exact decompressed record SHA256
`93fa5436660ef4c5793cf0b0c5cde10f0520e6f236753a607a03227a2f627c10`.
This audit creates no duplicated source corpus or model weights.

## Reproduce the inspection

From the SDK repository root:

```sh
GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./studies/canonical-input-inspection-20261011
GOWORK=off GOTOOLCHAIN=go1.27.2 go test ./studies/canonical-input-inspection-20261011
```

The first command emits the source-only report. The regression compares it with
the saved report. Neither command invokes the historical experiment producer.
Future training on this representation needs new, separately fixed evaluation
evidence; the earlier held-out results have already informed this change.
