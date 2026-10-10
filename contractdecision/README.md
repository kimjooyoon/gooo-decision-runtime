# Reading Gooo requirements before choosing a path

[Inspect one decision](../docs/contract-signals.md) with `ExplainInto`: the actual
case summary, source prefix, joint hidden values and scores from the same pass.

[Paired-goal training](../docs/contract-goal-pairs.md) can compare two goals for
the same source while retaining the current inference layout and weight budget.
The API and local command are implemented; a corpus-level comparison is pending.

The optional [signed extreme pooling experiment](../docs/contract-pooling.md)
adds an explicit v2 computation with the same weight layout. The default and
the v1 description below retain arithmetic mean.

Gooo provides a body plan, its permitted choices and concrete examples of the
required behavior. This package learns how to rank those choices using the
examples as an explicit input. It fits and runs entirely in Go on the local CPU.

Each of the 1–128 declared input/expected-output pairs is encoded into 32 cells.
Shared learned weights turn each row into eight values. Their arithmetic mean
is combined with the 384 source/intent cells for each choice, followed by a
24-unit hidden layer and two option scores. Both layers use a fixed 0.01 leaky
ReLU. Every case participates; no prefix is sampled or truncated.

The model owns **9,746 FP32 parameters, 38,984 weight bytes**, regardless of case
count. This is the weight payload, not process RAM. Inference streams one case
row at a time and uses caller-owned workspace. The allocation regression test
observes zero heap allocations for the model kernel. CPU fitting keeps one
16KiB case snapshot per sample pass so forward and backward see identical rows;
it does not cache a whole dataset of case tensors.

## Run it

The [local example](../examples/contract-model/main.go) consumes strict
`gooo/body-codegen-typed-path-plan/v1` documents derived from Gooo source:

```sh
go run ./examples/contract-model fit -out contract.json train-a.json train-b.json
go run ./examples/contract-model search -model contract.json evaluation.json
go run ./examples/contract-model search evaluation.json
```

Supply only training documents to `fit`. It enumerates every complete candidate
and derives acceptable sets from actual output and condition checks. Label
collection supports up to six binary choices (64 combinations); larger training
documents are rejected explicitly. The command accepts 1–256 documents and
writes a new model file without overwriting existing weights. Default fitting
uses 400 full-batch CPU epochs, learning rate 0.3, L2 0.0001 and seed 17.

`search` supports 1–16 binary choices and uses the document's 1–64-attempt budget.
It emits the ranking receipt, initial state, actual attempts, finite completeness
and selected Go body. An incomplete result is emitted with an error exit status.
This example receives already extracted documents; the compiler or caller must
bind them to authoritative `.gooo` source. A nonempty sampling seed is currently
rejected. The ordinary released compiler CLI does not load this new schema yet.

The SDK entry point is `PreparedPlan.NewContractSession(ctx, model, cases)`.
Initialization makes one prediction before candidate execution; `Advance`
immediately compiles and checks the chosen candidates using the existing finite
frontier. The session retains no model and makes no later prediction calls.
Passing nil preserves the deterministic fallback ordering. Unsupported source
features produce a decline receipt and continue with fallback. A wrong model
choice does not bypass any output or authored condition.

## What the current tests establish

- Shared encoder and source-network gradients agree with finite differences.
- Training, artifact round trips and concurrent read-only prediction are stable
  with fixed inputs on the tested runtime.
- One paired fixture has identical source/intent arrays and opposite declared
  output goals. Joint learning chooses different paths and immediately executes
  the expected exact int64 values, including values above 2^53.
- The 128th case contributes to the pool; reader failures preserve outputs.
- Wrong choices continue through the finite verifier. Condition-only failures
  reject output-correct candidates. Missing models use the original order.
- Busy sessions return immediately; cancelled initialization returns no session.

These tests are implementation and learning regressions. The separate
[196-document study](../studies/contract-goals-20261010/README.md) now publishes
one trained artifact, all finite outcomes and local timings. It achieved
107/170 first-path validity on satisfiable documents outside training, versus
85/170 with deterministic ordering. It reduced attempts but took longer overall.
English wording, unseen tasks and rare-tail goals expose incomplete decisions.

## Limits and the next measurement

Exact integer bytes reach the case encoder, but an eight-value average is a
lossy summary. Different suites can share that summary. Repeated cases receive
repeated weight; changing order can slightly alter floating-point accumulation.
Additive option scores also cannot express every interaction between choices.
The compiler therefore checks the complete original suite after selection.
Candidate probabilities are relative scores within a supplied pool, not a
percentage of general correctness or a guarantee of program completeness.

Further studies need broader expressions and Korean/English wording, with
unsatisfiable goals, rare cases near the 128-case bound and deterministic
comparisons retained. Report first-choice validity, passed
outputs, passed conditions, eventual finite completion, attempts, call count,
time and process memory separately. Keep training contracts out of those held-out
groups and publish incorrect choices as well as successes.

The artifact schema is `gooo/contract-candidate-decision/v1`; it binds v6 source
features, `declared_integer_case_v1`, mean pooling, activation and FP32 weights.
Earlier flow models and their published measurements retain their own ABI.
