package task

import (
	"booster/internal/config"
	"booster/internal/expr"
	"context"
	"errors"
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
	exprCtx   *expr.Context
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

func (b *Builder) WithExprContext(ctx *expr.Context) *Builder {
	b.exprCtx = ctx
	return b
}

func (b *Builder) Build(tasks []config.Task) ([]Task, error) {
	var result []Task

	for i, ct := range tasks {
		factory, ok := b.factories[ct.Action]
		if !ok {
			return nil, fmt.Errorf("task %d: unknown action %q", i+1, ct.Action)
		}

		if ct.When != nil {
			if b.exprCtx == nil {
				return nil, fmt.Errorf("task %d (%s): invalid when %q: expression context cannot be nil", i+1, ct.Action, formatWhenForMessage(string(*ct.When)))
			}

			whenExpr := string(*ct.When)
			whenValue, err := expr.NewValue(whenExpr)
			if err != nil {
				return nil, fmt.Errorf("task %d (%s): invalid when %q: %w", i+1, ct.Action, formatWhenForMessage(whenExpr), err)
			}

			lazy := NewDeferredFactoryTask(ct.Action, ct.Args, factory, b.exprCtx, i+1)
			conditional, err := NewConditionalTask(lazy, whenValue, b.exprCtx, whenExpr)
			if err != nil {
				return nil, fmt.Errorf("task %d (%s): invalid when %q: %w", i+1, ct.Action, formatWhenForMessage(whenExpr), err)
			}
			result = append(result, conditional)
			continue
		}

		resolvedArgs, err := resolveTaskArgs(ct.Args, b.exprCtx)
		if err != nil {
			var argErr *argResolveError
			if errors.As(err, &argErr) {
				return nil, fmt.Errorf("task %d (%s): args%s: %v", i+1, ct.Action, argErr.path, argErr.err)
			}
			return nil, fmt.Errorf("task %d (%s): args: %w", i+1, ct.Action, err)
		}

		created, err := factory(resolvedArgs)
		if err != nil {
			return nil, fmt.Errorf("task %d (%s): %w", i+1, ct.Action, err)
		}

		result = append(result, created...)
	}

	return result, nil
}

func DefaultBuilder(ctx *expr.Context) *Builder {
	return NewBuilder().
		WithExprContext(ctx).
		Register("dir.create", NewDirCreate).
		Register("symlink.create", NewSymlinkCreate)
}
