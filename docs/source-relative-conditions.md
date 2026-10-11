# Relate declared conditions to the source predicate

The source-relative study retains which Boolean goal belongs to
which side of a source comparison. For `x < 167`, the cases `166 → true` and
`168 → true` express different goals. Averaging their raw integer/Boolean cells
can obscure that relationship. The earlier
[role-learning study](../studies/canonical-role-learning-20261011/README.md)
changed the condition goals in 64 related pairs without changing either model's
first choice. Its logits did change; this does not establish a causal diagnosis
of the learned representation.

`ContractInput.SourceRelativeConditions(choiceID)` now returns exact relation
counts and a separate 16-cell input array. This is an experimental input API.
The study below consumes this version through a separate research artifact.
The current SDK checkout also provides a matching loader and construction
session: [use the relative model](source-relative-runtime.md). Existing model
formats retain their own inputs and ordering.

## Inspect a Gooo-derived plan

From this SDK checkout with Go 1.27.2:

```sh
GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./examples/contract-model inspect \
  -canonical-source -condition-relations path-to-your-plan.json
```

Each `source_choices[].condition_relations` includes availability, reason,
choice ID, plan digest, row count, exact `[3][2]uint16` counts, and features.
The plan digest binds the source and declared conditions used by the prepared
snapshot. The optional flag emits inspection schema v4; existing invocations
keep their earlier schemas and omit the new field.

The inspector makes zero model calls, training updates or candidate executions.
Normal plan preparation still validates and compiles the typed structural
options. The relation reader itself substitutes integer atoms and compares them;
it does not compile another body, execute candidates or decide acceptance.

## What the cells contain

For a fallback predicate `x < 167`, these two suites each contain one `true`
and two `false` expectations, but their relation counts differ:

| Input relation | Goal set A, false / true | Goal set B, false / true |
| --- | --- | --- |
| Less than 167 | 0 / 1 | 1 / 0 |
| Equal to 167 | 1 / 0 | 1 / 0 |
| Greater than 167 | 1 / 0 | 0 / 1 |

The input ABI is `canonical_predicate_condition_relations_v1`:

| Cells | Contents |
| --- | --- |
| 0 | At least one declared condition is present |
| 1–3 | `less_than`, `less_equal`, `equal` one-hot operator |
| 4–5 | Predicate operand-order or branch-layout choice |
| 6–7 | Each option reverses orientation relative to the normalized fallback |
| 8–13 | Less/equal/greater × false/true counts divided by the declared count |
| 14 | Declared condition count divided by the 128-row bound |
| 15 | Reserved zero |

Relations compare the resolved left and right predicate operands. Integer
values stay `int64` through substitution and comparison; no subtraction or
floating-point conversion is used to determine the relation. Reversing the
fallback operands exchanges the less/greater buckets. Equivalent resolved local
assignments keep the same cells. Source names and literal magnitude do not enter
the relation histogram; they remain available in the separate source context.

The array occupies 64 bytes. Context metadata, prepared source storage, stack
scratch and any future model/workspace are additional memory. No process-RAM or
latency measurement is inferred from that array size.

## Support and limits

The existing canonical reader must resolve one root branch and its returns.
The predicate must be integer `less_than`, `less_equal` or `equal`, with known
input/integer atoms. Unsupported shapes return `available: false` with a reason.
Bad or missing choice IDs return an error.

Conditions naming either structural choice of the same `if` share the histogram.
Every declared row is counted. Different input values in the same relation may
carry different goals and both counts remain visible. Existing plan validation
rejects duplicate inputs for one `if`, including aliases of its choice ID.
Zero rows means no supplied condition evidence. A histogram never establishes
that a program passed those goals or that a condition was reached in execution.

A fit must declare this input version and use fresh, frozen training and
evaluation records. The follow-up compares changed-goal response, first and final
finite completion, candidate counts and construction costs with fixed ordering
and a deterministic rule. These features make a narrow source relationship explicit.

## Measured follow-up

The [288-document study](../studies/source-relative-learning-20261011) now compares
this input with the existing representation at the same model size and fitting
budget. Related first-body completion rises from32/128 to64/128. A small
deterministic relation rule also reaches64/128 with fewer candidate checks and
lower observed construction cost. Missing and inconsistent conditions retain
separate completion/probe counts and partial bodies. The study publishes all
weights and records with a separate research artifact schema; current compiler
and runtime defaults are unchanged.
