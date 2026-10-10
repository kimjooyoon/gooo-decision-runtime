# Following a value through a branch

Gooo's small decision model chooses among paths supplied by the source. To make
that choice useful, its input needs to describe what those paths do. A return
that reads a local variable can hide a difference between two bodies:

```text
let result = 10
if candidate_condition {
    result = input
} else {
    result = 10
}
return result
```

Swapping the assignments changes the function. Both arms still end at the same
`return result`, so a summary of that return alone loses the difference.
`PreparedPlan.BranchValueFlow` follows the writes through to the return. It also
records the values reaching the selected branch's predicate operands.

## Static facts

Each value has one of these forms:

- the function input;
- an exact integer literal;
- a Boolean literal;
- an unknown value;
- no returning path or no operand.

A copied local keeps the value it held when copied. Later writes to the original
local do not change that copy. Each selected branch arm is followed through the
remaining body. Other branches are joined: equal values stay known; different
values become unknown. Paths returning before the selected branch are excluded.
Arithmetic expressions remain unknown. The existing source representation's
scope, shadowing and reachability restrictions still apply.

The selected arm is a syntactic assumption. These facts do not prove that an arm
is reachable for a particular input, or that a candidate satisfies the requested
behavior. They use no expected outputs, intent text or model scores. Gooo's
candidate checks and authored cases retain their existing responsibility.

The analysis uses fixed 128-slot state arrays, immutable prepared source and
fresh per-call scratch. A limit of 8,192 statement visits per assumed arm bounds
the traversal. Exhaustion returns `STATIC_VALUE_FLOW_BUDGET_EXCEEDED`.

## Proposed 384-cell input

`ExecutionFlowFeatureVersion` is
`source_intent_condition_output_value_flow_v4`.
`ConditionInput.ExecutionFlowFeaturesInto` exposes the combined array.

| Cells | Meaning |
| --- | --- |
| 0–319 | Existing v3 source, intent, condition and output feedback, unchanged |
| 320–335 | Return after assuming the selected branch's then arm |
| 336–351 | Return after assuming its else arm |
| 352–367 | Predicate left operand, or sole predicate value |
| 368–383 | Predicate right operand, if present |

Each appended slot has eight flags followed by eight big-endian two's-complement
integer bytes. Flags are scaled by 1/8 and bytes by 1/2048. Whole `int64` values
never pass through floating point. An absent slot is all zero; an unknown value
has its own present/kind flags. Non-branch choices have four absent slots.

The array occupies 1,536 bytes, 256 bytes more than v3. This is an input array
size, separate from traversal scratch, model tensors and process memory.
Invalid inputs leave the destination unchanged.

## Evidence and remaining work

The [fixed projection study](../studies/flow-feature-projection-20261010/README.md)
reused 128 original Gooo source snapshots. The old representation contained 72
pairs with identical inputs and disjoint acceptable path sets. This projection
distinguished all 72 pairs without excluding a source. That removes an observed
information loss on this collection; it does not establish a learned accuracy.

Existing model artifacts still require 256 or 320 inputs. The separate
[`flowdecision`](../flowdecision/README.md) package now provides 384-input CPU
training and `NewFlowSession` / `SearchFlowBatches` connect it to candidate
evaluation. It has no published trained weights yet. The next learning study
should compare fresh models on the same fixed split, report first-choice and
finite-search results separately, and include assignment/copy/overwrite cases
outside the training families. It should preserve deterministic continuation
and account for failures and unresolved static values.
