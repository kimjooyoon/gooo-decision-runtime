package bodyplan

// A source binding may remain unread in a selected candidate. Initialization
// and later writes still execute. Only Go emission needs a no-op read marker.
// Resolve reads by validated slot identity so sibling scopes can reuse names.
func unreadLocalStatements(plan Plan, validated *validatedPlan) [maxStmts]bool {
	var reads, unread [maxStmts]bool
	for _, encoded := range validated.expressionSlots {
		if encoded != 0 {
			reads[encoded-1] = true
		}
	}
	for index, statement := range plan.Statements {
		if statement.Kind == StmtLet {
			unread[index] = !reads[validated.statementSlots[index]-1]
		}
	}
	return unread
}
