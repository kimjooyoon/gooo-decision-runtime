# Learn the two Gooo decisions separately

Status: **three original fits and all original observations completed once**.
The [source and protocol were frozen publicly](https://github.com/kimjooyoon/gooo-decision-runtime/tree/36d14516a5f83a25d2efcf1bcf96fa87552b5ce2/studies/canonical-role-learning-20261011)
before execution. The original [protocol](protocol.txt) remains unchanged.

## Question and result

Can separate small models learn comparison operand order and branch layout more
reliably than one shared model? Each reads the same Gooo source and declared
output/condition requirements. The source defines four possible bodies; the model
orders their inspection. Gooo compiles each attempted body and checks the actual
output and condition values before accepting it.

On the 128 related evaluation documents, the joint model and split policy both
accepted their first body in **30/128 cases (23.44%)**, versus **32/128 (25%)** for
fixed order. Splitting doubled model calls and deployed weights. It improved eight
individual documents and regressed eight. This recipe gives no reason to adopt
the split policy as the default.

| Retained measure | Fixed order | Joint model | Two role models |
| --- | ---: | ---: | ---: |
| First body: training documents | 16/64 | 16/64 | 18/64 |
| First body: related evaluation | 32/128 | 30/128 | 30/128 |
| First body: excluded affine family | 16/64 | 16/64 | 18/64 |
| Final body: declared checks and separate probes | 256/256 | 256/256 | 256/256 |
| Candidate checks across all documents | 640 | 637 | 635 |
| Initial model calls across all documents | 0 | 256 | 512 |
| Total construction search time | 28.177 ms | 64.160 ms | 98.401 ms |
| Deployed parameters | 0 | 19,034 | 38,068 |
| FP32 weight bytes | 0 | 76,136 | 152,272 |

The final result concerns these finite checks and the four permitted bodies. It
comes from continuing candidate search after a failure. It provides no general
correctness guarantee or independent natural-language accuracy estimate.

## Did changed goals change the decision?

Among 64 related pairs, the source and output examples stay equal while the
required intermediate condition changes. Both proposed choice bits should change.
Both models change both score pairs in all 64 pairs, yet **neither changes either
first-choice bit in any pair**. Score movement alone is a weak success criterion.
The [audit](result/audit.json) preserves required and proposed changes separately.

The 96 direct-return/local-assignment pairs have equal canonical inputs, logits
and candidate order for every policy. These pairs overlap the other groups;
adding their denominators would inflate the sample count. The 32 affine condition
pairs also change expected outputs, so they are labelled separately in the audit.

## Frozen recipe and limits

- 64 training documents: bound and distance templates, constants 113 and -71,
  direct-return form and Korean/English wording variants 0/1.
- 128 related evaluation documents: constants 167 and -97, wording variants 2/3,
  direct-return and local-assignment forms.
- 64 excluded-family documents: the affine template, evaluation constants and
  forms. Templates and wordings were used in earlier studies; these are new
  documents from known generators, not 256 independent tasks or unseen prose.
- Each document has eight authored output checks and three condition checks.
  Sixteen additional output probes and three condition probes are recorded
  separately. Threshold condition checks intentionally overlap.
- Three fresh mean-pooling FP32 fits: seed 37, 2,000 epochs, learning rate 0.3,
  L2 0.0001. Evaluation and probe labels never enter training.

The joint target is the unique accepted mask. Comparison targets allow either
branch bit; branch targets allow either comparison bit. Splitting an additive
loss alone would be equivalent. Here independent weights change parameter sharing
and aggregate compute/size together. The complete recipes are compared with their
actual resource costs.

All policies use this producer's four-mask frontier: descending summed score,
numeric mask order for ties. It differs from the ordinary SDK lazy frontier, so
candidate counts and timings should not be compared directly with prior studies.
Each component currently computes both choice rows and the split policy retains
one row from each. That extra computation is included in the cost.

Execution uses Gooo-derived typed plans through SDK `Compile`, `Evaluate` and
`CheckConditions`; emitted Gooo and Go are retained. This study did not deploy
native Go programs. `split-policy.json` is research metadata; it adds no public
split-model loader and changes no default compiler policy.

## Time and memory on the local machine

One original complete process on Apple M4, 10 logical CPUs and 16 GiB RAM took
50.62 seconds wall time, 49.39 seconds user CPU and 0.34 seconds system CPU.
Maximum resident memory was **54.05 MiB**. CPU time divided by wall time was about
98.2% of one core on average; instantaneous machine CPU increase was not measured.

| Component | Fit time | Median prediction | p95 prediction | Calls |
| --- | ---: | ---: | ---: | ---: |
| Joint | 16.697 s | 105.063 µs | 113.125 µs | 256 |
| Comparison | 16.169 s | 103.792 µs | 111.166 µs | 256 |
| Branches | 16.643 s | 102.834 µs | 111.458 µs | 256 |

The split policy needs both component calls. A component latency is not its full
policy latency. Construction search time includes ranking, event writes,
compilation and authored checks; it excludes source lowering, initial four-body
collection, training and final probes. These are one-run observations, with no
causal speedup estimate. Last pre-update losses were 1.386181 (joint), 0.692716
(comparison) and 0.693149 (branches). [Local process cost](result/cost.json) retains
unmeasured fields as null.

## Retained evidence and read-only inspection

`result/` contains all three weight artifacts and loss histories, training rows,
256 source records, 768 construction records, 3,448 ordered events and summaries.
Large text files are compressed losslessly. `SHA256SUMS` covers the published
bytes; the audit's file hashes cover uncompressed original payloads.

From the SDK repository root:

```sh
(cd studies/canonical-role-learning-20261011/result && shasum -a 256 -c SHA256SUMS)
GOWORK=off GOTOOLCHAIN=go1.27.2 go run ./studies/canonical-role-learning-20261011/audit \
  studies/canonical-role-learning-20261011/result > /tmp/role-audit.json
cmp /tmp/role-audit.json studies/canonical-role-learning-20261011/result/audit.json
```

The reader performs zero fits, predictions or candidate executions. It checks
artifact identity, projected training targets, observations/pass flags, role score
selection, candidate order, event order and group counts. Its agreement checks
retained records; it does not independently execute the correctness oracle.

`freeze/` and `producer/` retain the original implementation. The separate producer
module uses the two existing local checkouts. The original run used compiler
`17153175844967a0153e0424cf2668353600164e` and SDK base
`6835253d45954ae89cf47cf255b8b23d8d99852a`. Do not repeat the original run to obtain a
preferred result. New experiments need a new question, corpus identity and record.
A missing `choice_id` in a tiny post-run audit regression fixture was corrected;
it did not alter any original model or experiment observation.

## Consequence for Gooo

Gooo can keep output examples, intermediate conditions, permitted choices and
observed failures in one inspectable source-bound contract. Here that deterministic
construction process completed every finite case even when the first choice was
weak. A useful next study should examine how condition evidence reaches each
choice, with explicit changed-goal checks. Increasing model count alone did not
solve that problem in this experiment. Adoption should depend on construction
cost and unmet requirements as well as first-choice completion.
