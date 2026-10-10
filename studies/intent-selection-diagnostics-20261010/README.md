# Where opposite intents lost their distinction

The prior v6 relation experiment selected the smaller-value result for every
max/min task. This diagnostic keeps both frozen v5/v6 models and asks where
the distinction between their source-authored Korean/English intents disappears.

`Model.ExplainInto` now exposes the actual hidden activations, option scores
and complete candidate scores from one ordinary prediction. Think of this as
measuring signals inside the decision mechanism. The observations can locate
an inactive path, while the source cases still decide whether the selected body
works.

## Observed result

We selected all32 direct forms from the frozen source collection. For each of
16 max/min pairs, the body structure and encoded source were the same. Input
arrays differed only in the branch choice's intent channel; acceptable candidate
sets were disjoint. No intent text was rewritten.

| Observed on32 direct sources /16 opposite-intent pairs | v5 | v6 |
| --- | ---: | ---: |
| First-choice max validity | 12/16 | 0/16 |
| First-choice min validity | 8/16 | 16/16 |
| Pairs with identical branch hidden vectors | 0/16 | 8/16 |
| Pairs with identical branch option scores | 0/16 | 8/16 |
| Pairs with identical complete prediction distributions | 0/16 | 8/16 |
| Pairs with all24 branch hidden activations zero for both intents | 0/16 | 8/16 |

All eight inactive v6 pairs use the reversed return-arm form (`r1`), across
constants7,13,11,17 and both wording variants. For those rows, the two branch
scores are exactly the final learned biases: `[-0.16643664, 0.16643664]`.
The intent channel differs, but it cannot influence the branch score through
an all-zero hidden vector. The comparison choice remains a separate computation.

This explains the identical branch scores in those eight pairs. It does not
explain every v6 failure: the other eight max/min pairs have different hidden
vectors and scores, and their max first choices also fail. It also does not
identify a unique training cause for the inactive paths.

## Scope and counting

The [protocol](protocol.txt) was committed with the clean producer
`5ccf5e21e8c41256eb50d7b0506f5893148bcb99` before execution. There were exactly
64 **new diagnostic forward calls**,32 for each frozen model. Every returned
prediction equaled its saved original result. There were zero fits, weight
updates, body compilations, case evaluations and native executions. Validity
here is a lookup against the frozen acceptable candidate sets, not a new test
run. These32 sources are a subset of the previous162-source study.

The producer took0.49s wall time,0.05s user CPU and0.01s system CPU, with
17,874,944 bytes peak RSS on Go1.27.2/darwin-arm64. This includes loading saved
source and prediction records. It is not an inference latency benchmark.

## Next controlled change

A useful follow-up is an explicitly versioned activation that keeps a small
negative slope, with the same dimensions, inputs,16 training rows and fixed
training schedule. This would test whether the inactive branch path can retain
intent sensitivity. The artifact must distinguish that computation from ReLU;
existing weights must keep their original execution semantics. No such fit has
been run in this diagnostic. Feature scaling and optimization remain separate
possible contributors; this observation does not prove which change will help.

The model continues to order a finite set of source-permitted choices. Gooo
assembly and case checking can continue after a wrong choice. Better intent
sensitivity should be evaluated alongside final case completeness and the
number of attempts needed, with the deterministic path retained.

## Evidence

- [Report](result/report.json), [all pair observations](result/pairs.json),
  [all64 traces](result/records.jsonl.gz), [read-only audit](result/audit.json).
- The audit verifies exact model/input/prediction identities, source-owned intent
  text, active-unit counts, bias-only inactive scores and candidate-score sums.
  It makes no model forward call and executes no candidate.
- [SHA256SUMS](result/SHA256SUMS) binds the publication. Compressed files use
  gzip-n and were compared with the original decompressed bytes.
- The API tests cover ordinary-prediction parity, owned storage, concurrent
  callers, nonfinite inputs and transactional errors. The audit rejects an
  invented score for an inactive choice.
