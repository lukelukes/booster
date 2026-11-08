package run

import (
	"booster/internal/action"
	"booster/internal/config"
	"booster/internal/expr"
	"errors"
	"fmt"
)

type Step struct {
	Name   string
	Action action.Action
}

type Compiled struct {
	Steps    []Step
	Excluded int
}

func Compile(tasks []config.Task, ctx *expr.Context, registry map[string]action.Factory) (Compiled, error) {
	var compiled Compiled
	var errs []error

	for i, task := range tasks {
		name := task.Name
		if name == "" {
			name = task.Action
		}

		step, include, err := compileTask(task, name, ctx, registry)
		if err != nil {
			errs = append(errs, fmt.Errorf("task %d %q: %w", i+1, name, err))
			continue
		}
		if !include {
			compiled.Excluded++
			continue
		}
		compiled.Steps = append(compiled.Steps, step)
	}

	if len(errs) > 0 {
		return Compiled{}, errors.Join(errs...)
	}

	return compiled, nil
}

func compileTask(task config.Task, name string, ctx *expr.Context, registry map[string]action.Factory) (Step, bool, error) {
	factory, ok := registry[task.Action]
	if !ok {
		return Step{}, false, fmt.Errorf("unknown action %q", task.Action)
	}

	include, err := evalWhen(task.When, ctx)
	if err != nil || !include {
		return Step{}, false, err
	}

	act, err := factory(task.Args, ctx)
	if errors.Is(err, action.ErrNotApplicable) {
		return Step{}, false, nil
	}
	if err != nil {
		return Step{}, false, fmt.Errorf("%s: %w", task.Action, err)
	}

	return Step{Name: name, Action: act}, true, nil
}

func evalWhen(when *config.WhenExpr, ctx *expr.Context) (bool, error) {
	if when == nil {
		return true, nil
	}

	value, err := expr.NewValue(string(*when))
	if err != nil {
		return false, fmt.Errorf("when: %w", err)
	}

	include, err := expr.ResolveCondition(value, ctx)
	if err != nil {
		return false, fmt.Errorf("when: %w", err)
	}

	return include, nil
}
