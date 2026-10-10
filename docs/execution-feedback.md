# Output counterexamples for a local decision model

The paired [Gooo branch study](../studies/branch-role-learning-20261010/README.md)
recorded 230 judgments with wrong outputs despite every declared intermediate
condition passing. Its condition-feedback searches made zero additional neural
calls: the observed failure did not change that model's input.

This implementation adds an exact output counterexample to the input and feeds
it back through the existing bounded search. Source intent still determines the
choices and cases. A model scores the next choices; the compiler checks the
combined body and executes its finite cases immediately.

## Representation

`source_intent_condition_output_channels_v3` has 320 FP32 input cells:

| Cells | Meaning |
| --- | --- |
| 0–255 | Complete v2 source, authored intent, intermediate condition observation, branch return roles; unchanged |
| 256–258 | Output failure present and the current choice's observed option |
| 259–267 | Signs of input, expected output and actual output |
| 268–269 | Actual output less than / greater than expected; integer comparison |
| 270–293 | Three exact int64 values, eight bytes each, most significant byte first; each byte divided by 2048 |
| 294–309 | All 16 candidate-mask bits |
| 310–311 | Declared choice count and current choice index, divided by 128 |
| 312–319 | Reserved zeros |

Initial input has no output observation. Cases have a separate digest but their
unseen expected outputs do not enter initial features. After a committed typed
attempt, the first failing output case enters the tail. The first failing
intermediate condition remains in its own channel. A subsequent successful
observation clears both. Type rejection or interrupted evaluation supplies no
new observation.

The failure belongs to the complete candidate. It does not identify a guilty
choice. The model receives the whole mask and can learn a different ranking;
search still retains the other paths. Only the latest committed candidate is
represented, so this does not provide memory of every earlier counterexample.

## Go API

```go
// model is an executiondecision.Model trained for the v3 representation.
// ctx has a deadline; prepared and cases come from the source compiler.
session, err := prepared.NewExecutionSession(ctx, model, cases, "")
// Handle err, then execute one finite candidate immediately.
progress, body, err := session.Advance(ctx, 1)
// For a partial search with remaining paths, refresh its frontier once.
ranking, err := session.ReconsiderExecution(ctx, model)
// Advance again to compile and evaluate the newly selected candidate.
```

`SearchExecutionBatches` provides the bounded loop. Existing limits remain:
1–16 binary choices, 1–128 integer cases, at most 64 attempted paths and 16
feedback rounds. The unchanged-feature and last-remaining-path cases skip a
neural call. Calls, timing, changed feature digests, exact output failure and
frontier application are recorded. Nil model uses the existing deterministic
search. Public session operations return a busy error instead of waiting on a
session mutex. The session never retains a model pointer.

For training, `InitialExecutionInput(cases)` and
`ObserveExecutionInput(ctx, choices, cases)` bind features to the prepared source
and finite cases. `ExecutionInput()` retrieves a session's committed observation
without executing the body again. `ExecutionFeaturesInto` fills a caller-owned
320-cell array for a named source choice. The owning compiler is responsible for
supplying source-derived cases; this SDK also accepts diagnostic caller cases.
Evaluation splits must stay out of fitting and feedback cases.

## Model and current evidence

`executiondecision` implements deterministic CPU fitting and immutable
320→24→2 inference. It owns 7,754 FP32 weights (31,016 bytes). This is weight
storage, not process RAM. Each choice shares the same network; complete candidate
masks define the acceptable set during fitting. Additive choice scores do not
express every joint dependency or calibrated correctness probability.

Its artifact schema is `gooo/execution-candidate-decision/v1`. The fixed-shape
implementation derives from the condition model's existing learning algorithm;
the older model implementation and artifacts stay at their original dimensions.
The extra channels need their own trained weights. The two decoders reject each
other's schema rather than reinterpreting dimensions.

Regression evidence includes exact large and extreme signed integers, simultaneous
condition/output failures, absence of unseen labels, owned receipts, deterministic
fallback, canceled observations and a wired model that changes a branch after an
output-only failure. That wired model tests the connection; it is not a learned
accuracy result. Unit fitting checks the new network separately.

The [new 128-source study](../studies/execution-feedback-learning-20261010/README.md)
trains v2, v3 with its output connections disabled, and v3 with output feedback.
It publishes all three models and all 2,112 judgments and 896 finite searches.
Output feedback changes four-region attempts from 62 to 54, with higher total
elapsed search time. First-choice validity and assignment cases show no benefit
over v2 in this run. The earlier paired study remains unchanged.

SDK0.2.32 publishes the representation and runtime. The current compiler release
does not yet load this new schema. Integration and richer assignment/literal
representation remain work; the finite study does not establish broad language
accuracy or a speedup guarantee.
