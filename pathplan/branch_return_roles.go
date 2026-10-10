package pathplan

import (
	decision "github.com/kimjooyoon/gooo-decision-runtime"
	"github.com/kimjooyoon/gooo-decision-runtime/bodyplan"
)

// BranchReturnRoles describes syntactic returns in a fallback-normalized branch.
// Each arm has input/literal/local/composed flags, local initializer input/literal
// flags, first/last declaration flags, an assigned-anywhere flag and a return
// count. Initializer facts do not describe the local's value after assignments.
// Nested returns are visited once per statement in each arm. Other choice kinds
// have zero roles. No cases, expected outputs, model or selected masks are read.
func (prepared *PreparedPlan) BranchReturnRoles(id string) ([decision.BranchReturnRoleDim]byte, error) {
	var roles [decision.BranchReturnRoleDim]byte
	// Keep v1 reachability, shadowing and declaration-ambiguity checks identical.
	if _, err := prepared.SourceFeatures(id); err != nil {
		return roles, err
	}
	for _, choice := range prepared.plan.Decisions {
		if choice.ID != id || choice.Kind != BranchLayout {
			continue
		}
		var arena sourceFeatureArena
		arena.normalize(prepared.plan)
		for _, index := range arena.roots[:arena.rootCount] {
			arena.markStatement(index)
		}
		branch := arena.statements[choice.Target]
		for arm, sequence := range [2][]int{branch.Then, branch.Else} {
			var visited [128]bool
			var fields [10]byte
			count := arena.returnRoles(sequence, &visited, &fields)
			fields[9] = sourceCount(count)
			copy(roles[arm*10:], fields[:])
		}
	}
	return roles, nil
}

func (a *sourceFeatureArena) returnRoles(sequence []int, visited *[128]bool, fields *[10]byte) int {
	count := 0
	for _, index := range sequence {
		if visited[index] {
			continue
		}
		visited[index] = true
		s := a.statements[index]
		switch s.Kind {
		case bodyplan.StmtIf:
			count += a.returnRoles(s.Then, visited, fields) + a.returnRoles(s.Else, visited, fields)
		case bodyplan.StmtReturn:
			count++
			a.returnExpressionRoles(a.expressions[s.Expr], fields)
		}
	}
	return count
}

func (a *sourceFeatureArena) returnExpressionRoles(e bodyplan.Expr, fields *[10]byte) {
	switch e.Kind {
	case bodyplan.ExprInput:
		fields[0] = 128
	case bodyplan.ExprInt, bodyplan.ExprBool:
		fields[1] = 128
	case bodyplan.ExprBinary, bodyplan.ExprHole:
		fields[3] = 128
	case bodyplan.ExprLocal:
		fields[2] = 128
		if declaration, rank := a.declaration(e.Name); declaration >= 0 {
			initializer := a.expressions[a.statements[declaration].Expr]
			fields[4] |= sourceFlag(initializer.Kind == bodyplan.ExprInput)
			fields[5] |= sourceFlag(initializer.Kind == bodyplan.ExprInt || initializer.Kind == bodyplan.ExprBool)
			fields[6] |= sourceFlag(rank == 0)
			fields[7] |= sourceFlag(rank == a.declarationCount-1)
		}
		for i, s := range a.statements[:a.statementCount] {
			if a.statementReachable[i] && s.Kind == bodyplan.StmtAssign && s.Name == e.Name {
				fields[8] = 128
			}
		}
	}
}
