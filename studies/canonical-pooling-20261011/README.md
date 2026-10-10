# Does case pooling affect small Gooo decision models?

Status: **both original fits and all 768 construction sessions completed once**.
The corpus, protocol and producer were public at
[`706c590b`](https://github.com/kimjooyoon/gooo-decision-runtime/tree/706c590ba188afed5992fba07d313dbd6bd3de09/studies/canonical-pooling-20261011)
before execution. Both research weight files, losses, complete records and a
read-only audit are now retained under `result/`. No default model is changed.

The previous canonical study's model fit only 18/64 training documents on its
first choice. A read-only audit found distinguishable full saved inputs, while
training loss changed little late in its fixed recipe. That evidence does not
identify the cause. This follow-up compares two existing ways of combining case
features, with a new frozen set of constants and the same small model size.

Both models read Gooo source structure, authored outputs and intermediate
conditions. Each has 19,034 FP32 parameters (76,136 tensor bytes), runs in Go and
proposes a finite path order. Gooo checks the proposed candidates. Fixed ordering
is included as a comparator. No text generation is involved.

| Cohort | Documents | Role |
| --- | ---: | --- |
| Bound/distance, direct return | 64 | Both fresh fits |
| New constants, direct return/assignment | 128 | Related-template evaluation |
| Conditional addition/multiplication | 64 | Family excluded from both fits |

The known templates and Korean/English descriptions are reused explicitly.
These 256 documents are related variants, not independent business tasks.
Mean pooling averages learned case activations; extreme pooling retains the
largest absolute activation per channel. Training starts identically and can
produce different weights. Results will compare those complete recipes.

The [protocol](protocol.txt) retains its original pre-execution wording and fixes
both fits, all evaluations, first-choice and final checks, costs, failure
retention and the one-run boundary. Source/oracle material was written before
any candidate collection or prediction.

## First choices and completed construction

Each first-choice cell counts bodies that meet all authored output/condition
checks and all separately fixed probes. These populations agreed in this corpus.

| Cohort | Fixed order | Mean pooling | Extreme pooling |
| --- | ---: | ---: | ---: |
| Training documents | 16/64 | 14/64 | 29/64 |
| Related evaluation | 32/128 | 32/128 | 36/128 |
| Excluded computation family | 16/64 | 16/64 | 16/64 |
| Final declared and probe completion | 256/256 | 256/256 | 256/256 |
| Total candidate checks | 640 | 635 | 607 |

Extreme pooling improves 14 related-evaluation documents and regresses on 10
against mean pooling. In the excluded family it improves 14 and regresses on 14.
Every ID is retained in [audit.json](result/audit.json). The larger training
difference does not carry through to a broad evaluation advantage. Both models
remain weak at first-choice selection, and fixed order reaches the same final
completion within the four source-declared candidates.

Both modes retain equal canonical inputs, logits and proposals across all 96
direct-return/assignment pairs. This supports representation consistency for the
covered expressions; it is separate from selecting the right program.

Mean loss before the last update was **1.386181**, versus **1.391441** for extreme
pooling. These are the retained epoch-2,000 pre-update values, not a new loss
evaluation after the final update. A better first-choice count can coexist with
a worse recorded loss. This one seed and update recipe cannot establish the
cause of underfitting or justify replacing the default.

## Local cost

| Observation | Mean pooling | Extreme pooling |
| --- | ---: | ---: |
| One fit | 15.256s | 15.045s |
| First prediction median, 256 sessions | 95.938µs | 95.584µs |
| First prediction p95, nearest rank | 102.166µs | 104.125µs |
| Saved JSON weight artifact | 223,116B | 223,210B |
| FP32 weight array | 76,136B | 76,136B |

The whole original process used **31.29s wall**, **30.98s user+system CPU**, and
**77.031MiB maximum RSS**, on an Apple M4 with 10 logical CPUs and 16GiB RAM.
CPU/wall is about 99.0% of one core on average. This is not a measured increase
in instantaneous machine-wide utilization. RSS includes both fits and all records.

The construction interval totals were **26.149ms fixed / 62.495ms mean /
62.948ms extreme**. These include session setup and finite candidate checks;
they exclude source lowering, collection, fitting and separate final probes.
The two models each predicted 256 times before the first candidate. All later
advances made zero predictions. A fixed order and uncontrolled shared host were
used, so small timing differences are descriptive, not causal speedup estimates.

For these cheap rules, fewer candidate checks did not produce lower total
construction time than fixed ordering. The next model question should separate
representation and optimization on new, frozen data while keeping both requirement
streams. More training, a larger model, quantization quality, native deployment
and customer savings have not been demonstrated by this comparison.

The compressed corpus SHA256 is
`9bde751be7f8c39cb7084868db41a3c372d10923b825e9bf682960a6c51d23c5`.
Local SDK unit/static checks and producer unit/static/race checks passed. Corpus
preflight read all 256 sources and matched 96 direct/assignment input pairs;
model-routing regressions used only the separate K=5 zero-weight fixture.

## Reading the evidence

All stored records total about 1.4MiB. JSON/JSONL histories are losslessly
compressed; weight files are directly readable JSON. To check the records:

```sh
GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./studies/canonical-pooling-20261011/audit
```

The audit checks source/corpus/model identities, exact training scope, all
candidate populations, recorded output/condition flags, one initial prediction,
zero later predictions, summaries, improved/regressed IDs and paired forms.
It reads records and model metadata; it does not fit, predict, run candidates
or independently re-execute the original oracle. Median uses the mean of the
two central observations; p95 uses nearest rank.

The producer is a separate Go module because it uses the compiler's source
lowering adapter. Its local replacements require the two named clean checkouts;
the experiment checks their recorded identities. Regression tests use a K=5
fixture outside the study corpus. Do not rerun the producer to verify these
results; use the audit and retain the original execution as the observation.
