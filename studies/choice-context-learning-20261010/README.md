# Choice-specific reading of Gooo contracts

Gooo가 선택할 수 있는 경로를 정하고, 작은 모델이 먼저 시도할 경로를 고릅니다.
이번 실험은 각 선택 지점이 입출력 사례를 따로 해석하도록 바꿨습니다.
388개 문서를 학습 전에 고정했고, 두 모델을 같은 32개 사례로 한 번씩 학습했습니다.
첫 선택 성공은 늘었지만 새 연산으로의 확장과 반대 의도 구분에는 한계가 남았습니다.

## What ran

- Producer: `cdc2466172547700c500aeb1cb36f73c943b9470`, clean Go 1.27.2 build.
- Compiler: `aa39466774afe64e56ee87f419ea56fb50917abe`, clean existing checkout.
- [Protocol](protocol.txt) and [complete frozen sources](corpus.jsonl.gz) were
  committed before either fit. The compressed corpus SHA256 is
  `7a28ccaef1d832c8e7259ed9371bb6933c4f53d7c066e556e58aaeabb39881a4`.
- Two new CPU fits: global case pooling and choice-conditioned case pooling.
  Same 32 training documents, signed-max-absolute pooling, 600 epochs, learning
  rate .3, L2 .0001, seed 17. No tuning or warm start.
- 388 documents × three modes = 1,164 finite searches; 776 initial model calls.
  Models rank the declared paths before execution. Gooo constructs each path
  and validates every expected output and condition. No later feedback calls.
- Negative constants, variable rebinding, English/Korean wording, unseen
  multiplication, 128-case tail requirements and contradictory contracts are
  retained. Integers above 2^53 stay exact in source and execution records.

## First valid selection

| Split | Documents | Deterministic | Global model | Choice model |
|---|---:|---:|---:|---:|
| Training | 32 | 16 | 17 | 19 |
| Alias / assignment / rebinding | 96 | 48 | 49 | 57 |
| New constants | 128 | 64 | 66 | 76 |
| English / Korean paraphrase | 64 | 32 | 32 | 37 |
| Unseen multiplication family | 64 | 32 | 32 | 31 |
| Last of 128 cases matters | 2 | 1 | 1 | 1 |
| Contradictory outputs | 2 | 0 | 0 | 0 |
| **All documents** | **388** | **193 (49.7%)** | **197 (50.8%)** | **221 (57.0%)** |

Outside training, the count is 177/356 → 180/356 → 202/356. Contradictions stay
in these first-selection denominators. Relative to the global model, the new
model improves 87 documents, regresses 63 and leaves 238 unchanged. Every
regression ID is in [audit.json](result/audit.json).

These are controlled variations of five arithmetic templates. They measure
this frozen set; wording variation does not establish general Korean/English
understanding. The new architecture has 3,072 extra parameters and more work per
choice, so this comparison does not isolate architecture from capacity/compute.

## Completeness and unresolved intent

| Observation | Deterministic | Global model | Choice model |
|---|---:|---:|---:|
| Satisfiable contracts completed | 386/386 | 386/386 | 386/386 |
| Contradictions exhausted and rejected | 2/2 | 2/2 | 2/2 |
| First-path output checks passed | 2,157/3,584 | 2,189/3,584 | 2,371/3,584 |
| First-path condition checks passed | 1,164/1,164 | 1,164/1,164 | 1,164/1,164 |
| Total attempted paths | 780 | 583 | 559 |
| Opposite-goal pairs both correct first | 0/193 | 15/193 | 29/193 |
| Opposite-goal pairs given the same path | 193/193 | 167/193 | 163/193 |

All 193 opposite-goal pairs change the learned scores, yet the new model still
chooses the same path for 163 pairs. One pair gets both first selections wrong.
The finite search, with all declared checks, provides the measured completion.
The model helps order that search and has substantial remaining discrimination
work. New multiplication and rare-tail selection are useful next targets.

## Local resource observations

| Observation | Global model | Choice model |
|---|---:|---:|
| FP32 parameters | 9,746 | 12,818 |
| Weight bytes | 38,984 | 51,272 |
| JSON artifact bytes | 113,988 | 149,979 |
| CPU fit wall time | 0.452 s | 1.725 s |
| Initial prediction median | 16.791 µs | 37.584 µs |
| Initial prediction p95 | 19.125 µs | 42.459 µs |
| Total measured search time, 388 documents | 54.99 ms | 64.43 ms |

Deterministic searches took 35.79 ms in total. Fewer attempted paths did not make
these tiny bodies faster overall; model preparation/prediction adds work. Each
model visits 19,200 training rows. The global encoder runs once per row; the
choice encoder runs twice for scoring and twice for gradient recomputation,
76,800 case encodings. Both run on this computer's CPU using Go fixed arrays.

The whole original process, including oracle collection, both fits and all
searches, took **3.20 s wall / 2.80 s user / 0.16 s system**, with maximum RSS
**75,776,000 bytes (72.27 MiB)** and zero swaps. CPU time / wall time is about
0.925 CPU cores on average. This is a process estimate; system-wide utilization
before/after and peak CPU utilization were not measured. Concurrent host work,
fixed mode order and a single run limit timing comparisons.

## Inspect the original evidence

The [readable examples](examples/) include a reversed bound operation whose two
opposite goals both select correctly, and a multiplication case that regresses
against the global model. Each `.gooo` file and exported `.json` is copied from
the saved original records. From the repository root, an optional local replay is:

```sh
go run ./examples/contract-model search \
  -model studies/choice-context-learning-20261010/result/model-choice.json \
  studies/choice-context-learning-20261010/examples/fresh-bound-k9-r1-w0-alias-g0.json
```

Running that command makes a new local prediction; it does not alter the
original study measurements. Omit `-model` for deterministic construction.

`result/` contains both trained models, full loss histories, every candidate's
generated Gooo/Go and int64 observations, declared training rows, every ranking
and progress chain, timings and build identity. Large JSON is `gzip -n` and was
byte-compared with the untouched originals after decompression. Local path
prefixes were removed from the published build/binary metadata names.

```sh
cd studies/choice-context-learning-20261010/result
shasum -a 256 -c SHA256SUMS
cd ../../..
GOWORK=off GOTOOLCHAIN=go1.27.2 go run \
  ./studies/choice-context-learning-20261010/audit \
  studies/choice-context-learning-20261010/result
```

The auditor independently checks int64 arithmetic, all frozen sources, training
membership, both original model hashes, candidate outputs/conditions, progress
chains and metric totals. It does no fitting, prediction, compilation or body
execution. Mutation tests reject changed large-integer observations, final-case
results, feature cells, labels, model bytes and receipts.

The new model is an explicit research artifact. The published default is
unchanged. SDK sessions and the local `examples/contract-model` command load this
format; ordinary released compiler support remains a separate integration step.
The next useful language-facing work is making each unresolved choice legible
through its expected outputs and failed conditions, while preserving exact
Gooo intent and bounded deterministic continuation.
