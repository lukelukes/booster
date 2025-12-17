package tui

import (
	"booster/internal/util"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// SummaryData contains completion statistics.
type SummaryData struct {
	Done    int
	Skipped int
	Failed  int
	Total   int
	Elapsed time.Duration
	// Slowest tasks (name -> duration)
	SlowestTasks []TaskTiming
}

// TaskTiming holds a task name and its execution duration.
type TaskTiming struct {
	Name     string
	Duration time.Duration
}

// RenderSummary renders the completion summary screen.
// Example:
// ┌────────────────────────────────────────────────────┐
// │              ✓ BOOSTER COMPLETE                    │
// │                 2m 34s total                       │
// └────────────────────────────────────────────────────┘
//
//	Summary
//	─────────────────────────────────────────
//	   12 completed    ████████████████████  80%
//	    3 skipped      █████░░░░░░░░░░░░░░░  20%
//	    0 failed       ░░░░░░░░░░░░░░░░░░░░   0%
//
//	Slowest Tasks
//	─────────────────────────────────────────
//	   45.2s   mise: Install node@22
//	   23.1s   mise: Install python@3.12
func RenderSummary(data SummaryData, width int) string {
	var b strings.Builder

	b.WriteString(renderHeaderBox("✓ BOOSTER COMPLETE", data.Elapsed, width, summarySuccessStyle))
	b.WriteString("\n\n")

	b.WriteString(renderStatistics(data))
	b.WriteString("\n")

	if len(data.SlowestTasks) > 0 {
		b.WriteString("\n")
		b.WriteString(renderSlowestTasks(data.SlowestTasks))
	}

	return b.String()
}

// RenderFailedSummary renders summary when there were failures.
func RenderFailedSummary(data SummaryData, width int) string {
	var b strings.Builder

	b.WriteString(renderHeaderBox("✗ BOOSTER FAILED", data.Elapsed, width, summaryFailureStyle))
	b.WriteString("\n\n")

	b.WriteString(renderStatistics(data))
	b.WriteString("\n")

	if len(data.SlowestTasks) > 0 {
		b.WriteString("\n")
		b.WriteString(renderSlowestTasks(data.SlowestTasks))
	}

	return b.String()
}

func renderHeaderBox(title string, elapsed time.Duration, width int, style lipgloss.Style) string {
	boxWidth := width
	if boxWidth <= 0 {
		boxWidth = 60
	}
	if boxWidth > 80 {
		boxWidth = 80
	}
	if boxWidth < 40 {
		boxWidth = 40
	}

	elapsedStr := formatDuration(elapsed)
	subtitle := elapsedStr + " total"

	content := fmt.Sprintf("%s\n%s", title, subtitle)

	return style.
		Width(boxWidth - 4). // Account for border and padding
		Align(lipgloss.Center).
		Render(content)
}

func renderStatistics(data SummaryData) string {
	var b strings.Builder

	b.WriteString(summaryStatStyle.Render("  Summary"))
	b.WriteString("\n")
	b.WriteString(summaryStatStyle.Render("  " + strings.Repeat("─", 41)))
	b.WriteString("\n")

	total := data.Total
	if total == 0 {
		total = 1
	}

	donePercent := float64(data.Done) / float64(total) * 100
	skippedPercent := float64(data.Skipped) / float64(total) * 100
	failedPercent := float64(data.Failed) / float64(total) * 100

	b.WriteString(renderStatLine(data.Done, "completed", donePercent, doneStyle))
	b.WriteString("\n")
	b.WriteString(renderStatLine(data.Skipped, "skipped", skippedPercent, skippedStyle))
	b.WriteString("\n")
	b.WriteString(renderStatLine(data.Failed, "failed", failedPercent, failedStyle))

	return b.String()
}

func renderStatLine(count int, label string, percent float64, style lipgloss.Style) string {
	const barWidth = 20

	bar := renderMiniBar(percent, barWidth)
	countStr := fmt.Sprintf("%2d", count)
	percentStr := fmt.Sprintf("%3.0f%%", percent)
	labelPadded := fmt.Sprintf("%-9s", label)

	return fmt.Sprintf("     %s %s    %s  %s",
		style.Render(countStr),
		summaryStatStyle.Render(labelPadded),
		bar,
		summaryStatStyle.Render(percentStr))
}

func renderMiniBar(percent float64, width int) string {
	filled := util.Clamp(int(math.Round(percent/100*float64(width))), 0, width)
	empty := width - filled

	filledBar := summaryBarStyle.Render(strings.Repeat("█", filled))
	emptyBar := summaryBarEmptyStyle.Render(strings.Repeat("░", empty))

	return filledBar + emptyBar
}

func renderSlowestTasks(tasks []TaskTiming) string {
	var b strings.Builder

	b.WriteString(summaryStatStyle.Render("  Slowest Tasks"))
	b.WriteString("\n")
	b.WriteString(summaryStatStyle.Render("  " + strings.Repeat("─", 41)))
	b.WriteString("\n")

	limit := min(len(tasks), 3)

	for i := range limit {
		task := tasks[i]
		durationStr := formatDuration(task.Duration)
		b.WriteString(fmt.Sprintf("     %s   %s",
			summaryStatStyle.Render(fmt.Sprintf("%6s", durationStr)),
			summaryStatStyle.Render(task.Name)))
		if i < limit-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// formatDuration formats a duration in a human-readable way.
// Examples: "2m 34s", "45.2s", "1.2s"
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return "0s"
	}

	// For durations >= 1 minute, show minutes and seconds
	if d >= time.Minute {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}

	// For durations < 1 minute, show seconds with one decimal
	seconds := d.Seconds()
	if seconds >= 10 {
		// Show whole seconds for 10s and above
		return fmt.Sprintf("%ds", int(seconds))
	}
	// Show one decimal place for under 10 seconds
	return fmt.Sprintf("%.1fs", seconds)
}
