# Separate source, intent and observed conditions

`ConditionFeaturesInto` is an experimental input for the next small Gooo decision
model. It writes a fixed `[256]float32` array (1,024 bytes) from structural source
fields, the complete Korean/English intent and one typed condition observation.
It performs feature extraction only. Training, a model artifact and an explicit
inference contract are subsequent steps; the existing model loader rejects its
new feature-version identifier.

## Why separate these inputs?

The [frozen input comparison](https://raw.githubusercontent.com/wiki/kimjooyoon/meta-ontology-go/observations/condition-feedback-ablation-20261010/README.md)
scored six original model inputs and three variations of each. Removing the
condition clause changed two eligible choices. Exchanging expected/actual
Boolean values changed none. Existing source-v3 features combine intent and
feedback into relative byte-position buckets. Appending a longer feedback clause
can therefore move the original intent between buckets.

This API keeps three regions independent:

| Region | Dimensions | Encoding |
| --- | ---: | --- |
| Source | 0–63 | Existing validated semantic fields, L2 normalized |
| Intent | 64–191 | Complete UTF-8, 1/2/3-byte fragments, four position buckets with 32 hash cells each, L2 normalized |
| Observed condition | 192–255 | Dedicated fields below, fixed power-of-two scaling |

Adding or changing feedback leaves the first 192 values byte-identical. An intent
can occupy the full 512-byte bound because source and observation are separate
typed arguments. Input is declined atomically if invalid or oversized.

## Observation layout

Indexes in this table are relative to 192. Boolean flags have value 1/8 when set.
There is no combined normalization that rescales the other two regions.

| Index | Meaning |
| --- | --- |
| 0 | Observation present |
| 1–2 | Predicted choice equals/differs from the observed choice |
| 3–4 | Condition reached/unreached |
| 5–6 | Expected false/true |
| 7–9 | Actual unobserved/false/true |
| 10–11 | The candidate used first/second option at the predicted choice |
| 12–13 | The candidate used first/second option at the observed choice |
| 14–16 | Negative/zero/positive input |
| 17–24 | Exact big-endian two's-complement input bytes, each divided by 2048 |
| 25–40 | Candidate mask's 16 bits, low bit first |
| 41–43 | Declared choice count, predicted index and observed index, divided by 128 |
| 44–63 | Reserved zero fields |

Expected and actual roles occupy different cells. An unreached condition has an
explicit unobserved cell and neither actual Boolean cell. The int64 input is
split with integer shifts before each byte is converted; it never passes through
a whole-value floating-point conversion. Contract tests reconstruct exact values,
including the signed extremes, `9007199254740993`, `±9007199254740995` and
`18014398509481990`.

`Choice`, `ObservedChoice` and `CandidateMask` must refer to the same immutable
Gooo plan with 1–16 choices. A consumer obtains source fields from the prepared
plan and binds the observation to its original source and declared cases. The
feature encoder validates ranges and representation; it does not independently
prove that the supplied observation came from executing that plan.

## Consumer outline

```go
source, err := prepared.SourceFeatures(choiceID)
// Handle err, bind the candidate mask and retain the original condition receipt.
var features [decision.FeatureDim]float32
err = decision.ConditionFeaturesInto(source, authoredIntent, decision.ConditionFeedback{
    Present: true, ChoiceCount: 3, Choice: 1, ObservedChoice: 1,
    CandidateMask: 2, Input: -9007199254740995,
    Expected: true, Reached: true, Actual: false,
}, &features)
```

Use a zero `ConditionFeedback{}` before an observation exists. A valid call uses
fixed local scratch and allocates zero heap objects in the contract test. Separate
callers share no mutable scratch; race tests exercise concurrent projections.
The destination is replaced only after source, intent and observation validation.

## Training and evaluation criteria

The next training run should use Gooo candidate evaluations to label the full set
of valid choices, retaining correlations between multiple decisions. Pair source
contracts with opposite expected conditions, distinguish unrelated choices and
unreached branches, and split new expressions/program structures separately from
new Korean/English wording. Measure correct selections and abstentions together
with candidate tests, model calls and failed finite cases.

This representation preserves distinctions that the previous text channel mixed
together. It still hashes intent fragments, so different sentences can alias.
Field separation alone provides no evidence of better model accuracy. The
compiler continues to evaluate declared output and condition cases after ranking.

This is new SDK-owned source. The 69 files tracked by the earlier extraction
manifest retain their original bytes and history.
