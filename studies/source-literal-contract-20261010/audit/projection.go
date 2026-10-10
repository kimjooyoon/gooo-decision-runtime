package main

import (
	"math/big"
	"reflect"
	"slices"

	decision "github.com/kimjooyoon/gooo-decision-runtime"
	r "github.com/kimjooyoon/gooo-decision-runtime/studies/contract-goals-20261010/record"
	q "github.com/kimjooyoon/gooo-decision-runtime/studies/source-literal-contract-20261010/record"
)

func caseVersion(mode string) string {
	if mode == "source_literals" {
		return decision.SourceLiteralCaseFeatureVersion
	}
	return decision.DeclaredCaseFeatureVersion
}

// The reference reads authored constants directly, without preparing a plan or
// using the production projection. Big integers check mathematical equality
// before requiring a representable int64 result.
func referenceProjection(source r.Source) q.Projection {
	p := q.Projection{ID: source.Spec.ID}
	for _, expression := range source.Document.Plan.Base.Expressions {
		if expression.Kind == "int" && !slices.Contains(p.Literals, expression.Int) {
			p.Literals = append(p.Literals, expression.Int)
		}
	}
	slices.Sort(p.Literals)
	for i, c := range source.Document.TestCases {
		row := source.CaseRows[i]
		slots := [...]int{2, 3, 4, 5, 6, 10, 11, 28, 29, 30, 31}
		row[1] = 0
		for _, index := range slots {
			row[index] = 0
		}
		if len(p.Literals) > 0 {
			row[1] = 0.125
		}
		var counts [11]int
		for _, k := range p.Literals {
			x, literal := big.NewInt(c.Input), big.NewInt(k)
			equal := func(value *big.Int) bool { return value.IsInt64() && value.Int64() == c.Expected }
			flags := [...]bool{c.Input == k, c.Input < k, c.Input > k,
				c.Expected == k, c.Expected < k, c.Expected > k,
				equal(new(big.Int).Add(x, literal)), equal(new(big.Int).Sub(x, literal)),
				equal(new(big.Int).Sub(literal, x)), equal(new(big.Int).Neg(literal)),
				equal(new(big.Int).Mul(x, literal))}
			for j, flag := range flags {
				if flag {
					counts[j]++
				}
			}
		}
		for j, count := range counts {
			if len(p.Literals) > 0 {
				row[slots[j]] = float32(count) / float32(8*len(p.Literals))
			}
		}
		p.Cases = append(p.Cases, row)
	}
	return p
}

func checkProjectionTraining(root string, sources []r.Source, training []r.Training, rep report) {
	raw := load(root, "projections.json.gz")
	consumedRaw := load(root, "literal-training.json.gz")
	need(r.Hash(raw) == rep.ProjectionSHA && r.Hash(consumedRaw) == rep.ConsumedSHA, "projection/training digests")
	checkProjections(sources, training, decode[[]q.Projection](raw), decode[[]r.Training](consumedRaw))
}

func checkProjections(sources []r.Source, training []r.Training, projections []q.Projection, consumed []r.Training) {
	need(len(projections) == 196 && len(sources) == 196 && len(training) == 24 && len(consumed) == 24, "all projections and training rows")
	byID := map[string]q.Projection{}
	for i, source := range sources {
		want := referenceProjection(source)
		need(reflect.DeepEqual(projections[i], want), "complete source-relative case projection")
		need(byID[want.ID].ID == "", "unique projection")
		byID[want.ID] = want
	}
	for i, row := range training {
		projection, ok := byID[row.ID]
		need(ok, "training projection membership")
		row.Cases = projection.Cases
		need(reflect.DeepEqual(row, consumed[i]), "only case features differ in consumed original training rows")
	}
}
