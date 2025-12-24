package task

import (
	"booster/internal/condition"
	"context"
	"errors"
)

type ConditionalTask struct {
	wrapped   Task
	condition *condition.Condition
	evaluator *condition.Evaluator
}

func NewConditionalTask(t Task, cond *condition.Condition, eval *condition.Evaluator) (*ConditionalTask, error) {
	if eval == nil {
		return nil, errors.New("evaluator cannot be nil")
	}
	return &ConditionalTask{
		wrapped:   t,
		condition: cond,
		evaluator: eval,
	}, nil
}

func (t *ConditionalTask) Name() string {
	return t.wrapped.Name()
}

func (t *ConditionalTask) NeedsSudo() bool {
	return t.wrapped.NeedsSudo()
}

func (t *ConditionalTask) Run(ctx context.Context) Result {
	if !t.evaluator.Matches(t.condition) {
		reason := t.evaluator.FailureReason(t.condition)
		return Result{
			Status:  StatusSkipped,
			Message: "condition not met: " + reason,
		}
	}
	return t.wrapped.Run(ctx)
}
