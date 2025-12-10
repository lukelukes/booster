package tui

import (
	"booster/internal/util"
	"fmt"
	"strings"
	"time"
)

// ProgressOptions holds optional parameters for progress rendering.
type ProgressOptions struct {
	// RunningTaskName is the name of the currently running task (optional)
	RunningTaskName string
	// AvgTaskDuration is the average duration per task for ETA calculation (optional)
	AvgTaskDuration time.Duration
}

// RenderProgress renders a progress bar with stats.
// Example output:
// "3 of 12 tasks  •  25%  •  1m23s"
// "████████░░░░░░░░░░░░░░░░░░░░░░░░"
//
// Parameters:
//   - current: number of completed items
//   - total: total number of items
//   - elapsed: time elapsed since start
//   - width: available width for the entire component (including text and bar)
//
// Returns a multi-line string with stats on first line and progress bar on second.
func RenderProgress(current, total int, elapsed time.Duration, width int) string {
	return RenderProgressWithOptions(current, total, elapsed, width, ProgressOptions{})
}

// RenderProgressWithOptions renders a progress bar with stats and optional features.
// Example output formats:
// - Basic: "3 of 12 tasks  •  25%  •  1m23s"
// - With ETA: "5 of 10 tasks  •  ~4m remaining  •  1m23s"
// - With running task: "5 of 10 tasks  •  Installing node@22  •  ~2m remaining  •  1m23s"
// - Complete: "10 of 10 tasks  •  Complete  •  2m34s"
// "████████░░░░░░░░░░░░░░░░░░░░░░░░"
//
// Parameters:
//   - current: number of completed items
//   - total: total number of items
//   - elapsed: time elapsed since start
//   - width: available width for the entire component (including text and bar)
//   - opts: optional parameters for progress rendering
//
// Returns a multi-line string with stats on first line and progress bar on second.
func RenderProgressWithOptions(current, total int, elapsed time.Duration, width int, opts ProgressOptions) string {
	if total <= 0 {
		total = 1 // Prevent division by zero
	}
	if current > total {
		current = total
	}
	if current < 0 {
		current = 0
	}

	// Calculate percentage
	percentage := float64(current) / float64(total) * 100

	// Format elapsed time
	elapsedStr := formatElapsedCompact(elapsed)

	// Build stats line with human-readable format
	var statsLine string
	isComplete := current == total

	if isComplete {
		// Format: "10 of 10 tasks  •  Complete  •  2m34s"
		statsLine = fmt.Sprintf("%d of %d tasks  •  Complete  •  %s", current, total, elapsedStr)
	} else {
		// Start with: "3 of 12 tasks"
		parts := []string{fmt.Sprintf("%d of %d tasks", current, total)}

		// Add running task name if provided
		if opts.RunningTaskName != "" {
			parts = append(parts, opts.RunningTaskName)
		}

		// Add ETA if average task duration is provided
		if opts.AvgTaskDuration > 0 {
			remaining := total - current
			eta := time.Duration(remaining) * opts.AvgTaskDuration
			etaStr := formatElapsedCompact(eta)
			parts = append(parts, fmt.Sprintf("~%s remaining", etaStr))
		} else {
			// Add percentage if no ETA
			parts = append(parts, fmt.Sprintf("%d%%", int(percentage)))
		}

		// Add elapsed time
		parts = append(parts, elapsedStr)

		// Join with bullet separator
		statsLine = strings.Join(parts, "  •  ")
	}

	styledStats := progressTextStyle.Render(statsLine)

	// Calculate bar width (use full width if provided, otherwise use stats line length)
	barWidth := width
	if barWidth <= 0 {
		// If no width specified, make bar same length as stats text
		barWidth = len(statsLine)
	}

	// Constrain bar width to 50% of terminal width
	maxWidth := width / 2
	if maxWidth > 0 && barWidth > maxWidth {
		barWidth = maxWidth
	}

	// Build progress bar using rune count (not byte count)
	// Each █ and ░ is one visual character even though multi-byte
	filledCount := util.Clamp((current*barWidth)/total, 0, barWidth)

	filled := strings.Repeat("█", filledCount)
	empty := strings.Repeat("░", barWidth-filledCount)

	styledBar := progressFilledStyle.Render(filled) + progressEmptyStyle.Render(empty)

	return styledStats + "\n" + styledBar
}

// formatElapsedCompact formats a duration into a compact human-readable string.
// Examples: "45s", "1m23s", "2h3m"
func formatElapsedCompact(d time.Duration) string {
	if d < 0 {
		d = 0
	}

	// Round to seconds
	seconds := int(d.Seconds())

	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm%ds", minutes, secs)
	}
	return fmt.Sprintf("%ds", secs)
}
