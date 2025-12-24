package tui

import (
	"booster/internal/util"
	"fmt"
	"strings"
	"time"
)

type ProgressOptions struct {
	RunningTaskName string

	AvgTaskDuration time.Duration
}

func RenderProgress(current, total int, elapsed time.Duration, width int) string {
	return RenderProgressWithOptions(current, total, elapsed, width, ProgressOptions{})
}

func RenderProgressWithOptions(current, total int, elapsed time.Duration, width int, opts ProgressOptions) string {
	if total <= 0 {
		total = 1
	}
	if current > total {
		current = total
	}
	if current < 0 {
		current = 0
	}

	percentage := float64(current) / float64(total) * 100
	elapsedStr := formatElapsedCompact(elapsed)

	var statsLine string
	isComplete := current == total

	if isComplete {
		statsLine = fmt.Sprintf("%d of %d tasks  •  Complete  •  %s", current, total, elapsedStr)
	} else {
		parts := []string{fmt.Sprintf("%d of %d tasks", current, total)}

		if opts.RunningTaskName != "" {
			parts = append(parts, opts.RunningTaskName)
		}

		if opts.AvgTaskDuration > 0 {
			remaining := total - current
			eta := time.Duration(remaining) * opts.AvgTaskDuration
			etaStr := formatElapsedCompact(eta)
			parts = append(parts, fmt.Sprintf("~%s remaining", etaStr))
		} else {
			parts = append(parts, fmt.Sprintf("%d%%", int(percentage)))
		}

		parts = append(parts, elapsedStr)
		statsLine = strings.Join(parts, "  •  ")
	}

	styledStats := progressTextStyle.Render(statsLine)

	barWidth := width
	if barWidth <= 0 {
		barWidth = len(statsLine)
	}

	maxWidth := width / 2
	if maxWidth > 0 && barWidth > maxWidth {
		barWidth = maxWidth
	}

	filledCount := util.Clamp((current*barWidth)/total, 0, barWidth)

	filled := strings.Repeat("█", filledCount)
	empty := strings.Repeat("░", barWidth-filledCount)

	styledBar := progressFilledStyle.Render(filled) + progressEmptyStyle.Render(empty)

	return styledStats + "\n" + styledBar
}

func formatElapsedCompact(d time.Duration) string {
	if d < 0 {
		d = 0
	}

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
