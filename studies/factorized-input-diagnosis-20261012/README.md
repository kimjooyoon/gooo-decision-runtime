# Where the frozen factorized model loses first-choice completeness

This follow-up reads the original 2,048 documents and 6,144 saved model outcomes
from the [factorized study](https://huggingface.co/asketeddy/gooo-canonical-decision-tiny-v1/tree/0aa1dd8c51e730632f0f4bc590b5498799178492/research/factorized-decision-20261012/result).
It makes zero fits, predictions, candidate executions or native executions.
The original models, arrays and observations retain their published identities.
The diagnosis is exploratory analysis of already observed outcomes.

## What the saved evidence says

Evaluation contains 512 document variants, sharing 16 goal contracts from four
held operator-pair families. Each family has 128 variants and four contracts.

| Ordered return operations | V2 full | V3 full | V3 prose-neutral |
| --- | ---: | ---: | ---: |
| add, multiply | 66/128 | 54/128 | 56/128 |
| subtract, add | 72/128 | 54/128 | 72/128 |
| multiply, divide | 64/128 | 56/128 | 72/128 |
| divide, subtract | 52/128 | 56/128 | 64/128 |
| All held documents | 254/512 | 220/512 | 264/512 |

Cells count first bodies satisfying all six output and three Boolean requirements.
In the neutral profile, all 248 incomplete first bodies satisfy the Boolean
requirements and fail some output requirements. None of the 16 held contracts
has a correct first body across every language, wording, layout and form variant,
in any of the three profiles.

The neutral profile's first-choice results also depend strongly on which raw
choice combination is required:

| Required raw mask | Complete first choices |
| --- | ---: |
| 0 | 72/128 |
| 1 | 0/128 |
| 2 | 64/128 |
| 3 | 128/128 |

A mask records switches relative to the original source layout. It does not name
a universal semantic outcome. The four layout variants change the required mask
while preserving the intended computation. These denominators share the same
512 rows as the table above; they add no independent evaluation tasks.

## Are opposite answers encoded as identical inputs?

The reader hashes both 528-cell source arrays, six 32-cell output rows and three
32-cell Boolean rows: 1,344 float32 cells, serialized as exact little-endian bits.
It applies only the published neutral clearing rule to source cells [64,192).
The original prediction call supplied the same ordered mask list [0,1,2,3] to
every row. All four retained candidate histories are checked against their
stored output equalities, Boolean observations and acceptance flags.

| Profile | Distinct inputs, all 2,048 rows | Distinct inputs, evaluation 512 rows | Conflicting input groups |
| --- | ---: | ---: | ---: |
| V2 full | 1,024 | 256 | 0 |
| V3 full | 1,024 | 256 | 0 |
| V3 prose-neutral | 256 | 64 | 0 |

No exact input group crosses the train, validation or evaluation split. Every
group has at least one raw mask that satisfies all its members. Thus exact input
identity alone does not force these observed failures. Distinct arrays can still
collapse inside a learned representation or be difficult for a small model to
use. Hidden activations, pooling collisions and learnability remain unmeasured.
The report's shared-choice upper bound is a combinatorial count for these rows,
not an achieved model score or a generalization estimate.

## What this changes in the next implementation

The immediate target is branch-to-requirement association and stability across
equivalent source layouts. A future version can expose which authored return
expression belongs to each branch and how both permitted switches relate to
the declared cases. Its feature contract must distinguish authored facts from
any executed candidate observations, with a separate schema if meaning changes.

Keep first-choice quality separate from completeness after bounded checking.
The existing compiler should keep checking source-owned requirements and retain
deterministic search when a model is absent or the source shape is unsupported.
This report supplies no evidence that a larger model, more epochs, quantization
or a new default would improve results. Any quality comparison needs new tasks
and a published protocol before fitting or calling a model.

## Reproduce the reader

Run from the SDK repository root. The fetch helper creates a fresh directory and
downloads only four immutable data files, about 6 MiB total. Its output directory
must not already exist. Go checks all four original SHA256 identities before
reporting results; it imports only the standard library.

```sh
bash studies/factorized-input-diagnosis-20261012/fetch.sh /tmp/gooo-frozen-inputs
GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./studies/factorized-input-diagnosis-20261012/audit \
  /tmp/gooo-frozen-inputs > /tmp/gooo-input-diagnosis.json
cmp /tmp/gooo-input-diagnosis.json studies/factorized-input-diagnosis-20261012/diagnosis.json
```

The synthetic helper tests exercise contradictory identical inputs and the
neutral clearing boundary. They fit no models and execute no generated bodies.
CI compares the read-only report to the retained result while preserving the
existing SDK compatibility checks.
