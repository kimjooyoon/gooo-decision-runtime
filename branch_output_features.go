package decision

// Branch output relations preserve the owner of each return's literal set.
// Blocks are then/else, each with less/equal/greater predicate regions and the
// same 16 relationship slots as SourceRelativeOutputFeatureVersion.
const BranchOutputFeatureVersion = "canonical_branch_owned_integer_output_relations_v1"
const BranchOutputFeatureDim = 96
