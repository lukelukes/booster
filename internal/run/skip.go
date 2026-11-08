package run

import "slices"

func (p PlanResult) Without(skip []int) PlanResult {
	kept := PlanResult{Steps: make([]Planned, 0, len(p.Steps)), Excluded: p.Excluded}
	for i, s := range p.Steps {
		if !slices.Contains(skip, i) {
			kept.Steps = append(kept.Steps, s)
		}
	}
	return kept
}
