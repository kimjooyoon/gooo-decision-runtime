# Gooo Decision Runtime

The Go library that helps [Gooo](https://github.com/kimjooyoon/meta-ontology-go)
assemble a program from choices declared in its source. A small local model can
suggest which choices to try first. The compiler builds the candidate, checks the
supplied outputs and intermediate conditions, and keeps the failures available
for the next step. Omitting the model gives deterministic ordering.

Think of a function with a few marked switches. Gooo describes the switches and
the requirements; this library operates them and records the checked result.
The models, training and runtime are implemented in Go.

## Start with your task

| What you want to do | Start here |
| --- | --- |
| Write a `.gooo` rule and construct its body | [Compiler guide for 0.6.28](https://github.com/kimjooyoon/meta-ontology-go/blob/ee24a413d311b0be117bb8a7e260bd1ca35994bd/docs/canonical-contract-model.md) |
| Read model inputs or load a research model | [Canonical branch input and model guide](docs/canonical-branch-input.md) |
| Specify intermediate Boolean requirements | [Declared condition input](docs/declared-condition-input.md) |
| Search finite condition and branch choices | [Condition search](docs/condition-search.md) |
| Inspect the first failure and a saved result | [Compiler construction reports](https://github.com/kimjooyoon/meta-ontology-go/blob/ee24a413d311b0be117bb8a7e260bd1ca35994bd/docs/construction-results.md) |
| Understand model inputs, identities and provenance | [Decision input audit](docs/decision-input-audit.md) |
| Inspect how Boolean goals relate to a source comparison | [Experimental relation input](docs/source-relative-conditions.md) |
| Compare a learned decision with a simple rule and partial results | [Source-relative study: 288 documents](studies/source-relative-learning-20261011) |
| Find an older API or experiment | [Detailed research and release history](RESEARCH.md) |

## Versions and model files

- **Go SDK:** [v0.2.40-experimental](https://github.com/kimjooyoon/gooo-decision-runtime/releases/tag/v0.2.40-experimental).
- **Compiler:** [0.6.28-dev](https://github.com/kimjooyoon/meta-ontology-go/releases/tag/v0.6.28-dev),
  including SDK0.2.40 and the canonical model loader. Both are development releases.
- **Toolchain:** Go1.27.2. Native construction needs it; reading a saved compiler
  report needs neither Go nor model weights.
- **Research weights:** [Gooo Canonical Decision Tiny v1 on Hugging Face](https://huggingface.co/asketeddy/gooo-canonical-decision-tiny-v1).
  The [download guide](docs/canonical-branch-input.md#download-an-existing-model)
  pins a revision and checksums. The Hub pooling fits and the SDK release's paired
  fits are distinct artifacts; their records and fingerprints identify them.

Install the library in your Go application:

```sh
go get github.com/kimjooyoon/gooo-decision-runtime@v0.2.40-experimental
```

The core packages are `bodyplan` for typed bodies, `pathplan` for source-owned
choices and finite search, and `contractdecision` for models that read source and
requirements. Earlier specialized model packages remain available. The compiler
owns the binding between `.gooo` declarations, semantic IDs and a prepared plan.

## Load and predict

Start by inspecting a prepared Gooo plan or an existing artifact. From this
repository checkout:

```sh
GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./examples/contract-model inspect \
  -canonical-source path-to-your-plan.json

GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./examples/contract-model search \
  -model path-to-model.json path-to-your-plan.json

GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./examples/contract-model search \
  path-to-your-plan.json
```

Replace the paths with your files. Inspection makes zero model calls or candidate
executions. The canonical search route predicts once before checking candidates;
later steps continue the finite queue without another prediction. The final
command uses fixed ordering. Other optional feedback routes have their own call
counts, documented in the [history](RESEARCH.md) and linked guides.

From an application, the canonical decoder is
`contractdecision.DecodeCanonicalInteractionRequirementConditioned`. The
[full guide](docs/canonical-branch-input.md) explains prepared plans, model ABI,
training input boundaries and returned progress. A partial body carries `PARTIAL`
progress. Check the completion state and its named requirements before using it.

The compiler command `gooo body-context` inspects `.gooo` sources directly.
`gooo body-construct --model` performs construction, and
`gooo body-construct --report result.json` reads a full saved JSON result without
executing its program. Preserve full JSON stdout for this last command; individual
`--out` files contain only parts of the result.

## Supported decisions and completeness

The canonical model input currently supports one root branch, comparison operand
order and branch layout, with at most one operation in each predicate or return
expression. Supported direct returns and local assignments expose the same
calculation to the model. Original source still determines the actual program.

Nested arithmetic, nested branches and other unsupported model shapes produce an
explicit reason and retain deterministic construction without inference. Malformed
model artifacts return an error. The general construction library has broader
body operations; a particular model's input support is narrower and is reported
separately.

Output examples and intermediate condition expectations are separate streams.
Reports keep first-choice failures, selected-body checks, remaining work, faults
and input overlap visible. Completion applies to the supplied finite checks;
behavior on other inputs remains unmeasured. Korean/English descriptions have
been used in synthetic studies with known wording variants. General prose
understanding has not been established.

## What the experiments show

These studies use different frozen corpora or recipes. Read each result in its
own scope; the rows are not a shared leaderboard.

| Study | First accepted body on 128 related documents | Fixed order |
| --- | --- | ---: |
| [Canonical source input](studies/canonical-contract-learning-20261011/README.md) | Canonical 36; paired legacy 33 | 32 |
| [Case pooling](studies/canonical-pooling-20261011/README.md) | Mean 32; extreme 36 | 32 |
| [Separate comparison/branch models](studies/canonical-role-learning-20261011/README.md) | Joint 30; split 30 | 32 |

The pooling and role studies finish all 256 documents through four-body search
under their finite checks. Fixed order has the lowest total construction time in
both studies. Role separation doubles calls and deployed weights without improving
its related first-choice result. Defaults remain unchanged. Original sources,
training scopes, weights, losses, failures and costs are retained with each study.

In the role study, changing only the intermediate conditions moves the scores
but leaves both initial choices unchanged in all 64 related pairs. This directs
further work toward how requirements reach a decision. It does not establish a
cause or justify a larger default model. Customer savings and external adoption
remain unmeasured.

## Memory ownership

The canonical models each have 19,034 FP32 parameters and 76,136 weight-array
bytes. APIs use fixed arrays and caller-owned workspaces for bounded storage.
Whole-process RAM includes the Go runtime, source plans, observations and training
data; weight bytes alone do not describe it. The original role-study process
used 54.05MiB maximum RSS and 50.62 seconds wall time on Apple M4. Component
prediction medians were about 0.10ms, with two calls needed by the split policy.

Older compact model formats and their packing/runtime arithmetic are documented
in [RESEARCH.md](RESEARCH.md). The canonical research weights use FP32; their
1.58-bit quantization quality is unmeasured.

## Read the evidence and contribute

CI runs formatting, vet, unit/race checks and retained-record audits. The role
study's auditor reads its original records without new training, inference or
candidate execution. Reuse those records to inspect results. A new model experiment
should identify its question, corpus, evaluation scope and costs before execution,
then retain failures alongside successes.

- [Compiler current status and supported releases](https://github.com/kimjooyoon/meta-ontology-go/wiki/Current-Status)
- [Detailed APIs, provenance and earlier releases](RESEARCH.md)
- [MIT license](LICENSE)
