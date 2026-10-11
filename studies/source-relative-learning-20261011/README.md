# Source-relative conditions at a fixed learning budget

A small comparison needs to know where each input sits relative to its threshold.
This experiment passes that relation explicitly to a Gooo decision model: below,
equal or above, paired with the requested true/false result. The model then ranks
four possible bodies assembled from a comparison order and a branch layout.

The new input improved first-body selection in this synthetic corpus. A small
deterministic rule reached the same related first-choice count with fewer checks
and lower observed construction cost. The learned recipe therefore remains an
experiment; this result does not justify changing the default.

## Frozen source and scope

The [producer, protocol and source-only corpus](https://github.com/kimjooyoon/gooo-decision-runtime/tree/53698ee2b342b6015e6363d876e2103dc32230ff/studies/source-relative-learning-20261011)
were public before the original observation. The producer was built from that
clean revision with Go1.27.2. The pinned compiler is `67ee67c7`, the same source
tree as public compiler0.6.28. SDK base is `eb013ff6`.

There are 288 documents using known bound, distance and affine templates and
known Korean/English wording variants. Training uses64 bound/distance documents;
related evaluation128 and excluded affine64 use different constants and direct
return/assignment forms. Another32 documents diagnose missing and inconsistent
conditions. They are related variants, not288 independent programming problems.

Each main document has8 authored output cases and3 Boolean conditions. Sixteen
separate output probes and3 condition probes were fixed before execution. Output
inputs are disjoint; the threshold condition intentionally overlaps. Only the64
training documents' authored acceptance labels enter either fit. The [protocol](protocol.txt)
retains its original future-tense text. The observation has now completed once.

## First selection and construction

| Population / observation | Fixed order | Relation rule | Baseline model | Relative model |
| --- | ---: | ---: | ---: | ---: |
| Training: first complete body | 16/64 | 32/64 | 11/64 | 32/64 |
| Related: first complete body | 32/128 | 64/128 | 32/128 | 64/128 |
| Excluded affine: first complete body | 16/64 | 32/64 | 14/64 | 32/64 |
| Main256: final authored checks and probes | 256/256 | 256/256 | 256/256 | 256/256 |
| All288: final authored completion | 272/288 | 272/288 | 272/288 | 272/288 |
| All288: retained-result probe completion | 280/288 | 280/288 | 272/288 | 280/288 |
| Candidate checks in all288 constructions | 728 | 472 | 726 | 542 |
| Model calls | 0 | 0 | 288 | 288 |
| Total construction time | 36.214ms | 25.589ms | 98.630ms | 84.688ms |

Relative improves46 related documents and regresses14 against baseline. Its first
comparison decision is correct on128/128 related documents; its branch decision
is correct on64/128. The rule also gets128/128 comparisons and64/128 branches.
All112 direct/assignment pairs have equal inputs, scores and order per policy.

On64 related pairs where only intermediate conditions change, both required bits
change. Relative changes both proposed bits on64/64 pairs, and gets both members'
first body right on32 pairs. Baseline changes its comparison bit on0 and branch
bit on2; both members are right on0 pairs. The rule changes comparison on64,
branch on0, and gets one member right in each pair. These paired counts overlap
the main table. They describe behavior under known templates, not calibrated
probabilities or a percentage of general natural-language intent.

## Incomplete and ambiguous requirements

The16 missing-condition documents each admit two authored-complete masks. All
policies finish16/16 authored checks. Separate probes still fix the original
condition goal: retained bodies pass8/16 probes for fixed/rule/relative and0/16
for baseline. Output completion alone leaves that condition undecided.

The16 inconsistent documents demand a true strict comparison at equality. None
of the four bodies can satisfy all authored requirements. Every policy retains
all16 as `PARTIAL`, with10/11 passing observations per body and the remaining
condition failure visible. All16 retained bodies happen to pass the separate
probes, whose equality goal remains false. That probe result does not erase the
authored failure. We report each population separately rather than calling the
aggregate3,104/3,120 passed checks a general correctness score.

## Model and local compute

Two fresh joint mean-pooling FP32 fits use the same19,034 parameters,76,136 raw
weight bytes,seed37,2,000 epochs,learning rate0.3 and L2.0001. Baseline uses existing
canonical528-cell source rows. Relative adds16 source-relative cells at192:208;
208:384 remain zero. This supplies domain knowledge using exact int64 comparisons.
Neither extractor reads accepted labels. There was no tuning, warm start or retry.

| Cost / identity | Baseline | Relative |
| --- | ---: | ---: |
| Fit duration | 15.585s | 15.945s |
| Last retained pre-update loss | 1.386186 | 1.230908 |
| Prediction median,288 calls each | 106.625µs | 107.292µs |
| Prediction p95, nearest rank | 133.333µs | 116.625µs |
| Input preparation median | 47.125µs | 61.188µs |
| JSON artifact bytes | 223,114 | 222,730 |

The original process took32.57s wall,32.03s user and0.24s system CPU, with
63.594MiB maximum RSS on Apple M4 /10 logical CPUs /16GiB RAM. CPU time divided
by wall time is99.08% of one logical core on average. Instantaneous machine-wide
CPU utilization increase was not sampled. RSS covers both fits and all records.

Every construction freshly prepares its inputs. Timings include preparation,
input hashing/equality assertions, prediction/rule scoring, ordering, event
writes, compilation and authored checks; lowering/collection,fits,and separate
probes are outside those intervals. The rule prepares816 relation rows and
performs1,632 bucket operator checks across288 sessions. Relative prepares1,632
rows across both choice contexts; those counts overlap source cases.

The rule's preparation median is6.542µs. Its tiny scoring interval records0ns in
55 sessions, including49 with actual bucket checks. Those are clock-resolution
limits, not zero-cost computation. The initial read-only auditor incorrectly
required strictly positive rule time and stopped. We preserved every original
measurement, corrected only that audit assertion, added a fixture, and audited
the retained records again. [Correction record](result/audit-correction.json).
No fit, prediction or candidate execution was repeated.

This is one local run with fixed policy order, one seed and shared hardware;
the observed timings do not establish a causal speedup. Candidate execution uses
the Gooo-derived typed SDK plan. Native deployment and quantized model quality
were not measured.

## Read the result and load the research artifact

From this SDK checkout, the audit reads recorded outcomes without running models
or candidate bodies:

```sh
cd studies/source-relative-learning-20261011/result
shasum -a 256 -c SHA256SUMS
cd ../../..
GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./studies/source-relative-learning-20261011/audit \
  studies/source-relative-learning-20261011/result
```

`result/` contains both original weights, every loss, training arrays, all1,152
construction sessions,4,772 ordered events, source/candidate observations,
resource cost and the read-only audit. Records are losslessly compressed. Do not
rerun the original producer to verify these results.

The relative model uses schema `gooo/source-relative-condition-research-model/v1`.
Its decoder is `model.Decode` in this study's Go package; obtain inputs using the
study adapter and the source-relative SDK API. It rejects old schemas, and old
canonical decoders reject it. The public0.6.28 native compiler loader does not
accept this relative artifact. The baseline uses the existing canonical schema.

- Baseline artifact SHA256: `bbc60ea44f6f75b592f9dd6978e88ee8c8819054a3139c16a58dda2e22657170`.
- Relative artifact SHA256: `cae32bdad17ab818498c6099066918cd9c9c591eb118afe1bc6a7ef82dc83cc4`.

## Next practical question

The relation input now reaches the comparison decision, while output-driven
branch selection remains at half of the related examples. The next useful
experiment can expose how each permitted return expression relates to the
requested outputs, with explicit computation counts and an equally informed
rule baseline. A learned ranker also needs a case where candidate checking is
expensive enough to repay its extra preparation/prediction cost. Adding epochs
alone is not supported by this observation. Current defaults stay unchanged.

## 한국어 요약

입력이 기준값보다 작은지·같은지·큰지를 요구한 참/거짓과 묶어 전달했습니다.
새 모델의 관련 사례 첫 선택은32/128에서64/128로 늘었고, 조건만 바꿨을 때
선택도 함께 바뀌었습니다. 다만 간단한 규칙도64/128이었고 검사 횟수와 실제
조립 비용은 더 적었습니다. 이번 결과로 학습 모델을 기본값으로 바꾸지는 않습니다.

조건이 없는16개는 두 답이 모두 가능했고, 모순된16개는 끝까지10/11을 충족한
부분 결과로 남았습니다. 이 차이를 숨기지 않고 원본·가중치·실패와 함께 공개합니다.
