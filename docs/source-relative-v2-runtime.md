# V2 return-layout model from Go

The V2 source API keeps a fixed 528-cell input and adds Integer division to its
ordered expression representation. Source-owned comparison/branch choices,
declared output cases and Boolean conditions enter one initial prediction.
Finite checks decide acceptance. The session retains no model for later steps.

SDK0.2.42 publishes the V2 library API and model asset. The example command
adapter below is a source addition after that tag. Earlier SDK0.2.41 and compiler
0.6.32 load the V1 model. The V2 compiler connection is accepted in source and
is being checked for the next public compiler release.

## Identity and arrays

| Part | V2 contract |
| --- | --- |
| Artifact | `gooo/source-relative-condition-research-model/v2` |
| Source input | `canonical_branch_intent_ordered_condition_relations_v2` |
| Source/intent prefix | `[0:192]`, unchanged interpretation |
| Predicate/condition relations | `[192:208]`, sixteen cells |
| Reserved cells | `[208:384]`, zero |
| Three ordered expressions | `[384:528]`, three blocks of 48 cells |
| Division flag | Cell nine in each expression block |
| Output and condition rows | 32 FP32 cells each, up to128 each |
| Parameters | 19,034 FP32 values, 76,136 tensor bytes |

V1 decoding and its ordered encoder remain separate. V1 rejects division in
this source representation. V2 metadata selects its new interpretation explicitly.
The original eight classifier output labels are unchanged. Division truncates
toward zero according to Go Integer arithmetic; zero divisors return an interpreter
error, while generated Go retains its ordinary runtime fault behavior.

## Go APIs

The development `contract-model` example now offers a V2 route. From a checkout
including this CLI change, inspect a Gooo-derived typed path document and search:

```sh
GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./examples/contract-model inspect \
  -source-relative-v2-source examples/contract-model/testdata/source-relative-v2-division.json
GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./examples/contract-model search \
  -model model-source-relative-v2.json examples/contract-model/testdata/source-relative-v2-division.json
GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./examples/contract-model search \
  examples/contract-model/testdata/source-relative-v2-division.json
```

Inspection exports only the explicit V2 input, source explanations and all declared
requirements, with zero model/candidate/training calls. An unavailable context
retains its reason and omits the model array. V1/V2 inspection flags cannot be
mixed. The model artifact chooses the search input ABI; an incompatible supplied
artifact returns an error. Omitting it keeps deterministic search.
This example adapter is a source development addition after the SDK0.2.42 tag.

Given a source-bound `prepared *pathplan.PreparedPlan`, declared cases and model bytes:

```go
model, err := contractdecision.DecodeSourceRelativeV2(raw)
if err != nil { return err }
input, err := prepared.InitialContractInput(cases)
if err != nil { return err }
var features [528]float32
if err := input.SourceRelativeV2FeaturesInto(choiceID, &features); err != nil { return err }
session, err := prepared.NewSourceRelativeV2ContractSession(ctx, model, cases)
if err != nil { return err }
progress, body, err := session.Advance(ctx, 1)
```

Input preparation reads no model and executes no candidate. Session creation
predicts once before candidate evaluation. Advance checks output and condition
cases within the ordinary bounded frontier. Nil model uses deterministic order.
Unsupported source representations decline before prediction and preserve finite
search. Callers sharing immutable weights use their own workspaces and sessions.

`FitSourceRelativeV2` accepts training documents through the existing bounded Go
learner. An empty/cancelled/invalid input never begins fitting. Evaluation documents
and additional probes belong outside that API.

## Published model and original measurements

[The new model and full records](https://huggingface.co/asketeddy/gooo-canonical-decision-tiny-v1/tree/ece9b3a7934fa5aae5ca7faecb3bc035a05ba917/research/return-layout-learning-20261012/result)
are pinned. The model file SHA256 is:

```text
8570dd8c6a2aef2676dfc027740d869797a8a11733bd7e07cdd501dc21e4a0bc
```

One new fit used60 documents. Validation20 and heldout20 stayed outside fitting;
all four siblings of a contract share a split. The heldout set has five contracts.
First complete selection was12/20, compared with fixed-order5/20. Conditions
matched60/60, while return placement remained wrong on eight first selections.
Checked selection over retained interpreter observations was20/20 with160/160
additional probes. These checks and first selection have different denominators
and meanings.

The original CPU fit took0.736s;100 single predictions had median0.094ms and
observed p95 0.105ms. Tensor memory was76,136B and whole trainer peak RSS18.4MB.
Fit CPU/wall was0.988, about one logical core. A separate new Gooo prototype
task made one0.149ms prediction and passed native8/8 input expectations. Saved
construction passed the same8/8 with no new model call. Those are retained one-run
observations; they are not repeated for SDK release checks.

The model reads structured Gooo source and required examples. Korean and English
descriptions accompany them; general prose comprehension is not measured. Token
generation, GPU training and 1.58-bit quantization are outside this recorded fit.

## Release compatibility checks

CI retains every original feature/hidden/logit/probability-bit comparison, full
ranked mask comparison and finite search/program comparison for the earlier SDK
models. Those existing checks use `--compatibility-only`: collector wall timings,
allocation counters and heap-residency trials are disabled. Prepared compatibility
uses one semantic round. Ordinary internal execution receipts still identify the
model calls, and their incidental timing fields are excluded from comparisons.
The V2 retained reader additionally compares all100 frozen source/requirement
arrays with current encoders and validates original result counts without inference.

Original extraction receipts remain byte-identical in
`source-provenance-v0.2.41.json` and
`source-provenance-order-judge-v0.2.41.json`. The division and compatibility
source-evolution records bind their before/after hashes and sizes to SDK base
`84410b0a3d4ef0632f4a522732f72d8eeaa1834a`. Origin paths and origin hashes stay
the same; the current copied bytes are recorded as explicit SDK extensions.
The source tests check all current files and every unchanged historical entry.
