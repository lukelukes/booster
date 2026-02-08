package task

import (
	"booster/internal/expr"
	"context"
	"errors"
	"fmt"
)

type DeferredFactoryTask struct {
	action    string
	args      any
	factory   Factory
	exprCtx   *expr.Context
	taskIndex int

	initialized bool
	created     []Task
	initErr     error
}

func NewDeferredFactoryTask(action string, args any, factory Factory, exprCtx *expr.Context, taskIndex int) *DeferredFactoryTask {
	return &DeferredFactoryTask{
		action:    action,
		args:      args,
		factory:   factory,
		exprCtx:   exprCtx,
		taskIndex: taskIndex,
	}
}

func (t *DeferredFactoryTask) Name() string {
	if t.initialized && len(t.created) == 1 {
		return t.created[0].Name()
	}
	return t.action
}

func (t *DeferredFactoryTask) NeedsSudo() bool {
	if !t.initialized {
		t.init()
	}
	if t.initErr != nil {
		return true
	}
	return AnyNeedsSudo(t.created)
}

func (t *DeferredFactoryTask) Run(ctx context.Context) Result {
	t.init()
	if t.initErr != nil {
		return Result{Status: StatusFailed, Error: t.initErr, Message: t.initErr.Error()}
	}

	if len(t.created) == 0 {
		return Result{Status: StatusDone}
	}

	if len(t.created) == 1 {
		return t.created[0].Run(ctx)
	}

	var doneCount int
	for i, created := range t.created {
		result := created.Run(ctx)
		switch result.Status {
		case StatusFailed:
			if result.Message == "" {
				result.Message = fmt.Sprintf("generated task %d for action %q failed", i+1, t.action)
			}
			return result
		case StatusDone:
			doneCount++
		}
	}

	if doneCount == 0 {
		return Result{Status: StatusSkipped, Message: fmt.Sprintf("all generated tasks skipped for action %q", t.action)}
	}

	return Result{Status: StatusDone, Message: fmt.Sprintf("executed %d generated task(s) for action %q", doneCount, t.action)}
}

func (t *DeferredFactoryTask) init() {
	if t.initialized {
		return
	}
	t.initialized = true

	resolvedArgs, err := resolveTaskArgs(t.args, t.exprCtx)
	if err != nil {
		var argErr *argResolveError
		if errors.As(err, &argErr) {
			t.initErr = fmt.Errorf("task %d (%s): args%s: %v", t.taskIndex, t.action, argErr.path, argErr.err)
			return
		}
		t.initErr = fmt.Errorf("task %d (%s): args: %w", t.taskIndex, t.action, err)
		return
	}

	created, err := t.factory(resolvedArgs)
	if err != nil {
		t.initErr = fmt.Errorf("task %d (%s): %w", t.taskIndex, t.action, err)
		return
	}
	t.created = created
}
