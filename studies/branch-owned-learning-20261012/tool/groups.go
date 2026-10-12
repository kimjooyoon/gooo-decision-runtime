package main

import "fmt"

type groupStats struct {
	Documents       int `json:"documents"`
	FirstComplete   int `json:"first_complete"`
	CheckedComplete int `json:"checked_complete"`
	Attempts        int `json:"candidate_checks"`
}

type contractStats struct {
	Contracts              int `json:"contracts"`
	CompleteAcrossSiblings int `json:"first_complete_across_all_siblings"`
	SiblingsPerContract    int `json:"siblings_per_contract"`
}

func groupedOutcomes(rows []observation) (map[string]groupStats, map[string]contractStats) {
	groups := map[string]groupStats{}
	contracts := map[string][]observation{}
	for _, row := range rows {
		contracts[row.Split+"/"+row.Contract] = append(contracts[row.Split+"/"+row.Contract], row)
		if row.Split != "evaluation" {
			continue
		}
		keys := []string{fmt.Sprintf("family/%d", row.Family), fmt.Sprintf("literal_regime/%d", row.Parameter), fmt.Sprintf("goal/%d", row.Goal), fmt.Sprintf("layout/%d", row.Parity), fmt.Sprintf("form/%d", row.Form)}
		for _, key := range keys {
			s := groups[key]
			s.Documents++
			s.Attempts += row.Attempts
			if row.First.Complete {
				s.FirstComplete++
			}
			if row.Checked.Complete {
				s.CheckedComplete++
			}
			groups[key] = s
		}
	}
	result := map[string]contractStats{}
	for _, siblings := range contracts {
		split := siblings[0].Split
		s := result[split]
		s.Contracts++
		s.SiblingsPerContract = 16
		complete := len(siblings) == 16
		for _, r := range siblings {
			complete = complete && r.First.Complete
		}
		if complete {
			s.CompleteAcrossSiblings++
		}
		result[split] = s
	}
	return groups, result
}
