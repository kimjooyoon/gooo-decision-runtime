# Learning with branch return roles

This study compares two fresh local decision models trained on the same Gooo
source contracts. Their feature representations differ: v2 adds branch return
roles to twenty reserved cells. The full
[protocol](protocol.txt) fixes source groups, training settings and observations
before fitting either model. Producer `b3360fb08c3f72bef003821fe4543de484c70664`
completed the original export and run once on macOS arm64 with Go1.27.2.

The exporter lowers actual Gooo source using compiler commit `1ceb96b9` and the
published SDK0.2.31. The training program consumes the exact exported documents,
derives acceptable complete masks from typed execution, and fits only the forty
training contexts. Each model prediction is immediately followed by assembly and
checking of its chosen body. Five finite-search modes then use frozen weights.

The sixty programs are variants of max/min, clamp and sign-code tasks. New
wording, local-return structures, nested clamps and remaining literal-value
collisions are reported separately. These small groups do not establish general
language accuracy. Source test cases remain visible during finite search.

## What happened

V2 distinguished the training branch layouts and reached 8/8 first judgments.
Held-out wording fell from 8/16 to 6/16, while local-return and nested-clamp
results stayed the same. Across the 52 non-training sources, v1 scored 24/52 and
v2 scored 22/52. Learning the new channel on these eight training sources did not
improve those evaluation groups.

| Source group | Programs | V1 first judgment | V2 first judgment | V2 first-judgment output cases |
| --- | ---: | ---: | ---: | ---: |
| Training max/min | 8 | 4/8 | 8/8 | 64/64 |
| New Korean/English wording | 16 | 8/16 | 6/16 | 58/128 |
| Local-return structure | 8 | 4/8 | 4/8 | 36/64 |
| New wording and local structure | 16 | 8/16 | 8/16 | 72/128 |
| Nested clamp challenge | 8 | 2/8 | 2/8 | 32/88 |
| Literal-value collision challenge | 4 | 2/4 | 2/4 | 14/28 |

A correct judgment satisfies every authored output and condition case. The last
column retains partial functional results when a proposed body fails some cases.
Nested clamp and sign sources declare zero condition cases; their output cases
supply the finite check. Initial/observed contexts of one source are correlated.
All 584 judgments and their immediate selected bodies/cases are retained in
`result/judgments.jsonl.gz`; per-split observed-context counts are in the report.

Initial input pairs with disjoint valid mask sets fell from 74 for v1 to 2 for
v2. The remaining pairs swap two literal return values: that value information
is still absent. Pair counts describe these particular source variants.

## Finite completion and feedback

| Search mode | Completed sources | Total candidate attempts | Total model calls | Additional feedback calls | Total search time |
| --- | ---: | ---: | ---: | ---: | ---: |
| Deterministic | 60/60 | 122 | 0 | 0 | 6.113 ms |
| V1 initial ranking | 60/60 | 98 | 60 | 0 | 6.436 ms |
| V1 condition feedback | 60/60 | 98 | 60 | 0 | 7.279 ms |
| V2 initial ranking | 60/60 | 96 | 60 | 0 | 6.005 ms |
| V2 condition feedback | 60/60 | 96 | 60 | 0 | 7.718 ms |

Each source permits only two or four combinations, and search sees its declared
cases. All 300 searches completed within that finite space. Timings are totals
from one small run; they include different bookkeeping and are not a speed claim.

There were 230 judgments where all declared condition checks passed but output
cases failed. The frozen models kept the comparison choice correct during search,
so output failures supplied no new condition observation. Feedback reused 34 v1
and 32 v2 rankings and made zero additional neural calls. This exposes a concrete
next input requirement: preserve an output mismatch's exact input, expected value
and actual value, alongside the separate intermediate-condition observation.

Further work should teach local/nested structures with separate held-out sources
and include literal-value roles. Those changes need a new protocol and new fits;
the current settings and results were kept after seeing these failures.

## Local resources and model files

| Measurement | V1 | V2 |
| --- | ---: | ---: |
| Fixed 400-epoch CPU fit | 158.960 ms | 159.924 ms |
| Prediction median / p95 | 7.708 / 8.791 µs | 7.750 / 8.000 µs |
| JSON artifact | 72,515 bytes | 72,379 bytes |
| FP32 weight values | 24,872 bytes | 24,872 bytes |

The combined fit/judgment/search command took 0.84 s wall time, 0.46 s user CPU
and 0.02 s system CPU, with maximum RSS 17,874,944 bytes. This is approximately
0.57 CPU cores averaged across that command; whole-machine utilization change
was not measured. The separate compiler-linked export took 0.98 s wall time and
maximum RSS 23,494,656 bytes. Native executable runs: zero.

- [Newly fitted v1 comparison model](result/model-v1.json), SHA256
  `dd3fec403f2c69718c6095dfaaa97a7b1b59e2ec21238e3f237a2f1c48a674cd`.
- [Newly fitted v2 model](result/model-v2.json), SHA256
  `9dea974459cfd5ff6b5f0496fc045127b1933577426047e88b4fe00c65740f7a`.

Both were trained from scratch in this paired study. The SDK0.2.29 published
absolute-value model remains a separate historical baseline. General compiler
CLI consumption of the new v2 artifact still needs the matching SDK dispatch.

## Inspect the saved evidence

```sh
(cd studies/branch-role-learning-20261010/result && shasum -a 256 -c SHA256SUMS)
GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./studies/branch-role-learning-20261010/audit \
  ./studies/branch-role-learning-20261010/result
```

The audit verifies source and model identities, 232 complete candidates, exact
large integer cases, acceptable sets, 292 feature records, training-only sample
hashes, all 584 selected results, progress/ranking digests and 300 search records.
It recounts the saved observations with zero new fits, predictions or program
executions. This verifies record consistency; the original committed producer
supplies the execution evidence.

Compressed files are deterministic gzip copies of the original bytes. Build
excerpts omit only their first line containing the local executable path; the
original build-record digests are retained. The report, model artifacts, resource
logs and start/completion records are copied unchanged. No private machine path
is needed to read the public results.

## Run a new experiment

Use clean committed sources, Go1.27.2 and fresh output directories. The nested
exporter module expects a sibling compiler checkout at its pinned revision.

```sh
# From exporter/, build the compiler-linked source exporter.
GOWORK=off GOTOOLCHAIN=go1.27.2 go build -trimpath -o /tmp/my-branch-export .
# From the SDK root, build the paired training/search producer.
GOWORK=off GOTOOLCHAIN=go1.27.2 go build -trimpath -o /tmp/my-branch-fit ./examples/branch-role-fit

/usr/bin/time -l /tmp/my-branch-export /path/to/meta-ontology-go /tmp/my-new-branch-sources
/usr/bin/time -l /tmp/my-branch-fit /tmp/my-new-branch-sources/dataset.json /tmp/my-new-branch-results
```

These commands make new observations. Preserve partial output if a command
fails; use a new destination for a separately declared experiment. The producer
refuses an existing output directory or mismatched source revisions. Exporting
does no training or model inference. The run contains two fixed fits, 584 judged
contexts and 300 bounded searches. Program/process RAM is measured separately
from the model's 24,872 weight bytes.
