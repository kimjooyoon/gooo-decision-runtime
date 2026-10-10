# Static value-flow projection — 2026-10-10

This study asks whether a small decision model can receive the distinction
between assigning a value and merely naming the local that eventually returns
it. The producer was committed before the one recorded projection. The
[protocol](protocol.txt), full arrays, source identities and measurements are
public in [result](result).

## Result

The input is the original 128-source collection from the
[output-feedback study](../execution-feedback-learning-20261010/README.md):
32 training, 32 wording, 32 changed-constant, 16 assignment and 16 four-region
sources. Only its 128 saved initial contexts are projected. No original fit,
candidate test or model prediction was repeated.

| Measurement | Recorded value |
| --- | ---: |
| Source plans prepared | 128 |
| Sources declined or excluded | 0 |
| Identical-input pairs with disjoint acceptable path sets, v3 | 72 |
| Such pairs with the additional static facts | 0 |
| Old collision pairs involving a declined source | 0 |
| New fits / model calls / candidate tests | 0 / 0 / 0 |

The old 72 pairs comprise 16 assignment/assignment, 16 four-region/four-region,
16 changed-constant/training, 8 changed-constant/changed-constant, 8 training/training
and 8 wording/wording pairs. Counts are pairs, not 72 unique programs or
independent tasks. Acceptable path sets come from the frozen earlier evaluations.
They are used only when counting collisions, never in the added static facts.

The first 320 values in every input match the original v3 arrays. Four appended
16-cell slots describe the then/else return values and the predicate operands.
The analysis follows assignment snapshots and joins other branches. Exact
integers remain exact, and unresolved expressions have an explicit unknown form.
See the [API and boundaries](../../docs/branch-value-flow.md).

This result addresses one measured representation bottleneck. It does not
measure a model's ability to learn the new representation, or prove unique
representations for arbitrary Gooo programs. All sources are authored variants
of a small number of related tasks. Wider functions, arithmetic value relations
and general Korean/English intent understanding remain open work.

## Local process observations

The complete projection process recorded 0.56 seconds wall time, 0.11 seconds
user CPU and 0.01 seconds system CPU. Peak resident memory was 19,103,744 bytes
(about 18.22 MiB). These include reading and decompressing the original records,
preparing plans, extracting facts and serializing arrays. Plan preparation
includes fallback compilation and validation. No candidate cases were executed.

The CPU times correspond to about 0.21 CPU cores averaged across this short
process. They do not measure the change in whole-computer CPU utilization or
isolate the cost of the static analysis. This single run is a process observation;
it is not a controlled performance comparison with the original model.

## Provenance and read-only audit

Producer revision: `ac4519b8b2dd50d7e155aa8485783e77b13412de`, built from a clean
tree with Go 1.27.2 on macOS arm64. The report records the SHA-256 of both original
compressed input files, every Gooo source, and the prepared source plan. The
compressed report is byte-identical to the original when decompressed. Its
binary digest and build-info excerpt are included; the excerpt omits only the
local executable path line.

From the repository root, inspect the saved evidence with:

```sh
(cd studies/flow-feature-projection-20261010/result && shasum -a 256 -c SHA256SUMS)
GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./studies/flow-feature-projection-20261010/audit \
  studies/flow-feature-projection-20261010/result \
  studies/execution-feedback-learning-20261010/result
```

The audit reads the frozen arrays and labels, checks original source/plan hashes,
independently decodes the added integer bytes, compares all v3 prefixes and
recounts pair collisions. It performs no plan preparation, fitting, inference
or candidate execution. It does not independently re-derive the static facts
from source; the committed producer and semantic regression tests cover that step.

The measurement entry point requires a fresh output directory and a clean
compiled producer. The recorded run is complete. Further model training must
use a new declared protocol and preserve these observations.
