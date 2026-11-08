package run

import (
	"booster/internal/model"
	"context"
	"fmt"
)

type Planned struct {
	Step   Step
	Change model.Change
}

type PlanResult struct {
	Steps    []Planned
	Excluded int
}

func Plan(ctx context.Context, compiled Compiled) (PlanResult, error) {
	result := PlanResult{Steps: make([]Planned, 0, len(compiled.Steps)), Excluded: compiled.Excluded}

	for _, step := range compiled.Steps {
		change, err := step.Action.Check(ctx)
		if err != nil {
			return PlanResult{}, fmt.Errorf("check %q: %w", step.Name, err)
		}
		result.Steps = append(result.Steps, Planned{Step: step, Change: change})
	}

	return result, nil
}

func (p PlanResult) View() model.Plan {
	view := model.Plan{Steps: make([]model.PlannedStep, 0, len(p.Steps)), Excluded: p.Excluded}
	for _, s := range p.Steps {
		view.Steps = append(view.Steps, model.PlannedStep{Name: s.Step.Name, Change: s.Change})
	}
	return view
}
