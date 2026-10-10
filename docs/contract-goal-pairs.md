# Learning to separate two goals for the same Gooo source

The [recorded diagnostic](../studies/contract-signal-diagnostics-20261010/README.md)
found that different expected outputs changed the internal scores for all 97
opposite-goal pairs. The mean model still chose the same path for 61 pairs; the
signed-pooling model did so for 62. This motivates an optional training objective
that explicitly compares paired goals. Its effect on those documents has **not
yet been measured**.

`contractdecision.FitWithGoalPairs` takes the ordinary training samples, explicit
pairs of sample indices and a weight/margin configuration. Each pair must have
identical source arrays, identical candidate masks in the same order and disjoint
acceptable sets obtained from the compiler's complete output/condition checks.
Each sample belongs to at most one pair. Evaluation examples stay outside fit.

Think of two requests that share the same tools: one asks for the smaller value,
the other for the larger value. We train each request against its correct paths
and also ask the two requests to establish opposite preferences.

## Objective

For a sample, `score(set)` is log-sum-exp over the candidate scores in that set.
Let A and B be the acceptable sets for the two goals:

```text
gap = (score_goalA(A) - score_goalA(B))
    + (score_goalB(B) - score_goalB(A))
pair_loss = softplus(margin - gap)
loss = mean ordinary candidate loss + weight * mean pair_loss
```

L2 is applied to the update separately, as in the existing fitter. `Epoch.Loss`
records the combined objective before its update. With singleton sets, a
goal-independent candidate bias cancels algebraically in the paired gap.
For larger sets, log-sum-exp also depends on how scores differ inside each set.
Finite-precision arithmetic still applies.

A positive paired gap alone does not establish that both requests choose valid
paths. One margin can remain negative. The ordinary candidate loss is retained,
and actual execution must still check every authored output and condition.
Candidate probabilities remain relative ranking scores.

## Local usage

```sh
go run ./examples/contract-model fit -goal-pairs -out paired.json \
  minimum-a.json maximum-a.json minimum-b.json maximum-b.json
go run ./examples/contract-model search -model paired.json evaluation.json
```

The example pairs consecutive training documents, derives labels by enumerating
their complete permitted candidates and uses weight 0.5, margin 2. It rejects
odd document counts, differing sources and overlapping acceptable sets. The
model path must be new. The fit receipt uses `gooo/contract-local-fit/v2` and
records the pair indices and settings. Without the flag, the existing v1 receipt
and ordinary fitting route remain.

The Go API accepts explicit pair indices and settings. Mean or signed extreme
pooling can be selected. Inference uses the existing artifact schemas, 9,746
FP32 parameters and 38,984 weight bytes. There is no new inference layer or
external dependency. Training reads each paired sample twice per epoch, once
for its ordinary loss and once for the paired loss, using bounded scratch arrays.
Training cost is expected to rise; this is not an efficiency claim.

## Evidence and next measurement

Tests cover analytic gradients against finite differences for both pooling
methods, set-valued targets, stable loss evaluation, pair validation, cancellation,
the 128th case, artifact round trips and repeated training on small synthetic
fixtures. The command test checks two opposite goals using exact integers beyond
2^53 through actual candidate execution. Two command-test epochs establish the
integration, with no first-choice accuracy claim.

The next controlled measurement should keep the original 24 training rows,
12 explicit pairs, seed, epochs and parameter budget fixed. Compare a single
new fit against the frozen baseline records, retaining English wording,
unseen families, all regressions and contradictory goals. Report both-valid and
same-proposal pair counts alongside first-choice validity, eventual finite
completion, attempts, latency, fit cost and process memory. The current default
and the published model weights remain unchanged until such evidence exists.
