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
| Write a `.gooo` rule and construct its body | [Compiler guide for public 0.6.34](https://github.com/kimjooyoon/meta-ontology-go/blob/1d3953acd92c3caf8416db5ceea0a35c3c8c739e/docs/source-relative-v3-contract-model.md) |
| Read model inputs or load a research model | [Canonical branch input and model guide](docs/canonical-branch-input.md) |
| Specify intermediate Boolean requirements | [Declared condition input](docs/declared-condition-input.md) |
| Search finite condition and branch choices | [Condition search](docs/condition-search.md) |
| Inspect the first failure and a saved result | [Compiler construction reports](https://github.com/kimjooyoon/meta-ontology-go/blob/83928fa4538245abe33b08549da47f8cbdc50ed2/docs/construction-results.md) |
| Understand model inputs, identities and provenance | [Decision input audit](docs/decision-input-audit.md) |
| Inspect how Boolean goals relate to a source comparison | [Experimental relation input](docs/source-relative-conditions.md) |
| Use the source-relative research weights in the SDK | [Loader, input inspection and construction](docs/source-relative-runtime.md) |
| Use the new V2 return-layout model and integer division | [V2 source contract and recorded results](docs/source-relative-v2-runtime.md) |
| Inspect output relationships or use V3 | [V3 source contract](docs/source-relative-v3-runtime.md) |
| Make decisions from source and declared cases with prose excluded | [Explicit prose-neutral V3 profile](docs/source-relative-v3-prose-neutral.md) |
| Inspect separate return-branch relationships for a future model fit | [V4 branch-owned input contract](docs/source-relative-v4-runtime.md) |
| Compare a learned decision with a simple rule and partial results | [Source-relative study: 288 documents](studies/source-relative-learning-20261011) |
| Find an older API or experiment | [Detailed research and release history](RESEARCH.md) |

## Versions and model files

- **Published Go SDK:** [v0.2.45-experimental](https://github.com/kimjooyoon/gooo-decision-runtime/releases/tag/v0.2.45-experimental),
  with the explicit prose-neutral V3 API. Its source release contains no model assets.
  SDK0.2.42's V2 model asset and earlier V1 files retain their own identities.
- **V1 compiler release:** [0.6.32-dev](https://github.com/kimjooyoon/meta-ontology-go/releases/tag/v0.6.32-dev),
  including SDK0.2.41 and the V1 relation model loader.
- **Current compiler release:** [0.6.34-dev](https://github.com/kimjooyoon/meta-ontology-go/releases/tag/v0.6.34-dev),
  including SDK0.2.44 and the explicit V3 adapter. Published native binaries bind
  source `b266c0e2cf6878385b59788945861a03e33f766b`; the release's zero-weight
  native checks and the fitted-model library studies have separate records.
  The SDK and compiler releases are experimental.
- **Toolchain:** Go1.27.2. Native construction needs it; reading a saved compiler
  report needs neither Go nor model weights.
- **Research weights:** [Gooo Canonical Decision Tiny v1 on Hugging Face](https://huggingface.co/asketeddy/gooo-canonical-decision-tiny-v1).
  The [download guide](docs/canonical-branch-input.md#download-an-existing-model)
  pins a revision and checksums. The Hub pooling fits and the SDK release's paired
  fits are distinct artifacts; their records and fingerprints identify them.

Install the library in your Go application:

```sh
go get github.com/kimjooyoon/gooo-decision-runtime@v0.2.45-experimental
```

The core packages are `bodyplan` for typed bodies, `pathplan` for source-owned
choices and finite search, and `contractdecision` for models that read source and
requirements. Earlier specialized model packages remain available. The compiler
owns the binding between `.gooo` declarations, semantic IDs and a prepared plan.

## Try the bundled V2 example

The following example adapter is a checkout addition after the SDK0.2.42 tag.
Its [division document](examples/contract-model/testdata/README.md) has three
output examples, one Boolean requirement and two possible return placements.
It is an authored SDK fixture; the compiler source guide covers `.gooo` binding.

```sh
GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./examples/contract-model inspect \
  -source-relative-v2-source examples/contract-model/testdata/source-relative-v2-division.json

GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./examples/contract-model search \
  examples/contract-model/testdata/source-relative-v2-division.json
```

The first command shows the 528-cell V2 input and all declared requirements with
zero model calls. The second constructs a body in deterministic order. To use
the released research weights, follow the [pinned download and V2 search guide](docs/source-relative-v2-runtime.md).
That route predicts once, then checks candidates; later search steps use no model.
An incompatible supplied artifact returns an error. Keep the selected-check
counts separate from the model's first-choice result.

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

[Source-relative V3](docs/source-relative-v3-runtime.md) adds explicit relationships
between declared integer outputs, predicate operand ranges and resolved source
constants. It reuses the 528-cell input and19,034-parameter numerical model with
a distinct artifact identity. The Go example exposes zero-call inspection and
one-call finite search. Public compiler0.6.34 includes this adapter. The
[factorized study](https://huggingface.co/asketeddy/gooo-canonical-decision-tiny-v1/tree/0aa1dd8c51e730632f0f4bc590b5498799178492/research/factorized-decision-20261012/result)
records first-complete results of254/512 for V2-full,220/512 for V3-full and
264/512 for V3 with prose excluded. These512 variants share16 held goal contracts
from four families. All three finish their supplied checks through finite search.

The [explicit prose-neutral profile](docs/source-relative-v3-prose-neutral.md)
is a checkout addition after SDK0.2.44. It clears only intent cells64..191 while
retaining the source, output relationships and every declared requirement.
Korean/English wording then yields identical model arrays in that study; this
agreement follows from the representation. Return-branch and layout decisions
remain areas for improvement. The public0.6.34 binary does not load this new
profile yet.

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
