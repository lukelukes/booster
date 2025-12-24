package coordinator

import (
	"booster/internal/task"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCoordinator_TaskCompletes_WhenLogsFinishFirst tests the normal case where
// logs complete before the task result arrives.
func TestCoordinator_TaskCompletes_WhenLogsFinishFirst(t *testing.T) {
	tests := []struct {
		name         string
		logLines     []string
		taskResult   task.Result
		wantLogCount int
	}{
		{
			name:         "multiple logs before task done",
			logLines:     []string{"line1", "line2"},
			taskResult:   task.Result{Status: task.StatusDone},
			wantLogCount: 2,
		},
		{
			name:         "empty logs",
			logLines:     []string{},
			taskResult:   task.Result{Status: task.StatusDone},
			wantLogCount: 0,
		},
		{
			name:         "single log line",
			logLines:     []string{"only line"},
			taskResult:   task.Result{Status: task.StatusDone},
			wantLogCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New()
			c.StartTask(0)

			// Feed log lines
			for _, line := range tt.logLines {
				c.AddLogLine(line)
			}

			// Signal logs done FIRST
			msg := c.LogsDone()
			assert.Nil(t, msg, "LogsDone alone should not complete task")

			// Then task done
			msg = c.TaskDone(tt.taskResult)
			require.NotNil(t, msg, "TaskDone after LogsDone should complete task")

			assert.Equal(t, tt.taskResult.Status, msg.Result.Status)
			assert.Equal(t, 0, msg.TaskIndex)
			assert.Len(t, c.LogsFor(0), tt.wantLogCount)
		})
	}
}

// TestCoordinator_TaskCompletes_WhenTaskFinishesFirst tests the race condition
// where the task result arrives before all logs are received.
func TestCoordinator_TaskCompletes_WhenTaskFinishesFirst(t *testing.T) {
	c := New()
	c.StartTask(0)

	// Add some logs
	c.AddLogLine("early log")

	// Task done arrives FIRST (before LogsDone)
	msg := c.TaskDone(task.Result{Status: task.StatusDone})
	assert.Nil(t, msg, "TaskDone before LogsDone should not complete task")

	// More logs can still arrive after task done
	c.AddLogLine("late log")

	// Verify current logs accumulator has both lines
	assert.Len(t, c.CurrentLogs(), 2)

	// Now logs done
	msg = c.LogsDone()
	require.NotNil(t, msg, "LogsDone after TaskDone should complete task")

	// All logs should be captured in history
	logs := c.LogsFor(0)
	require.Len(t, logs, 2)
	assert.Equal(t, "early log", logs[0])
	assert.Equal(t, "late log", logs[1])
}

// TestCoordinator_LogHistory_PersistsAcrossTasks verifies logs are correctly
// attributed and persisted for each task.
func TestCoordinator_LogHistory_PersistsAcrossTasks(t *testing.T) {
	c := New()

	// Task 0
	c.StartTask(0)
	c.AddLogLine("task0-line1")
	c.AddLogLine("task0-line2")
	c.LogsDone()
	c.TaskDone(task.Result{Status: task.StatusDone})

	// Task 1
	c.StartTask(1)
	c.AddLogLine("task1-line1")
	c.LogsDone()
	c.TaskDone(task.Result{Status: task.StatusDone})

	// Task 2 with no logs
	c.StartTask(2)
	c.LogsDone()
	c.TaskDone(task.Result{Status: task.StatusSkipped})

	// Verify task 0 logs still accessible
	logs0 := c.LogsFor(0)
	require.Len(t, logs0, 2)
	assert.Equal(t, "task0-line1", logs0[0])
	assert.Equal(t, "task0-line2", logs0[1])

	// Verify task 1 logs
	logs1 := c.LogsFor(1)
	require.Len(t, logs1, 1)
	assert.Equal(t, "task1-line1", logs1[0])

	// Verify task 2 has no logs
	logs2 := c.LogsFor(2)
	assert.Empty(t, logs2)
}

// TestCoordinator_CurrentLogs_ReturnsAccumulator verifies CurrentLogs returns
// the in-progress log accumulator.
func TestCoordinator_CurrentLogs_ReturnsAccumulator(t *testing.T) {
	c := New()
	c.StartTask(0)

	assert.Empty(t, c.CurrentLogs())

	c.AddLogLine("line1")
	assert.Equal(t, []string{"line1"}, c.CurrentLogs())

	c.AddLogLine("line2")
	assert.Equal(t, []string{"line1", "line2"}, c.CurrentLogs())
}

// TestCoordinator_TaskFailure_PreservesLogs verifies that logs are preserved
// even when a task fails.
func TestCoordinator_TaskFailure_PreservesLogs(t *testing.T) {
	c := New()
	c.StartTask(0)

	c.AddLogLine("before failure")
	c.AddLogLine("error output")
	c.LogsDone()
	msg := c.TaskDone(task.Result{Status: task.StatusFailed, Message: "something broke"})

	require.NotNil(t, msg)
	assert.Equal(t, task.StatusFailed, msg.Result.Status)
	assert.Equal(t, "something broke", msg.Result.Message)

	logs := c.LogsFor(0)
	require.Len(t, logs, 2)
	assert.Equal(t, "before failure", logs[0])
	assert.Equal(t, "error output", logs[1])
}

// TestCoordinator_StartTask_ResetsState verifies that starting a new task
// clears the current logs accumulator but preserves history.
func TestCoordinator_StartTask_ResetsState(t *testing.T) {
	c := New()

	// Complete first task
	c.StartTask(0)
	c.AddLogLine("task0 log")
	c.LogsDone()
	c.TaskDone(task.Result{Status: task.StatusDone})

	// Start second task - current logs should be empty
	c.StartTask(1)

	// CurrentLogs should be empty (new task)
	assert.Empty(t, c.CurrentLogs())

	// Task 0 logs should still be in history
	assert.Len(t, c.LogsFor(0), 1)
}

// TestCoordinator_LogsFor_ReturnsNilForUnknownTask verifies LogsFor returns
// nil for tasks that haven't been executed.
func TestCoordinator_LogsFor_ReturnsNilForUnknownTask(t *testing.T) {
	c := New()

	// No tasks started yet
	assert.Nil(t, c.LogsFor(0))
	assert.Nil(t, c.LogsFor(999))
}

// TestCoordinator_TaskCompleteMsg_ContainsLogs verifies the completion message
// includes the captured logs.
func TestCoordinator_TaskCompleteMsg_ContainsLogs(t *testing.T) {
	c := New()
	c.StartTask(0)
	c.AddLogLine("log1")
	c.AddLogLine("log2")
	c.LogsDone()

	msg := c.TaskDone(task.Result{Status: task.StatusDone})

	require.NotNil(t, msg)
	require.Len(t, msg.Logs, 2)
	assert.Equal(t, "log1", msg.Logs[0])
	assert.Equal(t, "log2", msg.Logs[1])
}

// TestCoordinator_MultipleTaskDoneCalls_OnlyFirstCounts verifies that calling
// TaskDone multiple times doesn't cause issues.
func TestCoordinator_DoubleCompletion_Handled(t *testing.T) {
	c := New()
	c.StartTask(0)
	c.AddLogLine("log")
	c.LogsDone()

	// First TaskDone should complete
	msg1 := c.TaskDone(task.Result{Status: task.StatusDone})
	require.NotNil(t, msg1)

	// Second TaskDone should return nil (already completed)
	msg2 := c.TaskDone(task.Result{Status: task.StatusFailed})
	assert.Nil(t, msg2, "Second TaskDone should return nil")
}
