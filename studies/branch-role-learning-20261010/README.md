# Learning with branch return roles

This study compares two fresh local decision models trained on the same Gooo
source contracts. Their feature representations differ: v2 adds branch return
roles to twenty reserved cells. The full
[protocol](protocol.txt) fixes source groups, training settings and observations
before fitting either model. Results are pending in this producer commit.

The exporter lowers actual Gooo source using compiler commit `1ceb96b9` and the
published SDK0.2.31. The training program consumes the exact exported documents,
derives acceptable complete masks from typed execution, and fits only the forty
training contexts. Each model prediction is immediately followed by assembly and
checking of its chosen body. Five finite-search modes then use frozen weights.

The sixty programs are variants of max/min, clamp and sign-code tasks. New
wording, local-return structures, nested clamps and remaining literal-value
collisions are reported separately. These small groups do not establish general
language accuracy. Source test cases remain visible during finite search.

## Run a new experiment

Use clean committed sources, Go1.27.2 and fresh output directories. The nested
exporter module expects a sibling compiler checkout at its pinned revision.

```sh
# From exporter/, build the compiler-linked source exporter.
GOWORK=off GOTOOLCHAIN=go1.27.2 go build -trimpath -o /tmp/my-branch-export .
# From the SDK root, build the paired training/search producer.
GOWORK=off GOTOOLCHAIN=go1.27.2 go build -trimpath -o /tmp/my-branch-fit ./examples/branch-role-fit

/usr/bin/time -l /tmp/my-branch-export /path/to/meta-ontology-go /tmp/my-new-branch-sources
/usr/bin/time -l /tmp/my-branch-fit /tmp/my-new-branch-sources/dataset.json /tmp/my-new-branch-results
```

These commands make new observations. Preserve partial output if a command
fails; use a new destination for a separately declared experiment. The producer
refuses an existing output directory or mismatched source revisions. Exporting
does no training or model inference. The run contains two fixed fits, 584 judged
contexts and 300 bounded searches. Program/process RAM is measured separately
from the model's 24,872 weight bytes.
