# Source-bound condition model search

`PreparedPlan.NewConditionSession` connects the 6,218-parameter
`conditiondecision.Model` to the existing finite candidate search. One forward
pass scores all 1–16 declared binary choices. Its additive logits rank the
frontier without allocating a probability table for all 65,536 masks.

```go
session, err := prepared.NewConditionSession(ctx, model, cases, "")
// Inspect a non-nil session even after initialization failure: Observe retains
// calls completed before cancellation or numerical failure.
progress, body, err := session.Advance(ctx, 1)
// After a partial, committed batch:
ranking, err := session.Reconsider(ctx, model)
```

Every operation needs a bounded context. A session owns immutable source and case
snapshots and retains one current condition observation. Returned progress is
owned by the caller. Concurrent operations receive `ErrSessionBusy` immediately.
No model pointer is retained; reconsideration checks the same semantic model
fingerprint. This fingerprint differs from the hash of a downloaded JSON file.

## What reaches the model

Initial input contains source structure and the full authored intent. After a
batch, the latest committed, typed candidate supplies its first failing source
condition, including exact int64 input, expected/observed Boolean and candidate
mask. A passing condition observation clears the previous failure. A type
rejection cannot provide a new condition observation. The receipt identifies
which attempt supplied the retained observation.

Feature extraction reuses the candidate's recorded result. It neither executes
the body again nor rewrites intent. Output cases still decide candidate
acceptance, but output failures and CI hints have no channel in this model ABI.

`ConditionRanking` records the exact feature hashes, logits, proposal, actual
calls, prediction time and whether frontier changes were applied. A skipped
call has an explicit reason: unchanged features, one remaining mask, or a
declined source representation. Initial representation decline uses declared
fallback search; numerical prediction errors remain visible errors.

## What the compiler still checks

Each proposed full mask passes the existing typed compiler, source conditions
and finite output cases. An output-correct body with a wrong intermediate
condition is rejected. An accepted result covers the supplied cases; wider
correctness needs further language evidence.

`Advance` commits each mask at most once. Cancellation requeues an unfinished
candidate. `Reconsider` updates a temporary heap and commits it only after its
context check; cancellation preserves the previous frontier while retaining
calls already made. At most 16 feedback rounds are allowed, with a new completed
batch required between rounds. The model can affect order, while the finite
declared space remains reachable.

`SearchConditionBatches` is the convenience API for 1–64 total attempts and up to
16 feedback rounds. With a nil model and zero rounds, it retains ordinary
deterministic search results. The empty seed picks the highest additive score;
an explicit UTF-8 seed up to 512 bytes samples the initial binary factors
reproducibly. Subsequent feedback picks the highest score. These scores and
sampling weights have not been calibrated as correctness probabilities.

The general Gooo CLI still needs an explicit provider for this artifact ABI.
The SDK can already assemble and check source-exported Gooo plans locally.
