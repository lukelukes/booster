package task

import (
	"booster/internal/condition"
	"booster/internal/config"
	"context"
	"fmt"
	"time"
)

type Status int

const (
	StatusPending Status = iota
	StatusRunning
	StatusSkipped
	StatusDone
	StatusFailed
)

type Result struct {
	Error    error
	Message  string
	Output   string
	Status   Status
	Duration time.Duration
}

type Task interface {
	Name() string

	Run(ctx context.Context) Result

	NeedsSudo() bool
}

type Factory func(args any) ([]Task, error)

func AnyNeedsSudo(tasks []Task) bool {
	for _, t := range tasks {
		if t.NeedsSudo() {
			return true
		}
	}
	return false
}

type Builder struct {
	factories map[string]Factory
	evaluator *condition.Evaluator
}

func NewBuilder() *Builder {
	return &Builder{
		factories: make(map[string]Factory),
	}
}

func (b *Builder) Register(action string, factory Factory) *Builder {
	b.factories[action] = factory
	return b
}

func (b *Builder) WithEvaluator(eval *condition.Evaluator) *Builder {
	b.evaluator = eval
	return b
}

func (b *Builder) Build(tasks []config.Task) ([]Task, error) {
	var result []Task

	for i, ct := range tasks {
		factory, ok := b.factories[ct.Action]
		if !ok {
			return nil, fmt.Errorf("task %d: unknown action %q", i+1, ct.Action)
		}

		created, err := factory(ct.Args)
		if err != nil {
			return nil, fmt.Errorf("task %d (%s): %w", i+1, ct.Action, err)
		}

		for _, t := range created {
			if b.evaluator != nil && ct.When != nil {
				cond := &condition.Condition{
					OS:      ct.When.OS,
					Profile: ct.When.Profile,
				}
				wrapped, err := NewConditionalTask(t, cond, b.evaluator)
				if err != nil {
					return nil, fmt.Errorf("task %d (%s): %w", i+1, ct.Action, err)
				}
				t = wrapped
			}
			result = append(result, t)
		}
	}

	return result, nil
}

func DefaultBuilder(ctx condition.Context) *Builder {
	eval := condition.NewEvaluator(ctx)

	return NewBuilder().
		WithEvaluator(eval).
		Register("dir.create", NewDirCreate).
		Register("symlink.create", NewSymlinkCreate)
}
