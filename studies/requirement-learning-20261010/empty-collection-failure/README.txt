Collector error before either model fit

Producer52c38312bb2172d1d2a02dc48b3498891e0c3cc6; compiler0d61324996d7723f4c8109cbb8508de8de3cb72c.
Frozen corpus6e1b0c73c7d539f69953af0658005b8d0609bc4054e30cf57cb8ce766bae6b63.
164 complete source records were collected. The collector then called the
nonempty CheckConditions API for source165's intentionally empty condition
suite. That API requires1..128 cases. No fit, prediction, training.json or model
artifact was produced. The source corpus and training settings remain fixed.
The collector now omits that observation call when there are no conditions,
retains all eight output checks, and counts its actual oracle compile calls.
The public failure excerpt omits local stack paths and addresses. Original
partial records and complete process timing fields are retained. A focused
collector regression covers all four empty-suite source variants.
