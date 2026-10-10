# Learning from executed Gooo choices

The previous model received failures during search, but its 16 training examples
all described the state before execution. Their failure channels were zero.
This experiment adds the states after actually running wrong choices, keeping
the same small network and the same Gooo tasks.

Think of the source as an assembly plan with four permitted configurations.
Gooo supplies the parts, conditions and expected results. The model selects a
configuration in one forward pass. The compiler checks what that configuration
does. Those observations can then become examples for the next model.

## What changed

The initial 16 training arrays remain byte-identical. Each training source has
one acceptable choice among four; the other three were compiled and evaluated
once. These 48 observations supply 32 condition failures and 32 output failures,
with 16 containing both. They are appended to the initial examples, giving 64
training samples. Labels retain the complete source-checked acceptable set.

The 48 observations took 1.106 ms in total. No model was called to collect them.
Only the original direct-source training tasks contributed observations. Other
forms, held-out constants and both controls remained evaluation-only.

## Recorded results

| First choice passes every declared case | Prior initial-only training | Added observed contexts |
| --- | ---: | ---: |
| 16 training sources | 14/16 | 16/16 |
| 64 equivalent source forms | 56/64 | 64/64 |
| 80 sources with held-out constants | 70/80 | 80/80 |
| 2 controls | 1/2 | 2/2 |
| Total | 141/162 | 162/162 |
| Output cases | 1149/1296 | 1296/1296 |
| Declared condition cases | 486/486 | 486/486 |

The read-only comparison found 21 improvements, zero regressions and 141
unchanged successes. All 128 equivalent-form pairs had identical distributions.
Both bounded search modes completed all 162 sources in 162 attempts each; the
previous model needed 183 attempts per mode.

Every new first choice passed, so the feedback-enabled searches made **zero
post-failure model calls**. This result leaves repair quality unmeasured. A next
experiment should start from a declared rejected candidate and evaluate the
next choice on held-out sources, using a separate fixed protocol.

The collection shares task templates across splits. The result measures these
finite cases and leaves unfamiliar tasks, richer bodies, Korean/English wording
coverage and general correctness open. Adding contexts also changes sample
weighting; this one fit cannot isolate the cause of the improvement. The model
remains an explicit experiment, with all previous weights and outcomes retained.

## Local cost

| Measurement | Observed |
| --- | ---: |
| CPU fit, 64 samples, 400 epochs | 514.836 ms |
| First judgment with trace, median / p95 | 14.875 / 16.875 µs |
| Parameters / FP32 weight bytes | 9,290 / 37,160 |
| Model JSON | 108,523 bytes |
| Whole experiment wall / user / system time | 1.06 / 0.67 / 0.03 s |
| Peak process RSS | 22,052,864 bytes (21.03 MiB) |

Rounded process times correspond to about 0.66 CPU cores on average. A host CPU
utilization delta was not collected. There were 162 diagnostic forwards and 324
search forwards, 486 total, plus 48 collection evaluations. Native executions,
GPU work and cloud jobs were all zero. Historical timing comes from separate
runs; the median includes `ExplainInto` trace copying.

## Use and provenance

The artifact uses the public SDK35 feature v6 and explicit schema v2 with
`leaky_relu_0.01_v1`. Load [model-feedback_v6.json](result/model-feedback_v6.json)
with `flowdecision.Decode`, then pass it to `PreparedPlan.SearchFlowBatches`.
Passing a nil model retains deterministic search. Compiler
[PR1443](https://github.com/kimjooyoon/meta-ontology-go/pull/1443) connects this
artifact contract to the CLI; installed compiler 0.6.25 still uses SDK28.

- Fixed [protocol](protocol.txt), committed before any collection or fitting;
  clean Go 1.27.2 producer `d9986e7bab77c8b36455e80a0ffd135a949da001`.
- [Report](result/report.json), [audit](result/audit.json),
  [original leaky baseline](result/baseline-report.json),
  [artifact manifest](result/SHA256SUMS).
- Model SHA256: `b81317ce44943549c58237eb9b87e797eca56c181a24086780f7f08a13d4ad59`.
- The bundle retains all 64 training arrays, 48 exact observations, 400 losses,
  162 first judgments and 324 search records. Integers remain exact `int64`,
  including values beyond 2^53.
- The auditor compares recorded failures with the original complete candidate
  table and reconstructs only feature channels. It performs zero model calls,
  candidate executions or fitting. Tests reject altered labels, feedback bytes,
  premature feedback and coordinated one-unit changes to large output values.

One fit was run once. Compression uses gzip-n and decompressed byte comparison.
