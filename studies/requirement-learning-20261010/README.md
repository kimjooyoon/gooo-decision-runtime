# Output goals and intermediate conditions: 172 Gooo documents

This study asks whether a small local model can use both the required outputs
and the required intermediate comparisons to choose a Gooo body. Two contracts
can require the same final values while requiring different internal paths.
Gooo supplies the alternatives and checks each complete candidate.

We froze 172 authored documents and trained on 32. Both models then searched
all documents, alongside deterministic ordering. The requirement model's first
candidate passed **44/136 satisfiable held-out documents**, compared with
33/136 for the choice model and 35/136 for deterministic ordering. All three
methods eventually completed those 136 documents. Four contradictory documents
remained incomplete.

Learning was weak in this run. The requirement model passed only 8/32 training
documents on its first choice, and its loss stayed near the uniform four-path
baseline. Across all documents it saved one candidate attempt. These weights
are a recorded experimental result; the existing defaults remain available.

## What was fixed before fitting

[The protocol](protocol.txt) specifies source templates, cohort boundaries,
exact integer examples, model settings and all reported outcomes. Both models
use the same 32 training documents, arithmetic-mean pooling, 2,000 epochs,
learning rate 0.3, L2 0.0001 and seed 17. Each model was fitted once. Evaluation
did not select a seed, epoch or a revised fit.

The source choices are comparison operand order and branch layout. Output goals
and condition goals vary independently. Each document has eight output cases,
two binary choices and four complete candidates. Gooo enumerates all four to
label the acceptable set before training. Exact int64 examples include values
larger than 2^53.

| Cohort | Documents | Deterministic first pass | Choice first pass | Requirement first pass | Eventually complete, each method |
| --- | ---: | ---: | ---: | ---: | ---: |
| Training | 32 | 8 | 7 | 8 | 32 |
| New constants | 32 | 8 | 7 | 8 | 32 |
| Aliases, assignment, rebinding | 48 | 12 | 9 | 18 | 48 |
| English and fresh Korean wording | 32 | 8 | 10 | 10 | 32 |
| New negative-bound family | 8 | 2 | 2 | 2 | 8 |
| Reordered choice declarations | 8 | 2 | 2 | 3 | 8 |
| Decisive 128th condition | 4 | 1 | 1 | 1 | 4 |
| Empty condition suite | 4 | 2 | 2 | 2 | 4 |
| Contradictory conditions | 4 | 0 | 0 | 0 | 0 |

The rare-condition source has a fixed prefix that maps 127 distinct inputs to
an ambiguous equality comparison. Its final negative input distinguishes the
two comparison directions. That cohort also introduces a fixed branch, so its
result combines source structure and rare-condition effects. These are related
template families; the denominators describe their declared finite cases.

## First decisions and completed bodies

| All 172 documents | Deterministic | Choice | Requirements |
| --- | ---: | ---: | ---: |
| First candidate satisfies every output and condition | 43 | 40 | 52 |
| Eventually complete | 168 | 168 | 168 |
| Candidate attempts | 432 | 432 | 431 |
| First-candidate outputs passed / checked | 772/1,376 | 730/1,376 | 898/1,376 |
| First-candidate conditions passed / checked | 840/1,008 | 840/1,008 | 839/1,008 |
| Model predictions | 0 | 172 | 172 |
| Later predictions | 0 | 0 | 0 |
| Total recorded search time | 20.407ms | 37.482ms | 36.223ms |

Relative to the choice model, the requirement model improved the first result
in 38 documents and regressed in 26. All regression IDs are in
[the independent audit](result/audit.json). The added prediction time still
makes these small searches slower than the deterministic route.

There are 82 satisfiable pairs with different condition goals and two control
pairs with empty condition suites. The new model changed its scores in 69 of
the 84 pairs, but proposed different paths in only one pair. No pair with
opposite condition goals had both first candidates valid. Finite Gooo checking
preserved their final completeness despite those poor initial choices.

## Information still missing from the model input

On the 168 satisfiable documents, the full-array input audit finds a first-choice
information bound of 66/168 for source/output inputs and 128/168 when declared
conditions are included. These bounds describe indistinguishable inputs for a
deterministic decision rule; learned performance is measured separately above.

