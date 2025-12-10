package task

import (
	"booster/internal/condition"
	"context"
	"errors"
)

// ConditionalTask wraps a task with a condition that must be met for execution.
type ConditionalTask struct {
	wrapped   Task
	condition *condition.Condition
	evaluator *condition.Evaluator
}

// NewConditionalTask creates a task that only executes when the condition matches.
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

// Name returns the wrapped task's name.
func (t *ConditionalTask) Name() string {
	return t.wrapped.Name()
}

// NeedsSudo delegates to the wrapped task.
func (t *ConditionalTask) NeedsSudo() bool {
	return t.wrapped.NeedsSudo()
}

// Run executes the task if the condition matches, otherwise returns StatusSkipped.
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
