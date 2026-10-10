# Source-relative cases: a smaller signal is not automatically a better signal

2026-10-10. One fresh CPU fit replaced 12 declared-case cells with relations to
integer literals in the source. For example, `expected == input + k` can be
represented directly when `k` is an authored literal. Every exact input and
expected-value byte remains available. No candidate execution supplies these
features. [Representation and API](../../docs/source-literal-cases.md).

The [protocol](protocol.txt) and producer were committed before execution at
`79f871f67e6e52d55b03235e5e42773541e68809`. The architecture stayed at 9,746 FP32
parameters / 38,984 weight bytes, 32 cells per case and eight pooled values.
The original 24 training documents, labels, source arrays, order, 600 epochs,
learning rate, seed and ordinary candidate objective stayed fixed. This fit
uses no paired-goal objective.

## Measured result

Comparison uses the saved [unpaired signed-pooling study](../contract-pooling-20261010/README.md).
Neither that model nor the original compiler/oracle collection was rerun.

| Measure | Saved declared-case model | Source-literal case model |
| --- | ---: | ---: |
| First valid path, all 196 documents | 132 | 109 |
| First valid path, training 24 | 20 | 14 |
| First valid path, satisfiable outside training 170 | 112 | 95 |
| First valid path, representation variants 48 | 32 | 28 |
| First valid path, new constants 72 | 52 | 42 |
| First valid path, English wording 24 | 12 | 12 |
| First valid path, unseen family 24 | 14 | 12 |
| First valid path, 128-case rare tails 2 | 2 | 1 |
| Same proposal for opposite goals, 97 pairs | 62 | 85 |
| Both opposite goals valid on first choice | 35 | 12 |
| First-body outputs passed, out of 2,048 | 1,582 | 1,417 |
| First-body conditions passed, out of 588 | 588 | 588 |
| Eventual finite completion, satisfiable 194 | 194 | 194 |
| Contradictions incorrectly accepted, out of 2 | 0 | 0 |
| Total candidate attempts, including contradictions | 264 | 287 |

There were **0 improvements, 23 regressions and 173 unchanged first choices**.
All regression identifiers and split counts are in [audit.json](result/audit.json).
The finite constructor recovered from incorrect choices while retaining the
authored output and condition requirements. Its eventual success is separate
from the quality of the model's initial hint.

The result does not support adopting this representation as the default.
It replaced sign/parity/successor information as well as adding literal
relations; the comparison cannot attribute the regression to one of those
changes. A low final training loss also did not imply high path validity.
These are known-corpus measurements, not an estimate of accuracy on new natural
language requests. The published default model remains unchanged.

## Local cost

- One fit: **338,539,209 ns**, 14,400 training forwards.
- 196 new sessions and immediate predictions; zero post-failure predictions.
- Prediction median **17,167 ns**, p95 **20,625 ns**.
- Whole producer: **0.93 s wall**, **0.47 s user + 0.02 s system**.
- Maximum RSS: **31,408,128 bytes / 29.95 MiB**. Rounded process times imply
  about 0.53 CPU core on average. Host CPU utilization increase was not measured.
- No GPU, external model, native subprocess or extra label collection.

The process includes preparation, projection, training, finite construction and
record writing. Separate old/new process timings do not establish an isolated
inference speed change. The JSON model is 114,007 bytes; its SHA-256 is
`aecf0b8f0ad570d4ab3019c14305d7a9c1d3c716f16c6c2f915b64515ef9afa9`.
The v3 fingerprint is
`0a3b35e90e878beab8f1fb7cb20bd3caf75831e27d3e0216f65ad33c44588153`.

## Evidence and next design question

The result directory retains the source-literal set and projected cases for all
196 documents, the original and consumed training arrays, all 600 losses, every
search step and failed output, model bytes, producer identity and process cost.
The auditor independently reconstructs literal relations with big-integer
arithmetic and compares exact int64 oracle outcomes and ranking/progress chains.
It never fits, predicts, lowers source or executes a body.

```sh
go run ./studies/source-literal-contract-20261010/audit \
  studies/source-literal-contract-20261010/result \
  studies/contract-goals-20261010/result \
  studies/contract-pooling-20261010/result
```

Mutation tests cover source literals, relation cells, retained integer bytes,
the 128th case, training labels/order/source arrays, goal edits and coordinated
one-unit edits to positive/negative outputs beyond 2^53. Original observations
are retained; a repaired auditor cannot replace them.

The next useful question is where a goal should meet a particular choice. This
model pools all cases before scoring each source choice. Literal relations add
context, but still omit which branch or expression uses a literal. A bounded
choice-to-case representation could retain that association while Gooo owns
the available operations and validates complete candidates. A future study
should preserve the old case channels, distinguish adding information from
replacing it, and use new evaluation contracts fixed before fitting. This is a
design direction, not an implemented or measured improvement.
