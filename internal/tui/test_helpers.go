package tui

import (
	"booster/internal/task"
	"testing"

	"github.com/stretchr/testify/assert"
)

// statusIndicators maps task statuses to their visual indicators.
// Centralizing these makes tests resistant to UI changes.
var statusIndicators = map[task.Status]string{
	task.StatusDone:    "✓",
	task.StatusSkipped: "○",
	task.StatusFailed:  "✗",
	task.StatusRunning: "→",
	// StatusPending has no indicator (just whitespace)
}

// AssertTaskStatus verifies that the view shows the expected status indicator for a task.
// Use this instead of raw assert.Contains to make tests resistant to UI changes.
func AssertTaskStatus(t *testing.T, view, taskName string, status task.Status) {
	t.Helper()

	// Task name should always be present
	assert.Contains(t, view, taskName, "View should contain task name: %s", taskName)

	// Check for the appropriate status indicator
	switch status {
	case task.StatusDone, task.StatusSkipped, task.StatusFailed, task.StatusRunning:
		indicator := statusIndicators[status]
		assert.Contains(t, view, indicator, "View should show %v indicator (%s) for task: %s", status, indicator, taskName)
	case task.StatusPending:
		// Pending tasks have leading spaces but no indicator
		assert.Contains(t, view, "  "+taskName, "Pending task should have indentation")
	}
}

// AssertTaskStatusNot verifies that the view does NOT show a specific status indicator.
func AssertTaskStatusNot(t *testing.T, view string, status task.Status) {
	t.Helper()

	if indicator, ok := statusIndicators[status]; ok {
		assert.NotContains(t, view, indicator, "View should NOT show %v indicator (%s)", status, indicator)
	}
}

// AssertHasError verifies that the view shows an error message.
func AssertHasError(t *testing.T, view, errorText string) {
	t.Helper()
	assert.Contains(t, view, errorText, "View should show error message: %s", errorText)
}

// AssertShowsTitle verifies that the view shows the BOOSTER title.
func AssertShowsTitle(t *testing.T, view string) {
	t.Helper()
	assert.Contains(t, view, "BOOSTER", "View should contain title")
}

// AssertRunningEllipsis verifies that a running task shows with ellipsis.
func AssertRunningEllipsis(t *testing.T, view, taskName string) {
	t.Helper()
	assert.Contains(t, view, taskName+"...", "Running task should show with ellipsis")
}

// AssertSkippedReason verifies that a skipped task shows the skip reason.
func AssertSkippedReason(t *testing.T, view, reason string) {
	t.Helper()
	assert.Contains(t, view, reason, "Skipped task should show reason: %s", reason)
}
