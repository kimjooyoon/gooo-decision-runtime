# Does case pooling affect small Gooo decision models?

Status: source and protocol frozen; **no original fit or study run**.

The previous canonical study's model fit only 18/64 training documents on its
first choice. A read-only audit found distinguishable full saved inputs, while
training loss changed little late in its fixed recipe. That evidence does not
identify the cause. This follow-up compares two existing ways of combining case
features, with a new frozen set of constants and the same small model size.

Both models read Gooo source structure, authored outputs and intermediate
conditions. Each has 19,034 FP32 parameters (76,136 tensor bytes), runs in Go and
proposes a finite path order. Gooo checks the proposed candidates. Fixed ordering
is included as a comparator. No text generation is involved.

| Cohort | Documents | Role |
| --- | ---: | --- |
| Bound/distance, direct return | 64 | Both fresh fits |
| New constants, direct return/assignment | 128 | Related-template evaluation |
| Conditional addition/multiplication | 64 | Family excluded from both fits |

The known templates and Korean/English descriptions are reused explicitly.
These 256 documents are related variants, not independent business tasks.
Mean pooling averages learned case activations; extreme pooling retains the
largest absolute activation per channel. Training starts identically and can
produce different weights. Results will compare those complete recipes.

The [protocol](protocol.txt) fixes both fits, all evaluations, first-choice and
final checks, costs, failure retention and the one-run boundary. Source/oracle
material is written before any candidate collection or prediction. No result,
model quality improvement, deployment change or paid adoption is claimed here.

The compressed corpus SHA256 is
`9bde751be7f8c39cb7084868db41a3c372d10923b825e9bf682960a6c51d23c5`.
Local SDK unit/static checks and producer unit/static/race checks passed. Corpus
preflight read all 256 sources and matched 96 direct/assignment input pairs;
model-routing regressions used only the separate K=5 zero-weight fixture.

The producer is a separate Go module because it uses the compiler's source
lowering adapter. Its local replacements require the two named clean checkouts;
the experiment checks their recorded identities. Regression tests use a K=5
fixture outside the study corpus. A read-only result audit will be added after
the original execution, without running the fits or models again.
