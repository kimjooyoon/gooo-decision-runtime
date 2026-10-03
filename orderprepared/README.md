# Prepared whole-candidate search

This additive package retains the eight compiled bodies of the existing
two-update profile. It preserves the original `orderjudge` and `orderfacts`
source inventory. It uses the same weights, feature functions and arithmetic.

```go
runtime, err := orderprepared.NewRuntime(model) // nil selects deterministic order
// Check err, then prepare under a context with a deadline.
prepared, err := runtime.Prepare(ctx, sourceBoundPlan)
// Check err. Reuse only for that same source-bound plan.
search, program, receipt, err := prepared.Search(
    ctx, currentSourceBoundPlanSHA, finiteCases, 8, true,
)
```

The compiler must bind the current source to its typed plan before passing the
plan digest. A mismatch fails before prediction. A runtime captures a private
snapshot of final weights and hashes it once. Copying the Runtime value shares
that private snapshot; overwriting the caller's original model cannot change it.
Each plan preparation validates and compiles all eight complete combinations.
Its fixed arrays hold 128 intent features, 8×32 source features, eight 48-byte
descriptors and candidate references. Program arenas, rendered sources and model
storage consume additional memory.

Each search performs fresh inference with a caller-local workspace and evaluates
this call's cases. Predictions and chosen answers are not cached. All maps,
receipts and returned Program wrappers belong to the caller. Internal Program
arenas are private and immutable. No global cache or background worker is used.
The caller owns the lifetime and number of prepared plans.

## Measurement protocol

`examples/order-prepared` consumes the unchanged, digest-bound initial fit bundle
and selects the 64 previously observed new-template/new-constants requests.
The weights digest is pinned. Additional training and native executions are zero.

For budgets 1 and 8, both deterministic and model arms run three paired modes:
original Search, new runtime + preparation + Search per call, and retained Search.
Five rounds alternate mode order. The default collects 3,840 actual searches
and 1,920 predictions. Complete semantic records must match with only the named
prediction timer excluded. All budget-8 bodies also meet all eight finite
evaluation inputs. The original observed cohort is a development comparison.

Per-operation time and process allocation counters, one-time runtime capture and
preparation costs are recorded separately. Counter reads and JSON serialization
are outside the wall interval; they can still perturb the process. Three live
heap observations per arm retain 64 prepared plans and one runtime, including
private program objects. All raw deltas are retained, including runtime noise.

```sh
go build -trimpath -o /tmp/order-prepared ./examples/order-prepared
/tmp/order-prepared --input /tmp/order-initial --output /tmp/order-prepared-results
```

The CI replay uses one round (768 searches, 384 predictions) and retains all
records. CI timing is a separate machine observation. Compiler integration and
whole-command latency must be measured separately before claiming a codegen gain.
