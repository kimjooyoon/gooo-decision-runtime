# Cached probe continuation, 2026-10-03

Local darwin/arm64, Apple M4, Go 1.27.1. This is a four-candidate conditional
integer fixture. The initial case is `0 → 0`, with supplied probe inputs
`[-1,1,0]`. Appending the declared oracle observation `-1 → 1` filters candidates.
Initial preparation/evaluation is excluded in both arms. Each cached iteration
includes copying the bounded session; the fresh arm enumerates candidates again.
No model calls or training occur.

Three repetitions of 1,000 operations produced:

| Operation | ns/op, each repetition | B/op | allocs/op |
| --- | --- | --- | --- |
| Fresh ranking | 72,304 / 59,001 / 58,750 | 78,724 / 78,688 / 78,724 | 1,441 |
| Cached append | 1,245 / 1,215 / 1,209 | 993 / 993 / 993 | 20 |

The [raw output](probe-session-20261003.txt) preserves every reported sample.
These are continuation kernel measurements on one fixture. End-to-end codegen,
process RSS and host-wide CPU utilization were not measured in this comparison.
The 16 KiB retained output matrix is separate from allocation bytes per operation.

The paired semantic test appends `-1 → 1` and then `1 → 1`, comparing each cached
snapshot with a fresh rank over all accumulated cases. Candidate sets, output
partitions, recommendations and hashes agree. Initial evaluation count is 16;
both continuations do zero new evaluations and compare four then two cached rows.
The constructor and first observation still incur their original costs.

```sh
go test ./pathplan -run ProbeSession \
  -bench BenchmarkProbeObservationContinuation -benchtime=1000x -count=3
```

Only small owned case/probe slices are passed to JSON encoding. Passing slices
directly into the complete session made its output matrix escape into a heap
allocation on every copied continuation. Keeping those slices separate reduces
that allocation while retaining the fixed matrix and transactional update.

This API is a development change. Compiler integration must continue to bind
the original Gooo source, declared oracle and append-only observations, then
perform its native emission/replay checks. Reusing finite outputs does not prove
behavior beyond those inputs.
