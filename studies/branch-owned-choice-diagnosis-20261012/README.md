# Which decisions changed in the first V4 study?

This reader breaks down the [original paired study](../branch-owned-learning-20261012)
by the two choices already declared in each Gooo source. It loads three files at
[the immutable result revision](https://huggingface.co/asketeddy/gooo-canonical-decision-tiny-v1/tree/8d5c1c2310017ec5d147b77b891bce894aaa03e4/research/branch-owned-learning-20261012/result),
checks their SHA256 identities and joins every observation to its original source.
It requires exactly one acceptable mask per row before counting individual bits.
The reader loads no weights and calls no model, compiler, candidate or native
program. The original fits and observations remain unchanged.

## Held-out observations

| First choice observation | V3 prose-neutral | V4 branch-owned |
| --- | ---: | ---: |
| Correct comparison operand order | 216/384 (56.25%) | 164/384 (42.71%) |
| Correct return-branch placement | 168/384 (43.75%) | 192/384 (50.00%) |
| Both choices correct | 112/384 (29.17%) | 60/384 (15.63%) |
| Only comparison wrong | 56/384 | 132/384 |
| Only branch placement wrong | 104/384 | 104/384 |
| Both choices wrong | 112/384 | 88/384 |

The source's choice order is checked, rather than inferred from the mask's name:
bit 0 is `operand_order`, and bit 1 is `branch_layout`. Each acceptable full mask
appears 96 times; each acceptable choice bit is balanced 192/192. These are the
same 384 sibling rows from 24 contracts in four held-out operator families.

V4's decline in complete first choices accompanies worse comparison selection;
its branch-placement total increases to the balanced 50% baseline. More explicit
branch input did not translate into reliable whole-body decisions. This
decomposition describes the outcomes; it does not establish that the new input
channels caused the comparison errors.

## The saved ranking is nearly uniform

The mean largest candidate probability is 0.269655 for neutral and 0.271315 for
V4. Mean entropy is 1.996820 and 1.995816 bits; four equal probabilities have
entropy 2 bits. These are properties of the saved four-candidate rankings, not
calibrated probabilities of program correctness.

The existing [decision head](../../contractdecision/interaction_requirement_forward.go)
adds one logit per selected choice to score a full mask. With all four masks
present, this form factorizes across the two choices. The retained probabilities
satisfy `p(0)*p(3) = p(1)*p(2)` to within 1.2e-8 over every split/profile, consistent
with rounding of the original float32 probabilities. Factorization is a property
of this scoring form; it does not prove that the tasks require a different head.

## Next implementation question

Use Gooo's typed choice and dependency information to select what each small
decision head needs to see. Comparison orientation has declared Boolean cases;
branch placement also needs the return expressions and expected outputs. A fresh
experiment can compare the shared head with separate typed heads while holding
the parameter budget, source coverage and fit settings explicit. It should report
both per-choice errors and full finite completion, including a deterministic
baseline. This is a proposed comparison, not a trained successor model.

Retain the language's source-owned candidate palette and finite checks while
studying this change. A better ranking should reduce the checks needed to assemble
a body; its probability alone does not discharge the declared requirements.

## Run the retained reader

From the SDK repository root:

```sh
GOWORK=off GOTOOLCHAIN=go1.27.2 go test -race ./studies/branch-owned-choice-diagnosis-20261012/audit
bash studies/branch-owned-learning-20261012/check-retained.sh
```

The second command shares the existing three-file download, runs the original
retained audit, then compares this diagnosis with `diagnosis.json`. There is no
additional model download or collection. Standalone use is
`go run ./studies/branch-owned-choice-diagnosis-20261012/audit <result-directory>`.
The report includes all three splits, confusion tables, mask counts, descriptive
probability summaries and input identities. Synthetic tests cover all 16 pairs
of predicted/acceptable masks and reject ambiguous acceptance or invalid saved
probabilities. This is a saved-data diagnosis, not a new learning method or an
independent performance experiment.