Forty conflicting groups remain in the complete three-channel representation:
36 pairs in the distance family and four in the negative-bound family. Each pair
has bit-identical source, output and condition feature arrays, but disjoint
acceptable paths. For example, swapping these returned arithmetic expressions
changes the required branch selection:

```text
if input < 6 { return input - 6 } else { return 6 - input }
if input < 6 { return 6 - input } else { return input - 6 }
```

The Gooo source and typed plan retain this difference. The current model feature
projection loses it. [The audit](result/audit.json) records both bodies, required
masks and identical input hashes for every conflicting pair. A next input
revision needs to preserve ordered operands and their return-branch relations.
The low training score also calls for examining how the learned summary uses
the association between each condition input and its expected Boolean value.

## Local cost and model files

| Measurement | Choice | Requirements |
| --- | ---: | ---: |
| FP32 parameters | 12,818 | 13,282 |
| Weight-array bytes | 51,272 | 53,128 |
| JSON artifact bytes | 150,297 | 155,816 |
| CPU fit wall time | 5.913s | 2.449s |
| Prediction median / p95 | 36.875 / 44.041µs | 25.625 / 34.500µs |
| Initial → final training loss | 1.390177 → 1.386296 | 1.387552 → 1.386285 |

The uniform four-path loss is about 1.386294. The small loss change and chance-level
training result limit what can be attributed to learning. This comparison also
changes the architecture and its initialization, so it does not isolate a causal
effect of condition input alone.

The completed producer process took 9.28s wall time, 8.65s user CPU and 0.16s
system CPU, with maximum RSS 51,429,376 bytes (49.05MiB). That averages about
0.95 CPU cores during the command. Whole-PC utilization was not measured. These
figures cover both fits, input collection and all searches in one process.
The study used Go's typed evaluator; native generated Go execution was zero.

From this SDK checkout, inspect or use the recorded requirement artifact:

```sh
go run ./examples/contract-model inspect \
  studies/requirement-learning-20261010/examples/req-bound-k6-r1-w0-o0-alias-g0-c0.json

go run ./examples/contract-model search \
  -model studies/requirement-learning-20261010/result/model-requirements.json \
  studies/requirement-learning-20261010/examples/req-bound-k6-r1-w0-o0-alias-g0-c0.json

go run ./examples/contract-model search \
  studies/requirement-learning-20261010/examples/req-bound-k6-r1-w0-o0-alias-g0-c0.json
```

The last command uses deterministic ordering. The opposite-condition document
ends in `c1.json`; the third readable example exercises the 128-condition source.
General compiler loading of this new artifact is follow-up work. Compiler
0.6.26 preparation currently covers the earlier released choice-model schema.

## Evidence and preparation failures

The clean measured producer is `1d59cf13f80562f26213d3a2771e7f3947eb77fa`, using
compiler `0d61324996d7723f4c8109cbb8508de8de3cb72c` and Go1.27.2. The frozen corpus
SHA256 is `6e1b0c73c7d539f69953af0658005b8d0609bc4054e30cf57cb8ce766bae6b63`.
[result/](result/) contains both models, both full histories, all source/candidate
observations, all search steps, input bounds, timings and start/completion records.
Compressed records use `gzip -n`; decompressed bytes match the originals.

Preparation failures are retained with their own source and process identity:

- [Rejected source draft](preflight-rejected/): duplicate condition inputs were rejected during source-only preflight.
- [Rejected collection](collection-rejected/): the rare source's original positive output obligations made one requested path impossible; collection stopped before either fit.
- [Empty-suite collector error](empty-collection-failure/): the collector called a nonempty observation API with zero conditions; it was corrected without changing the final corpus or fit settings.

The completed study's timing excludes these failed preparation runs, whose
available timings and partial records are retained separately. Local stack
paths and addresses were omitted from public failure excerpts.

Recount the stored observations with no new training, prediction, source lowering
or candidate execution:

```sh
go run ./studies/requirement-learning-20261010/audit \
  studies/requirement-learning-20261010/result
```

The audit checks independent exact-integer candidate arithmetic, all declared
condition rows, progress chains, model identities, input bounds and every
reported group. Regression tests reject altered outputs, condition bits,
training labels, artifacts and ranking records.
