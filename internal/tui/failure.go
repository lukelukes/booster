package tui

import (
	"fmt"
	"strings"
	"time"
)

const (
	maxOutputLines = 10 // Maximum number of output lines to show in failure box
)

// FailureInfo contains information about a failed task
type FailureInfo struct {
	TaskName string
	Error    error
	Output   string // Last output/logs from the task
	Duration time.Duration
}

// RenderFailure renders a failure box with context
// Should produce something like:
// ┌─ FAILED ──────────────────────────────────────────┐
// │  ✗ mise: Install python@3.12                      │
// │                                                   │
// │  Error: Build failed - missing libssl-dev         │
// │                                                   │
// │  ─── Last output ───                              │
// │  configure: error: OpenSSL not found              │
// │  make: *** [Makefile:123] Error 1                 │
// └───────────────────────────────────────────────────┘
func RenderFailure(info FailureInfo, width int) string {
	// Calculate inner width (accounting for box borders and padding)
	innerWidth := max(
		// 2 for borders + 4 for padding
		width-6,
		// Minimum reasonable width
		20)

	var content strings.Builder

	// Header with task name
	header := failureHeaderStyle.Render("FAILED")
	content.WriteString(header)
	content.WriteString("\n")

	// Task name with error icon
	taskLine := "✗ " + info.TaskName
	content.WriteString(failureTaskStyle.Render(truncateLine(taskLine, innerWidth)))
	content.WriteString("\n\n")

	// Error message
	if info.Error != nil {
		errMsg := "Error: " + info.Error.Error()
		// Wrap error message if it's too long
		wrappedErr := wrapText(errMsg, innerWidth)
		for _, line := range wrappedErr {
			content.WriteString(failureErrorStyle.Render(line))
			content.WriteString("\n")
		}
		content.WriteString("\n")
	}

	// Output section (if present)
	if info.Output != "" {
		content.WriteString(failureOutputHeaderStyle.Render("─── Last output ───"))
		content.WriteString("\n")

		// Get last N lines of output
		outputLines := getLastLines(info.Output, maxOutputLines)
		for _, line := range outputLines {
			truncated := truncateLine(line, innerWidth)
			content.WriteString(failureOutputStyle.Render(truncated))
			content.WriteString("\n")
		}
	}

	// Wrap content in a bordered box
	boxContent := content.String()
	boxStyle := failureBoxStyle.Width(innerWidth)

	return boxStyle.Render(boxContent)
}

// truncateLine truncates a line to the specified width, adding ellipsis if needed
func truncateLine(line string, maxWidth int) string {
	if len(line) <= maxWidth {
		return line
	}
	if maxWidth <= 3 {
		return "..."
	}
	return line[:maxWidth-3] + "..."
}

// wrapText wraps text to fit within the specified width
func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	var lines []string
	remaining := text

	for len(remaining) > 0 {
		if len(remaining) <= width {
			lines = append(lines, remaining)
			break
		}

		// Try to break at a word boundary
		breakPoint := width
		for i := width - 1; i >= width/2; i-- {
			if remaining[i] == ' ' {
				breakPoint = i
				break
			}
		}

		lines = append(lines, strings.TrimSpace(remaining[:breakPoint]))
		remaining = strings.TrimSpace(remaining[breakPoint:])
	}

	return lines
}

// getLastLines returns the last N lines from a multi-line string
func getLastLines(text string, n int) []string {
	if text == "" {
		return nil
	}

	// Split by newlines and filter out empty trailing lines
	allLines := strings.Split(strings.TrimSpace(text), "\n")

	// Return last N lines
	if len(allLines) <= n {
		return allLines
	}

	return allLines[len(allLines)-n:]
}

// RenderFailureSummary renders a compact failure list for multiple failures
// Used when multiple tasks fail and you want to show them all
func RenderFailureSummary(failures []FailureInfo, width int) string {
	if len(failures) == 0 {
		return ""
	}

	var content strings.Builder

	content.WriteString(failureHeaderStyle.Render(
		fmt.Sprintf("FAILURES (%d)", len(failures))))
	content.WriteString("\n\n")

	for i, failure := range failures {
		// Task name with error icon
		taskLine := "✗ " + failure.TaskName
		content.WriteString(failureTaskStyle.Render(taskLine))
		content.WriteString("\n")

		// Brief error message (first line only)
		if failure.Error != nil {
			errMsg := failure.Error.Error()
			// Take only first line if multi-line error
			if idx := strings.Index(errMsg, "\n"); idx > 0 {
				errMsg = errMsg[:idx]
			}
			innerWidth := max(width-8, 20)
			truncated := truncateLine("  "+errMsg, innerWidth)
			content.WriteString(failureErrorStyle.Render(truncated))
			content.WriteString("\n")
		}

		// Add spacing between failures
		if i < len(failures)-1 {
			content.WriteString("\n")
		}
	}

	innerWidth := max(width-6, 20)

	boxStyle := failureBoxStyle.Width(innerWidth)
	return boxStyle.Render(content.String())
}
