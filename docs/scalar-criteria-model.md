# A small model runtime for declared Gooo paths

The development `criteriadecision` package implements a source-only numerical
ranking runtime. It reads `gooo/scalar-criteria-features/v1` and returns a ranking
over Gooo's declared scalar choice masks. The runtime has compiled; fitted
weights, prediction observations and a compiler adapter are subsequent work.

Gooo supplies the allowed construction paths and fixed requirements. The model
prioritizes those paths. The compiler owns combined typed construction, predicate
evaluation and source-bound observations. This keeps the model's task finite:
choose among the switches already present in the Gooo plan.

## Architecture and storage

| Layer | Shape | Parameters |
| --- | --- | --- |
| Shared choice encoder | 256 → 32, tanh | 8,224 |
| Shared predicate encoder | 256 → 32, tanh | 8,224 |
| Choice + mean predicate context | 64 → 32, tanh | 2,080 |
| Two option scores per choice | 32 → 2 | 66 |
| Shared pair energy | 32 elementwise products | 32 |
| Total | `shared_rows_pair_energy_v1` | 18,626 |

The FP32 parameter carrier occupies 74,504 bytes. Numerical scratch arrays
reserve 1,792 bytes; caller features, metadata, artifacts, Go and source preparation
occupy additional memory. These are layout figures, with process RAM and latency
still unobserved. The current artifact variant is FP32; quantized weights need a
separate implementation and their own observations.

Predicate embeddings are averaged into a context for each choice. Shared weights
operate on the active rows, allowing one to six choices and one to eight fixed
predicates. Pair terms couple different choices, so the architecture can learn
interactions between construction edits. Mean pooling and hashed input features
still lose information; expressive coverage and fitted quality need measurement.

## Full finite ranking

Bit `i` of a mask selects option 0 or option 1 of declared choice `i`. The model
scores every mask from zero through `(1 << choice_count) - 1`, up to 64 masks.
Scores include unary option terms and pair terms; the whole-mask distribution is
normalized with a maximum-subtracted exponential. Ties use ascending masks.
Choice marginals are derived from that same distribution.

`Prediction.ProposedMask` and `Candidates[:CandidateCount]` retain declaration
order. The compiler must map their bits through the prepared plan's option
metadata, bind the input/plan/model digests, and compile the complete combination.
`FeatureSHA256` additionally binds the exact active float32 rows: schema bytes,
the two row counts, then choice and predicate values in row order as little-endian
float32 bits. IDs, labels and unused rows are excluded from that numeric digest.
Individually valid options can still interact, so that combined check stays part
of construction. IDs and labels are absent from the network computations.

Probability describes the model's normalized scores. Fixed predicate evaluation
establishes the actual requirement outcome on its declared inputs. First-choice
satisfaction and eventual finite completion retain different denominators.

## Artifact and API

`EncodeArtifact` writes a complete little-endian FP32 parameter array and metadata
for `gooo/scalar-criteria-model/v1`. It binds feature schema, architecture,
parameter count and weight digest. It performs no fitting and supplies no
training-quality claim. `Load` owns immutable finite parameters and their exact
metadata identity; another call uses another scratch workspace.
An uninitialized Go `Model` value is unavailable to prediction. A caller must load
the complete artifact before obtaining a model-bound ranking.

```go
model, err := criteriadecision.Load(metadata, weights)
if err != nil {
    return err
}
var work criteriadecision.Workspace
var prediction criteriadecision.Prediction
if err := model.PredictInto(&features, &work, &prediction); err != nil {
    return err
}
```

`PredictInto` reads active numerical rows and copies digest bindings. It uses no
network provider, tokenizer, generated text, source execution or mutable model
state. An input error leaves output unchanged. Workspace ownership belongs to
the caller; each concurrent call needs its own workspace. The compiler's current
scalar path keeps deterministic selection while the new adapter is developed.

## Evidence and next work

The package was compiled against actual SDK source
`272861a2e8df24302fc84eae08dddedf32a05ece`, containing the numerical encoder.
Scoped Go modernization and formatting were checked. Five new unit tests passed
for initialization, complete uniform ranking over one to six choices, pair energy,
numeric input identity and artifact loading. They made seven numerical predictions
with synthetic test parameters; they executed no Gooo candidates or predicates
and performed no training. These checks establish the tested API invariants.
No placeholder weights were published as a fitted model.

Next work must produce fresh source-family data, fit this exact architecture,
record the training source and input identities, and keep held evaluation separate.
The first-choice result should be compared with the existing deterministic path.
The complete ranking also allows bounded retries, whose eventual satisfaction
needs its own observation. A trained artifact and compiler adapter will each
retain their own public status and evidence.
