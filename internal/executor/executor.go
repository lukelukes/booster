// Package executor handles sequential task execution with state tracking.
package executor

import (
	"booster/internal/task"
	"context"
	"time"
)

// Summary holds aggregate statistics after execution.
type Summary struct {
	Done        int
	Skipped     int
	Failed      int
	Pending     int
	HasFailures bool
}

// Executor runs tasks sequentially and tracks their results.
// An Executor is NOT safe for concurrent use from multiple goroutines.
// Create separate Executor instances for concurrent task execution.
type Executor struct {
	tasks     []task.Task
	results   []task.Result
	current   int
	aborted   bool
	startTime time.Time
	endTime   time.Time // Frozen when execution stops
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

// Abort stops execution early. Subsequent calls to RunNext will return false.
func (e *Executor) Abort() {
	if !e.aborted {
		e.aborted = true
		e.endTime = time.Now()
	}
}

// Stopped returns true if execution has stopped, either by completing all tasks
// or by being aborted early (e.g., due to a task failure).
func (e *Executor) Stopped() bool {
	return e.aborted || e.Done()
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
// Returns false if no tasks remain or execution was aborted.
// RunNext is NOT safe for concurrent calls; use from a single goroutine only.
func (e *Executor) RunNext(ctx context.Context) (task.Result, bool) {
	if e.Stopped() {
		return task.Result{}, false
	}

	// Record start time on first task
	if e.current == 0 {
		e.startTime = time.Now()
	}

	t := e.tasks[e.current]
	taskStart := time.Now()
	result := t.Run(ctx)
	result.Duration = time.Since(taskStart)
	e.results[e.current] = result
	e.current++

	// Freeze end time when all tasks complete
	if e.Done() {
		e.endTime = time.Now()
	}

	return result, true
}

// ElapsedTime returns the total time elapsed since execution started.
// Returns 0 if no tasks have been executed yet.
// When execution has stopped (completed or aborted), returns the frozen duration.
func (e *Executor) ElapsedTime() time.Duration {
	if e.startTime.IsZero() {
		return 0
	}
	// Use frozen end time if execution has stopped
	if !e.endTime.IsZero() {
		return e.endTime.Sub(e.startTime)
	}
	return time.Since(e.startTime)
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
		case task.StatusPending:
			s.Pending++
		}
	}
	s.HasFailures = s.Failed > 0
	return s
}
