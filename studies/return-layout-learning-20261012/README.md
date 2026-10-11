# Fresh return-layout learning study

The frozen source, one fit and observed failures are public on
[Hugging Face](https://huggingface.co/asketeddy/gooo-canonical-decision-tiny-v1/tree/ece9b3a7934fa5aae5ca7faecb3bc035a05ba917/research/return-layout-learning-20261012).
The pre-fit revision is `567eae921fcf58c24e52023dee93cc625ac77a75`.
It contains100 sources,25 grouped contracts and the original collection records.
The result revision contains the new model, loss history, all100 first predictions,
split summaries and a separate native prototype observation.

`train/` is the frozen Go training program from SDK source
`889869fc553c6849194894dd6a47c7a91b9d9b27`. Its synthetic loader regressions
execute neither a fit nor a prediction. The original fit has already completed;
release checks preserve its files and do not invoke this training command.
Read the [V2 runtime guide](../../docs/source-relative-v2-runtime.md) for its input
ABI, integration APIs, measured first-selection limits and compiler availability.

`result/` keeps the original fitted files byte-for-byte. `corpus.json.gz` is a
timestamp-free compression of the original pre-fit JSON; the reader checks its
uncompressed SHA256 against the pinned public snapshot. `audit/` re-encodes all
100 source and requirement arrays with the current SDK, validates the original
labels, prediction order, first/checked counts and split summaries, and decodes
the pinned model identity. It performs zero new fits, predictions or candidate
executions. Mutation regressions reject changed source, features, probe exposure,
probabilities and completeness counts. CI checks file sums and exact audit output.
