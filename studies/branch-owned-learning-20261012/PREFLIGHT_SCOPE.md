# Preflight scope clarification, before original collection

The original preregistration is retained at revision
c520e47d5e68ac44cd0ad0326b3aba711a226fbf.

The sentence "All signed literals passed a parse/array-only preflight" in
prefit/PROTOCOL.md is too broad. The published preflight test parsed and encoded
three documents: family0, goal0, Korean, layout0, assignment form, one for each
literal regime. That includes the negative literal pair -17/-29. It did not
preflight every signed pair or every document. The separate packer checked all
1536 source hashes and factor combinations without calling the compiler.

Original collection will parse and encode every document as planned and stop on
any unsupported input. No row is silently excluded. This clarification changes
no source, split, fit setting, candidate oracle or measurement obligation.
Collection, fitting and prediction have not started at this publication.
