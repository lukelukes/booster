// Package task defines the task execution interface and builder.
package task

import (
	"booster/internal/condition"
	"booster/internal/config"
	"context"
	"fmt"
	"time"
)

// Status represents the outcome of a task execution.
type Status int

const (
	StatusPending Status = iota
	StatusRunning
	StatusSkipped // Already in desired state (idempotent)
	StatusDone
	StatusFailed
)

// Result holds the outcome of executing a task.
type Result struct {
	Error    error
	Message  string
	Output   string // Command output (stdout/stderr) if any
	Status   Status
	Duration time.Duration // How long the task took to execute
}

// Task is a single executable action.
type Task interface {
	// Name returns a human-readable description for display.
	Name() string

	// Run executes the task. It should be idempotent.
	// Returns StatusSkipped if already in desired state.
	Run(ctx context.Context) Result

	// NeedsSudo returns true if this task requires elevated privileges.
	// This is used to prompt for sudo credentials before the TUI starts.
	NeedsSudo() bool
}

// Factory creates Task instances from raw args.
type Factory func(args any) ([]Task, error)

// AnyNeedsSudo returns true if any task in the slice requires sudo.
func AnyNeedsSudo(tasks []Task) bool {
	for _, t := range tasks {
		if t.NeedsSudo() {
			return true
		}
	}
	return false
}

// Builder creates executable tasks from configuration.
// Use NewBuilder to create an instance, then register factories with Register.
type Builder struct {
	factories map[string]Factory
	evaluator *condition.Evaluator
}

// NewBuilder creates a new Builder with an empty factory registry.
func NewBuilder() *Builder {
	return &Builder{
		factories: make(map[string]Factory),
	}
}

// Register adds a task factory for the given action name.
func (b *Builder) Register(action string, factory Factory) *Builder {
	b.factories[action] = factory
	return b
}

// WithEvaluator sets the condition evaluator for the builder.
func (b *Builder) WithEvaluator(eval *condition.Evaluator) *Builder {
	b.evaluator = eval
	return b
}

// Build converts config tasks to executable tasks.
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

		// Wrap with conditions if evaluator is set and task has When
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

// DefaultBuilder returns a Builder with all standard task factories registered
// and condition evaluation enabled for the given context.
func DefaultBuilder(ctx condition.Context) *Builder {
	eval := condition.NewEvaluator(ctx)

	return NewBuilder().
		WithEvaluator(eval).
		Register("dir.create", NewDirCreate).
		Register("symlink.create", NewSymlinkCreate)
	// Note: git.config is registered in main.go with runner/prompter dependencies
}
