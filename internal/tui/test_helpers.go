package tui

import (
	"booster/internal/task"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var statusIndicators = map[task.Status]string{
	task.StatusDone:    "✓",
	task.StatusSkipped: "○",
	task.StatusFailed:  "✗",
	task.StatusRunning: "→",
}

func AssertTaskStatus(t *testing.T, view, taskName string, status task.Status) {
	t.Helper()

	assert.Contains(t, view, taskName, "View should contain task name: %s", taskName)

	switch status {
	case task.StatusDone, task.StatusSkipped, task.StatusFailed, task.StatusRunning:
		indicator := statusIndicators[status]
		assert.Contains(t, view, indicator, "View should show %v indicator (%s) for task: %s", status, indicator, taskName)
	case task.StatusPending:
		for line := range strings.SplitSeq(view, "\n") {
			if strings.Contains(line, taskName) {
				for status, indicator := range statusIndicators {
					assert.NotContains(t, line, indicator, "Pending task %s should not have %v indicator", taskName, status)
				}
				return
			}
		}
	}
}

func AssertTaskStatusNot(t *testing.T, view string, status task.Status) {
	t.Helper()

	if indicator, ok := statusIndicators[status]; ok {
		assert.NotContains(t, view, indicator, "View should NOT show %v indicator (%s)", status, indicator)
	}
}

func AssertHasError(t *testing.T, view, errorText string) {
	t.Helper()
	assert.Contains(t, view, errorText, "View should show error message: %s", errorText)
}

func AssertShowsTitle(t *testing.T, view string) {
	t.Helper()
	assert.Contains(t, view, "BOOSTER", "View should contain title")
}

func AssertSkippedReason(t *testing.T, view, reason string) {
	t.Helper()
	assert.Contains(t, view, reason, "Skipped task should show reason: %s", reason)
}
