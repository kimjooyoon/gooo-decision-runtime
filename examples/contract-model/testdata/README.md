# Small V2 input example

`source-relative-v2-division.json` is an authored SDK test document. Its single
branch-layout choice selects the return placement for input<13. Three output
cases and one Boolean condition are requirements. The companion Gooo file shows
the intended computation. The JSON was authored from the SDK fixture constructor;
it does not carry a compiler-produced source-binding receipt.

This example can be inspected without a model:

```sh
GOWORK=off GOTOOLCHAIN=go1.27.3 go run ./examples/contract-model inspect \
  -source-relative-v2-source examples/contract-model/testdata/source-relative-v2-division.json
```

The modified Go example dispatches an explicitly supplied V2 artifact to its V2
encoder and one-prediction session. It does not create code text. The source
requirements remain the finite acceptance criteria. Omitting the model makes
selection deterministic. This source example addition follows SDK0.2.42.
