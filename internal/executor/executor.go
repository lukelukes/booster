package executor

import (
	"booster/internal/task"
	"context"
	"time"
)

type Summary struct {
	Done        int
	Skipped     int
	Failed      int
	Pending     int
	HasFailures bool
}

type Executor struct {
	tasks     []task.Task
	results   []task.Result
	current   int
	aborted   bool
	startTime time.Time
	endTime   time.Time
}

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

func (e *Executor) Total() int {
	return len(e.tasks)
}

func (e *Executor) Current() int {
	return e.current
}

func (e *Executor) Done() bool {
	return e.current >= len(e.tasks)
}

func (e *Executor) Abort() {
	if !e.aborted {
		e.aborted = true
		e.endTime = time.Now()
	}
}

func (e *Executor) Stopped() bool {
	return e.aborted || e.Done()
}

func (e *Executor) Tasks() []task.Task {
	return e.tasks
}

func (e *Executor) Results() []task.Result {
	return e.results
}

func (e *Executor) ResultAt(i int) task.Result {
	if i < 0 || i >= len(e.results) {
		return task.Result{Status: task.StatusPending}
	}
	return e.results[i]
}

func (e *Executor) RunNext(ctx context.Context) (task.Result, bool) {
	if e.Stopped() {
		return task.Result{}, false
	}

	if e.current == 0 {
		e.startTime = time.Now()
	}

	t := e.tasks[e.current]
	taskStart := time.Now()
	result := t.Run(ctx)
	result.Duration = time.Since(taskStart)
	e.results[e.current] = result
	e.current++

	if e.Done() {
		e.endTime = time.Now()
	}

	return result, true
}

func (e *Executor) ElapsedTime() time.Duration {
	if e.startTime.IsZero() {
		return 0
	}

	if !e.endTime.IsZero() {
		return e.endTime.Sub(e.startTime)
	}
	return time.Since(e.startTime)
}

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
