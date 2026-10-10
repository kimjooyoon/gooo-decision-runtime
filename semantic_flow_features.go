package decision

// SemanticFlowFeatureVersion names a separately trained 384-cell ABI. A
// source-only single-branch analysis can normalize direct/copy/assignment forms.
// Ineligible source keeps the v4 array. Cell255=2 marks normalized inputs; v4
// branch-role counts in that cell are at most1. This is not a correctness proof.
const SemanticFlowFeatureVersion = "source_intent_condition_output_semantic_flow_v5"
