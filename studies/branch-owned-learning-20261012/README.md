# Original paired V3-neutral / V4 results

These are the once-collected outcomes of the
[preregistered study](https://huggingface.co/asketeddy/gooo-canonical-decision-tiny-v1/tree/c520e47d5e68ac44cd0ad0326b3aba711a226fbf/research/branch-owned-learning-20261012).
A [preflight scope clarification](https://huggingface.co/asketeddy/gooo-canonical-decision-tiny-v1/blob/5b963a36a27c7bae9b8bcc7074d52bad129cbf99/research/branch-owned-learning-20261012/PREFLIGHT_SCOPE.md)
was published before collection. It corrects an overly broad sentence about
parse-only coverage; data, split, settings and outcome obligations were unchanged.

1536 new Gooo documents represent96 goal contracts in16 operator-pair families.
Eight output requirements and three Boolean requirements define complete bodies.
The models each fitted once from fresh identical seeds on768 documents. Evaluation
uses384 sibling rows from24 contracts in four held families. Neither fit used
validation or evaluation rows. Both explicit model artifacts are retained here.

| Held evaluation observation | V3 prose-neutral | V4 branch-owned |
| --- | ---: | ---: |
| First candidate meets all8 outputs +3 Boolean requirements | 112/384 (29.17%) | 60/384 (15.63%) |
| First output expectations met | 1896/3072 | 1396/3072 |
| First Boolean expectations met | 816/1152 | 712/1152 |
| Candidate checks to finite completion | 960 | 1016 |
| Checked finite completion | 384/384 | 384/384 |
| Separate probes passed after completion | 1536/1536 | 1536/1536 |
| Contracts complete across all16 sibling layouts/forms/languages on first choice | 1/24 | 0/24 |

The fixed-first mask0 baseline completes96/384 rows. V4-only first completion
occurs on40 paired rows, neutral-only on92; both complete20 and neither completes
232. These are paired observations of the same384 rows. Differences are descriptive
and do not establish an estimate for an independent task population.

Both profiles have384 distinct exact raw inputs across all1536 source variants,
zero incompatible accepted-mask groups and zero exact input groups crossing splits.
The added branch channels did not increase that distinct-input count. This does
not establish hidden representation separability or learnability. Both profiles
have low training first-completion (240/768 and196/768). The retained losses and
row outcomes support investigating how source and condition information reaches
the decision head; this study does not identify a single cause of the regression.

All four held families show lower first completion for V4. Small positive,
negative and above2^53 literal regimes give neutral36/128,36/128,40/128 versus
V4 20/128 in each regime. Direct-return and assignment views agree within each
profile. Identical Korean/English arrays explain pair agreement; no language
comprehension score is claimed. The1536 separate probe observations repeat96
unique held contract/input pairs; they are not1536 independent behavioral inputs.

Collection retained6144 original candidate histories. Evaluation made3072 model
predictions total, and read those histories without executing the bodies again.
The saved-data audit invokes no fit, model, candidate or native program. There are
no native/compiler-binary, timing, CPU/RSS, GPU or quantization measurements here.
19,034 FP32 parameters /76,136 raw tensor bytes per model remain layout sizes.

Both artifacts load in SDK0.2.46. V4 compiler support is under review in PR1492;
public compiler0.6.34 continues its earlier SDK0.2.44 routes. Models are explicit
research choices and the default ordering remains deterministic. This result does
not justify promoting V4 as a quality improvement or replacing prior weights.

Files include the compressed corpus, both models, full training histories,
per-row observations, split/group reports and all paired evaluation rows.
`SHA256SUMS` binds every file. The producer source is in the preregistration's
`tool` directory; its `audit` mode reads these saved observations without fitting
or inference. All original failures are retained, and no fit was retried.

## Repository layout

This GitHub folder contains the preregistered Go producer, source archive,
specifications, exact preflight clarification and small retained reports. The
[complete immutable results and both models](https://huggingface.co/asketeddy/gooo-canonical-decision-tiny-v1/tree/8d5c1c2310017ec5d147b77b891bce894aaa03e4/research/branch-owned-learning-20261012/result)
are on Hugging Face. Source/model arrays and candidate histories are not duplicated
in this checkout. Run `bash studies/branch-owned-learning-20261012/check-retained.sh`
to parse the prepared fixtures and audit frozen outcomes without fitting,
prediction or candidate execution. It uses a temporary directory and downloads
only the compressed corpus and two observation files, about4MiB total.

The subsequent [per-choice diagnosis](../branch-owned-choice-diagnosis-20261012)
reads these same frozen outcomes. It separates comparison-order and branch-layout
errors and describes the saved ranking distribution without another prediction.
