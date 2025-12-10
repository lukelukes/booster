// Package executor handles sequential task execution with state tracking.
package executor

import (
	"booster/internal/task"
	"context"
)

// Summary holds aggregate statistics after execution.
type Summary struct {
	Done        int
	Skipped     int
	Failed      int
	HasFailures bool
}

// Executor runs tasks sequentially and tracks their results.
// An Executor is NOT safe for concurrent use from multiple goroutines.
// Create separate Executor instances for concurrent task execution.
type Executor struct {
	tasks   []task.Task
	results []task.Result
	current int
}

// New creates an Executor for the given tasks.
func New(tasks []task.Task) *Executor {
	results := make([]task.Result, len(tasks))
	for i := range results {
		results[i] = task.Result{Status: task.StatusPending}
	}
	return &Executor{
		tasks:   tasks,
		results: results,
	}
}

// Total returns the number of tasks.
func (e *Executor) Total() int {
	return len(e.tasks)
}

// Current returns the index of the next task to run (0-indexed).
func (e *Executor) Current() int {
	return e.current
}

// Done returns true if all tasks have been executed.
func (e *Executor) Done() bool {
	return e.current >= len(e.tasks)
}

// Tasks returns the list of tasks.
func (e *Executor) Tasks() []task.Task {
	return e.tasks
}

// Results returns all results (pending tasks have StatusPending).
func (e *Executor) Results() []task.Result {
	return e.results
}

// ResultAt returns the result for task at index i.
func (e *Executor) ResultAt(i int) task.Result {
	if i < 0 || i >= len(e.results) {
		return task.Result{Status: task.StatusPending}
	}
	return e.results[i]
}

// RunNext executes the next pending task and returns its result.
// Returns false if no tasks remain.
// RunNext is NOT safe for concurrent calls; use from a single goroutine only.
func (e *Executor) RunNext(ctx context.Context) (task.Result, bool) {
	if e.Done() {
		return task.Result{}, false
	}

	t := e.tasks[e.current]
	result := t.Run(ctx)
	e.results[e.current] = result
	e.current++

	return result, true
}

// Summary returns aggregate statistics for all executed tasks.
func (e *Executor) Summary() Summary {
	var s Summary
	for _, r := range e.results {
		switch r.Status {
		case task.StatusDone:
			s.Done++
		case task.StatusSkipped:
			s.Skipped++
		case task.StatusFailed:
			s.Failed++
		}
	}
	s.HasFailures = s.Failed > 0
	return s
}
