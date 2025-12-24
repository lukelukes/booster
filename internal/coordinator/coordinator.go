// Package coordinator handles task execution coordination including
// log streaming and the race condition between task completion and log delivery.
package coordinator

import (
	"booster/internal/task"
)

// TaskCompleteMsg is sent when both task execution AND log streaming are done.
// This replaces the separate taskDoneMsg and logDoneMsg that the TUI previously handled.
type TaskCompleteMsg struct {
	TaskIndex int
	Result    task.Result
	Logs      []string
}

// Coordinator manages the race condition between task completion and log streaming.
// It ensures that logs are correctly attributed to tasks regardless of message arrival order.
//
// Usage:
//
//	c := coordinator.New()
//	c.StartTask(taskIndex)
//	// ... feed log lines via AddLogLine
//	// ... call LogsDone when log channel closes
//	// ... call TaskDone when task execution completes
//	// When both are done, the appropriate call returns TaskCompleteMsg
type Coordinator struct {
	logHistory  map[int][]string // task index -> persisted log lines
	currentLogs []string         // accumulator for running task
	currentTask int              // current task index

	// Coordination state (internal - handles the race condition)
	pendingResult *task.Result // holds task result if arrived before logs
	logsDone      bool         // true when LogsDone called for current task
}

// New creates a new Coordinator.
func New() *Coordinator {
	return &Coordinator{
		logHistory: make(map[int][]string),
	}
}

// StartTask prepares the coordinator for a new task.
// This resets the coordination state and clears the current log accumulator.
func (c *Coordinator) StartTask(taskIndex int) {
	c.currentTask = taskIndex
	c.currentLogs = nil
	c.pendingResult = nil
	c.logsDone = false
}

// AddLogLine adds a log line to the current task's accumulator.
func (c *Coordinator) AddLogLine(line string) {
	c.currentLogs = append(c.currentLogs, line)
}

// CurrentLogs returns the current log accumulator (for display during execution).
func (c *Coordinator) CurrentLogs() []string {
	return c.currentLogs
}

// LogsFor returns the persisted logs for a completed task.
// Returns nil if the task has no logs or hasn't been executed.
func (c *Coordinator) LogsFor(taskIndex int) []string {
	return c.logHistory[taskIndex]
}

// LogsDone signals that the log channel has closed.
// Returns TaskCompleteMsg if the task result was already received, nil otherwise.
func (c *Coordinator) LogsDone() *TaskCompleteMsg {
	c.logsDone = true

	if c.pendingResult != nil {
		return c.complete(*c.pendingResult)
	}
	return nil
}

// TaskDone signals that task execution has completed.
// Returns TaskCompleteMsg if logs are already done, nil otherwise.
func (c *Coordinator) TaskDone(result task.Result) *TaskCompleteMsg {
	// Guard against double completion
	if c.pendingResult == nil && !c.logsDone {
		// First call, logs not done yet - store and wait
		c.pendingResult = &result
		return nil
	}

	if !c.logsDone {
		// Already have a pending result, ignore duplicate
		return nil
	}

	// Logs are done and this is the first valid TaskDone call
	return c.complete(result)
}

// complete finishes task coordination, persists logs, and returns the completion message.
func (c *Coordinator) complete(result task.Result) *TaskCompleteMsg {
	// Persist current logs to history (even if empty, for consistency)
	if len(c.currentLogs) > 0 {
		c.logHistory[c.currentTask] = c.currentLogs
	}

	msg := &TaskCompleteMsg{
		TaskIndex: c.currentTask,
		Result:    result,
		Logs:      c.currentLogs,
	}

	// Clear coordination state (ready for next task)
	c.currentLogs = nil
	c.pendingResult = nil
	c.logsDone = false

	return msg
}
