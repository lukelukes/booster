package run

import (
	"booster/internal/action"
	"booster/internal/model"
	"context"
	"fmt"
)

type Planned struct {
	model.PlannedStep
	Action action.Action
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
		result.Steps = append(result.Steps, Planned{
			PlannedStep: model.PlannedStep{Name: step.Name, Change: change},
			Action:      step.Action,
		})
	}

	return result, nil
}

func (p PlanResult) View() model.Plan {
	view := model.Plan{Steps: make([]model.PlannedStep, len(p.Steps)), Excluded: p.Excluded}
	for i, s := range p.Steps {
		view.Steps[i] = s.PlannedStep
	}
	return view
}
