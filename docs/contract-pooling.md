# Keeping a rare declared case visible

The first contract model averages eight learned coordinates across all declared
cases. In the fixed Gooo study, 61 of 97 opposite-goal pairs still selected the
same path. A case appearing once among 128 receives only 1/128 of the mean.
This motivates a controlled pooling comparison; it does not identify averaging
as the sole cause of the observed errors.

The optional `signed_max_abs` computation reads every case and keeps the signed
largest magnitude in each of the eight coordinates. Equal magnitudes choose the
positive value. Exact equal winners use the first row for the training
subgradient. Inference pooling is unchanged by permuting or duplicating rows;
training gradients can change at exact ties between different rows. The compiler
continues to check every original output and condition after selection.

The source input, case encoder, 24-unit hidden layer, output head and all 9,746
parameter positions stay the same. Weight payload remains 38,984 bytes. The
runtime still makes one initial judgment and uses a finite typed frontier.
It neither decodes text nor executes candidate bodies to form model inputs.

This computation has artifact schema `gooo/contract-candidate-decision/v2`.
The v1 schema and default `New`/`Fit` retain `arithmetic_mean`, original artifact
bytes, fingerprints and computation. A different pooling name requires explicit
construction and a matching artifact schema; old weights are never silently
reinterpreted by a loader.

```sh
go run ./examples/contract-model fit -pooling signed_max_abs \
  -out new-contract.json training-a.json training-b.json
go run ./examples/contract-model search -model new-contract.json evaluation.json
```

The API is `NewForPooling` / `FitForPooling`. `Pooling` and `ArtifactSchema`
describe the decoded computation. The ordinary compiler's new local integration
currently accepts v1; this v2 comparison is available through the SDK session
and example commands. No default model or compiler profile is switched here.

The [fixed protocol](../studies/contract-pooling-20261010/protocol.txt) compares
one fresh fit against the saved mean-model outcomes, retaining regressions,
contradictory goals, rare cases, every split and process costs. The original fit
and observations are not rerun. Results on this known corpus measure this
specific change, not broad language understanding.

## Research context

[Deep Sets](https://arxiv.org/abs/1703.06114) studies neural operations on sets
with shared element processing and aggregation. [PointNet](https://openaccess.thecvf.com/content_cvpr_2017/papers/Qi_PointNet_Deep_Learning_CVPR_2017_paper.pdf)
uses shared point processing and max pooling. Those papers motivate treating
aggregation as an explicit design choice. Our signed-magnitude variation is a
small Gooo experiment; their application results do not establish its quality.

Extreme pooling can discard frequency and interactions between cases, and an
irrelevant outlier can dominate a coordinate. Its eight-value summary is still
lossy. Scores remain finite-choice hints; exact source-bound cases and observed
program behavior determine completeness.
