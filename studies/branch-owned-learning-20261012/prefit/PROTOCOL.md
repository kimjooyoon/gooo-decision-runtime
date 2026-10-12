# Branch-owned V4 input: a fresh paired decision study

Prepared 2026-10-12 before candidate collection, fitting or prediction. Both
models and every result will be retained, including failures. This study uses
public compiler library0.6.34-dev plus public SDK0.2.46-experimental and Go1.27.2,
with no local module replacement. This is a research library composition;
the installed public compiler0.6.34 binary does not acquire V4 support from it.
Compiler V4 integration is separately under original CI in PR1492.

## Question

Does giving the model the literals owned by each return branch help it connect
source operators, declared outputs and Boolean conditions when choosing one of
four possible bodies? Compare fresh V3 prose-neutral and V4 models with identical
seed, parameter count, data and training settings. Their source inputs differ
only in the 96 previously reserved branch-owned cells[256:352]. Intent cells
[64:192] are zero in both. No old fitted model, old task execution or old prediction
is repeated. Collection/training code is adapted from the earlier factorized
study; new source documents, specifications, code and dependencies are public and
read back byte-exact before any candidate collection or fit begins.

## Cohort and split

There are16 ordered operator-pair families from add/subtract/multiply/divide,
three literal regimes and two opposing goals (input below/above a threshold).
The96 parameterized goal contracts each have2 languages,4 source layouts and
2 body forms (direct return/local assignment):1536 documents,16 siblings per
contract. These are crossed variants of a finite task, not1536 independent
algorithms or100 distinct experimental methods. There is one wording per language;
the retained `wording` field is0. The `parameter` field identifies the literal
regime:0 small positive,1 small negative,2 positive beyond2^53.

Threshold=811+19*family+37*regime. Base literals are17+family%7 and29+(3*family)%11.
Regime1 negates both. Regime2 adds9007199254740993 and9007199254741033 respectively.
Every source operator has input on the left and a nonzero literal on the right.
Operand-order choices concern the predicate, and layout choices swap its returns.
The supplied Go oracle uses signed int64 arithmetic, including wrapping integer
multiplication/addition and truncating division. Each source includes8 output
cases: -9007199254741021,-41,-1,0,threshold-1,threshold,threshold+1,9007199254741025.
It includes3 Boolean cases at threshold-1,threshold,threshold+1. Opposing goals
retain the same source parts and change the required outputs/conditions and text.

For family f compute bucket=(7*f+3)%16:0..7train,8..11validation,12..15evaluation.
Keep every goal, regime, language, layout and form of a family in that split.
Train has768documents/48contracts/8families; validation and evaluation each have
384documents/24contracts/4families. Primitive operators appear in training;
evaluation holds out ordered pair combinations. Broader task generalization and
overlap with all historical synthetic contracts are outside this claim.

Four separate probe inputs per contract are -9007199254741063,threshold-17,
threshold+17,9007199254741059. They are disjoint from all declared case inputs in
that contract. They do not enter arrays, labels, fitting or candidate selection.
Evaluation contains1536 probe observations but96 unique contract/input pairs.

## Execution plan

Collect each document once through the pinned Gooo compiler library. Retain the
source hash, typed plan, exact neutral/V4 arrays, both output-relation views,
all8 output/3 Boolean feature rows and all4 candidate histories. Candidate
histories include separate probes. All6144 candidate histories are collected
once. A body is acceptable only when all8 outputs and3 Boolean cases match.
Any collection failure stops the study; it is retained and not silently filtered.
All signed literals passed a parse/array-only preflight before publication;
that preflight executed no model, candidate, fit or native program.

Fit exactly one fresh model per profile. Use seed2026101227,80epochs,LR0.01,
L2=0.00001, all768 training documents in fixed source order. Each has19,034 FP32
parameters/76,136 tensor bytes. Use `FitSourceRelativeV3ProseNeutral` and
`FitSourceRelativeV4`; both emit their explicit loader-ready artifact schemas.
No sweep, retry, validation stopping, default replacement or tensor conversion.
Fit API receives only training rows. Validation remains a reported split and does
not tune anything. Each original fit has a 10-minute execution deadline.

Make one prediction per profile per document,3072 total, after fitting. Retained
candidate outcomes determine first completion and traversal of the finite four
masks. No body is re-executed during model evaluation. Report the fixed mask0
baseline from the same retained histories. Exclusive start markers prevent
accidental collection/fit repeats; failure artifacts and incomplete work remain.

## Required reporting

Report first completion,8 output/3 condition match counts, candidate checks,
checked finite completion, first/checked probe results and fixed-first completion
by split. Report held-family/regime/goal/layout/form views as overlapping groups
of the same384 evaluation rows. Report24 held contracts separately, including
how many satisfy every one of their16 siblings on the first decision. Publish
paired row IDs and V3/V4 wins/losses/ties, not just a percentage.

Inspect exact raw input equality, accepted-mask conflicts and cross-split input
identity without invoking the models again. Group masks relative to the original
source layout; a raw mask is not a universal semantic label. Language agreement
comes from identical numeric arrays, not evidence of Korean/English comprehension.
Native behavior, timing, CPU/RSS, GPU training and1.58-bit quality remain separate
requirements. This study adds no resource benchmark. The full language goal and
100-method scope remain ongoing work regardless of these finite outcomes.
