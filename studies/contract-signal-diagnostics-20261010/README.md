# Goals reach the scores but often keep the same choice

The previous Gooo contract models read authored examples before choosing a
typed code path. Opposite goals still often received the same proposal. This
diagnostic records the internal values of both frozen models to locate where
their responses differ, without another fit or another body search.

There were **392 new diagnostic forward calls**, one per model and source.
Every option score and selected path exactly matched its saved original result.
Each call read every declared case once, including the 128-case contracts.

## What the internal values show

The 194 satisfiable documents form 97 opposite-goal pairs. Each pair has the
same source arrays and different expected outputs. Both models produced
different case summaries, joint hidden values, hidden activations and option
scores for **all 97 pairs**. The source-and-bias prefixes remained identical.

| Observed on the 97 goal pairs | Mean model | Signed pooling model |
| --- | ---: | ---: |
| Different case summary, hidden values and option scores | 97/97 | 97/97 |
| Same selected path despite different goals | 61/97 | 62/97 |
| Both first choices valid | 32/97 | 35/97 |
| Both first choices invalid | 4/97 | 0/97 |
| Hidden coordinates crossing zero between goals | 28/4,656 | 675/4,656 |
| Source+bias prefix larger than the case suffix | 8,819/9,312 | 6,750/9,312 |

The last row counts 24 hidden coordinates × two choices × two documents × 97
pairs. The case suffix is the recorded joint value minus its source+bias
prefix; it includes FP32 rounding. The comparison is about intermediate
magnitudes, with no weighting by downstream importance. It cannot by itself
identify the cause of a semantic error. The crossing row compares corresponding
coordinates across two goals, so its denominator is half as large.

For example, `bound-k13-r0-w1-direct` uses an English description. With signed
pooling, the second choice's option-1 minus option-0 score moves from
**−7.193 to −0.367** between its two goals. The margin changes substantially
but stays negative, so that choice remains option 0. Both choices keep their
original preference and the pair receives the same path. All twelve English
pairs behaved this way at the whole-path level.

These observations show that case information reaches the final scores.
Signed pooling changes more hidden activation boundaries, while the number
of same-proposal pairs remains high. A useful next experiment is to compare a
paired-goal training objective with the existing objective under the same
parameter budget, while retaining the finite-case completeness checks and
reporting regressions. Increasing the number of hidden units is not yet
supported by this diagnostic alone.

The study does not change accuracy: it reproduces the prior selections. All
196 documents, including both contradictions, remain in the trace file. The
paired table covers only the 194 satisfiable documents. See the
[original mean study](../contract-goals-20261010/README.md) and
[pooling comparison](../contract-pooling-20261010/README.md) for actual body
outcomes and the distinction between first selection and eventual completeness.

## Local cost

| Diagnostic measurement | Observed |
| --- | ---: |
| Mean-model call median / p95 | 18.709 / 19.709 µs |
| Signed-model call median / p95 | 16.792 / 18.125 µs |
| Whole process wall / user / system time | 0.50 / 0.11 / 0.02 s |
| Peak process RSS | 32,079,872 bytes (30.59 MiB) |
| Fits / body executions / native processes | 0 / 0 / 0 |

The calls include tracing and copying their results. Models ran in a fixed
order in one process; these times do not establish a speedup over normal
prediction or between pooling methods. Rounded CPU time corresponds to about
0.26 cores averaged over the process. Host utilization increase was not measured.

## Inspect the evidence

- [ExplainInto API](../../docs/contract-signals.md): captures the values from
  the normal computation, with caller-owned arrays and atomic error behavior.
- [Protocol](protocol.txt): fixed before the one diagnostic execution.
  Clean producer: `662389ae86b0606a1354e5e73d33d3df48a8bd32`, Go 1.27.2.
- [Report](result/report.json), [every pair and its margins](result/summary.json),
  [compressed traces](result/traces.jsonl.gz), [checksums](result/SHA256SUMS).
- Both existing model files, source arrays and saved search records are bound
  by exact hashes. The diagnostic producer requires exact old-score parity.
- The [read-only audit](audit) reconstructs the recorded FP32 arithmetic using
  frozen arrays and weights, checks distributions and original rankings, and
  recounts every pair. It makes no prediction API calls and records no new
  measurements. Mutation tests reject changed intermediates, goal bindings and
  an input changed by one above 2^53.

From the SDK root, verify the saved record without rerunning the experiment:

```sh
go run ./studies/contract-signal-diagnostics-20261010/audit \
  studies/contract-signal-diagnostics-20261010/result studies
```
