package task

import (
	"booster/internal/expr"
	"context"
	"errors"
	"fmt"
	"strings"
)

type ConditionalTask struct {
	wrapped Task
	when    *expr.Value
	ctx     *expr.Context
	rawWhen string
}

func NewConditionalTask(t Task, when *expr.Value, ctx *expr.Context, rawWhen string) (*ConditionalTask, error) {
	if when == nil {
		return nil, errors.New("when expression cannot be nil")
	}
	if ctx == nil {
		return nil, errors.New("expression context cannot be nil")
	}
	return &ConditionalTask{
		wrapped: t,
		when:    when,
		ctx:     ctx,
		rawWhen: rawWhen,
	}, nil
}

func (t *ConditionalTask) Name() string {
	return t.wrapped.Name()
}

func (t *ConditionalTask) NeedsSudo() bool {
	shouldRun, err := expr.ResolveCondition(t.when, t.ctx)
	if err != nil {
		return true
	}
	if !shouldRun {
		return false
	}
	return t.wrapped.NeedsSudo()
}

func (t *ConditionalTask) Run(ctx context.Context) Result {
	shouldRun, err := expr.ResolveCondition(t.when, t.ctx)
	if err != nil {
		return Result{
			Status: StatusFailed,
			Error:  err,
			Message: fmt.Sprintf(
				"condition evaluation failed for when %q: %v",
				formatWhenForMessage(t.rawWhen),
				err,
			),
		}
	}
	if !shouldRun {
		return Result{
			Status:  StatusSkipped,
			Message: fmt.Sprintf("condition not met: when %q evaluated to false", formatWhenForMessage(t.rawWhen)),
		}
	}
	return t.wrapped.Run(ctx)
}

const maxWhenMessageLen = 120

func formatWhenForMessage(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) <= maxWhenMessageLen {
		return trimmed
	}
	return trimmed[:maxWhenMessageLen-3] + "..."
}
