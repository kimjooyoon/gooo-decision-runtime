# Find requirements that a decision model cannot distinguish

Before adding training epochs or parameters, check whether the model receives
enough information to distinguish the training requirements.
`contractdecision.AuditInputs(ctx, samples)` groups complete, ordered input
representations and compares the acceptable candidate sets supplied by Gooo.
The local `examples/contract-model fit` command includes its result as
`input_audit` in the training receipt, for each supported model variant.

## A concrete missing signal

Consider two Gooo bodies that both return zero. Each can choose between
`input < 0` and `0 < input`. The first contract requires the condition to be true
for a negative input; the second requires it to be false. Both contracts have
the same final-output cases and the same available operations.

Gooo's condition checks give these contracts opposite acceptable choices.
However, the current initial v6 source features and declared-output case
features are identical for the two contracts. The source identity binds the
conditions, but that identity is not a numerical model input. The regression
in `examples/contract-model/input_audit_test.go` constructs both complete labels
through the ordinary Gooo candidate and condition checks.

Any deterministic first-choice rule using only these identical arrays and
candidate masks must choose the same candidate for both rows. At most one of
the two first choices can pass. Finite Gooo search can still inspect the other
candidate and meet each contract. This separates an input limitation from a
training failure and from eventual construction completeness.

## Read the report

| Field | Meaning |
| --- | --- |
| `samples` | Number of supplied labelled training rows, including duplicates |
| `repeated_input_groups` | Groups containing more than one bit-identical input |
| `conflicting_input_groups` | Groups with no candidate acceptable to every member |
| `best_possible_first_choice_passes` | Upper bound on passing rows for a deterministic rule using these inputs |
| `unavoidable_first_choice_misses` | Rows that must miss their first choice because requirements conflict within identical inputs |
| `groups[].sample_indices` | Zero-based indices into the original sample list; for `fit`, the supplied document order |
| `groups[].candidate_masks` | Original ordered candidate masks |
| `groups[].common_acceptable_candidate_bits` | Intersection of acceptable sets, with bits indexing that candidate list |

For each group, count how many rows accept each candidate. The highest count is
the group's first-choice upper bound; subtract it from the group size to obtain
its unavoidable misses. Sum both counts across groups. For acceptable sets
`{0,1}`, `{1,2}` and `{0,2}`, every pair overlaps but the full group conflicts:
at most two of three first choices can pass. The implementation handles up to
64 complete candidate labels without truncating the highest bit.

The upper bound is about these supplied rows. It is not a measured model score
or a promise that a particular neural architecture can reach it. Different
input arrays can still collapse through pooling or learned weights. A report
with zero input conflicts leaves those failures unmeasured. Report this bound
alongside observed first-choice validity and eventual finite completeness.

## Scope and local cost

The audit includes all source feature cells, all 1–128 case feature rows, their
order, the case ABI and the ordered candidate masks. Each float32 cell is
compared by its bits. The input digest excludes acceptable labels. Hash matches
are checked again against the actual arrays before grouping; the hash alone
does not establish equality. Cases encoding adjacent integers above 2^53 remain
distinct. The audit supports 1–4096 samples and retains compact group counts
instead of a case-feature tensor for the entire dataset.

The audit does no training, prediction or candidate execution. The owning caller
must obtain acceptable sets through its complete compiler checks. Case readers
must stay immutable for the call and may be revisited during exact comparison.
The `fit` command reuses the labels it already collected; its `training_ns`
continues to time fitting, with audit work outside that interval. Conflicting
valid labels remain trainable and are reported as diagnostics.

The [declared-condition input](declared-condition-input.md) now carries source
condition targets, exact integer inputs and expected Boolean values in a
separate versioned channel. Its local inspection command exposes both sides of
the opposite-condition pair. Current contract models and this audit retain
their original input contract, so the diagnosed first-choice limit remains
applicable to those models. Learning from the new channel requires an explicit
new model computation contract and measured construction results.
