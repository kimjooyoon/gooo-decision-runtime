package record

import (
	"math"
	"sort"

	r "github.com/kimjooyoon/gooo-decision-runtime/studies/contract-goals-20261010/record"
)

func Summarize(rows []Row, sources []r.Source) Summary {
	byID := map[string]r.Source{}
	for _, source := range sources {
		byID[source.Spec.ID] = source
	}
	paired := map[string][]Row{}
	for _, row := range rows {
		source, ok := byID[row.ID]
		Need(ok, "source identity")
		if source.Acceptable != 0 {
			key := row.Mode + "/" + source.Spec.Pair
			paired[key] = append(paired[key], row)
		}
	}
	var keys []string
	for key := range paired {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := Summary{Groups: map[string]Group{}}
	for _, key := range keys {
		pair := paired[key]
		Need(len(pair) == 2, "two goals per pair")
		a, b := pair[0], pair[1]
		x, y := byID[a.ID], byID[b.ID]
		Need(x.Spec.Goal != y.Spec.Goal && x.Spec.Split == y.Spec.Split && x.Acceptable&y.Acceptable == 0, "opposite disjoint goals")
		p := Pair{ID: x.Spec.Pair, Mode: a.Mode, Split: x.Spec.Split, Sources: [2]string{a.ID, b.ID},
			PoolEqual: a.Trace.Pool == b.Trace.Pool, SourceEqual: a.Trace.SourcePrefix == b.Trace.SourcePrefix,
			JointEqual: a.Trace.Joint == b.Trace.Joint, HiddenEqual: a.Trace.Hidden == b.Trace.Hidden,
			OptionsEqual: a.Trace.OptionScores == b.Trace.OptionScores, SameProposal: a.Prediction.Selected == b.Prediction.Selected,
			BothValid:   x.Acceptable>>a.Prediction.Selected&1 != 0 && y.Acceptable>>b.Prediction.Selected&1 != 0,
			BothInvalid: x.Acceptable>>a.Prediction.Selected&1 == 0 && y.Acceptable>>b.Prediction.Selected&1 == 0}
		for choice := range a.Trace.ChoiceCount {
			for h := range a.Trace.Joint[choice] {
				p.TotalPairedUnits++
				if (a.Trace.Joint[choice][h] > 0) != (b.Trace.Joint[choice][h] > 0) {
					p.CrossedUnits++
				}
				for _, row := range []*Row{&a, &b} {
					prefix := float64(row.Trace.SourcePrefix[choice][h])
					suffix := float64(row.Trace.Joint[choice][h]) - prefix
					p.Units++
					if math.Abs(prefix) > math.Abs(suffix) {
						p.SourceDominated++
					}
				}
			}
			p.Margins[0][choice] = float64(a.Trace.OptionScores[choice][1]) - float64(a.Trace.OptionScores[choice][0])
			p.Margins[1][choice] = float64(b.Trace.OptionScores[choice][1]) - float64(b.Trace.OptionScores[choice][0])
		}
		result.Pairs = append(result.Pairs, p)
		for _, split := range []string{"all", p.Split} {
			key := p.Mode + "/" + split
			g := result.Groups[key]
			g.Pairs++
			g.CrossedUnits += p.CrossedUnits
			g.TotalPairedUnits += p.TotalPairedUnits
			g.SourceDominated += p.SourceDominated
			g.Units += p.Units
			for _, field := range []struct {
				value bool
				count *int
			}{
				{p.PoolEqual, &g.PoolEqual}, {p.SourceEqual, &g.SourceEqual}, {p.JointEqual, &g.JointEqual},
				{p.HiddenEqual, &g.HiddenEqual}, {p.OptionsEqual, &g.OptionsEqual}, {p.SameProposal, &g.SameProposal},
				{p.BothValid, &g.BothValid}, {p.BothInvalid, &g.BothInvalid},
			} {
				if field.value {
					*field.count++
				}
			}
			result.Groups[key] = g
		}
	}
	return result
}
