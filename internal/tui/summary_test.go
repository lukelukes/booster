package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderSummary_SuccessState(t *testing.T) {
	data := SummaryData{
		Done:    12,
		Skipped: 3,
		Failed:  0,
		Total:   15,
		Elapsed: 2*time.Minute + 34*time.Second,
		SlowestTasks: []TaskTiming{
			{Name: "mise: Install node@22", Duration: 45*time.Second + 200*time.Millisecond},
			{Name: "mise: Install python@3.12", Duration: 23*time.Second + 100*time.Millisecond},
		},
	}

	result := RenderSummary(data, 60)

	assert.Contains(t, result, "✓ BOOSTER COMPLETE", "Should contain success message")
	assert.Contains(t, result, "2m 34s", "Should contain formatted elapsed time")

	assert.Contains(t, result, "Summary", "Should contain summary section")
	assert.Contains(t, result, "12", "Should show completed count")
	assert.Contains(t, result, "completed", "Should show completed label")
	assert.Contains(t, result, "3", "Should show skipped count")
	assert.Contains(t, result, "skipped", "Should show skipped label")
	assert.Contains(t, result, "0", "Should show failed count")
	assert.Contains(t, result, "failed", "Should show failed label")

	assert.Contains(t, result, "80%", "Should show 80% for completed")
	assert.Contains(t, result, "20%", "Should show 20% for skipped")
	assert.Contains(t, result, "0%", "Should show 0% for failed")

	assert.Contains(t, result, "Slowest Tasks", "Should contain slowest tasks section")
	assert.Contains(t, result, "45", "Should show first task duration")
	assert.Contains(t, result, "mise: Install node@22", "Should show first task name")
	assert.Contains(t, result, "23", "Should show second task duration")
	assert.Contains(t, result, "mise: Install python@3.12", "Should show second task name")

	assert.Contains(t, result, "█", "Should contain filled bar blocks")
	assert.Contains(t, result, "░", "Should contain empty bar blocks")
}

func TestRenderFailedSummary_FailureState(t *testing.T) {
	data := SummaryData{
		Done:    5,
		Skipped: 2,
		Failed:  3,
		Total:   10,
		Elapsed: 45 * time.Second,
		SlowestTasks: []TaskTiming{
			{Name: "failed task", Duration: 10 * time.Second},
		},
	}

	result := RenderFailedSummary(data, 60)

	assert.Contains(t, result, "✗ BOOSTER FAILED", "Should contain failure message")
	assert.Contains(t, result, "45s", "Should contain formatted elapsed time")

	assert.Contains(t, result, "5", "Should show completed count")
	assert.Contains(t, result, "2", "Should show skipped count")
	assert.Contains(t, result, "3", "Should show failed count")

	assert.Contains(t, result, "50%", "Should show 50% for completed")
	assert.Contains(t, result, "20%", "Should show 20% for skipped")
	assert.Contains(t, result, "30%", "Should show 30% for failed")

	assert.Contains(t, result, "Slowest Tasks", "Should contain slowest tasks section")
	assert.Contains(t, result, "failed task", "Should show task name")
}

func TestRenderSummary_NoSlowestTasks(t *testing.T) {
	data := SummaryData{
		Done:         5,
		Skipped:      0,
		Failed:       0,
		Total:        5,
		Elapsed:      30 * time.Second,
		SlowestTasks: []TaskTiming{},
	}

	result := RenderSummary(data, 60)

	assert.NotContains(t, result, "Slowest Tasks", "Should not contain slowest tasks section when empty")
}

func TestRenderSummary_AllSkipped(t *testing.T) {
	data := SummaryData{
		Done:    0,
		Skipped: 10,
		Failed:  0,
		Total:   10,
		Elapsed: 5 * time.Second,
	}

	result := RenderSummary(data, 60)

	assert.Contains(t, result, "0%", "Should show 0% for completed")
	assert.Contains(t, result, "100%", "Should show 100% for skipped")

	assert.Contains(t, result, "10", "Should show skipped count")
	assert.Contains(t, result, "0", "Should show completed count as 0")
}

func TestRenderSummary_EdgeCaseZeroTasks(t *testing.T) {
	data := SummaryData{
		Done:    0,
		Skipped: 0,
		Failed:  0,
		Total:   0,
		Elapsed: 0,
	}

	result := RenderSummary(data, 60)

	assert.NotEmpty(t, result, "Should return non-empty result even with zero tasks")
	assert.Contains(t, result, "✓ BOOSTER COMPLETE", "Should contain success message")
}

func TestRenderSummary_VariousWidths(t *testing.T) {
	data := SummaryData{
		Done:    5,
		Skipped: 3,
		Failed:  2,
		Total:   10,
		Elapsed: 1*time.Minute + 30*time.Second,
	}

	tests := []struct {
		name  string
		width int
	}{
		{"small width", 40},
		{"medium width", 60},
		{"large width", 80},
		{"very large width", 120},
		{"zero width", 0},
		{"negative width", -10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderSummary(data, tt.width)
			assert.NotEmpty(t, result, "Should return non-empty result")
			assert.Contains(t, result, "✓ BOOSTER COMPLETE", "Should contain success message")
		})
	}
}

func TestRenderSummary_ManySlowestTasks(t *testing.T) {
	data := SummaryData{
		Done:    10,
		Skipped: 0,
		Failed:  0,
		Total:   10,
		Elapsed: 2 * time.Minute,
		SlowestTasks: []TaskTiming{
			{Name: "task1", Duration: 50 * time.Second},
			{Name: "task2", Duration: 40 * time.Second},
			{Name: "task3", Duration: 30 * time.Second},
			{Name: "task4", Duration: 20 * time.Second},
			{Name: "task5", Duration: 10 * time.Second},
		},
	}

	result := RenderSummary(data, 60)

	assert.Contains(t, result, "task1", "Should show first slowest task")
	assert.Contains(t, result, "task2", "Should show second slowest task")
	assert.Contains(t, result, "task3", "Should show third slowest task")

	assert.NotContains(t, result, "task4", "Should not show fourth task")
	assert.NotContains(t, result, "task5", "Should not show fifth task")
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "less than a second",
			duration: 500 * time.Millisecond,
			want:     "0s",
		},
		{
			name:     "exactly one second",
			duration: 1 * time.Second,
			want:     "1.0s",
		},
		{
			name:     "under 10 seconds with decimal",
			duration: 5*time.Second + 200*time.Millisecond,
			want:     "5.2s",
		},
		{
			name:     "exactly 10 seconds",
			duration: 10 * time.Second,
			want:     "10s",
		},
		{
			name:     "over 10 seconds",
			duration: 45 * time.Second,
			want:     "45s",
		},
		{
			name:     "exactly one minute",
			duration: 1 * time.Minute,
			want:     "1m 0s",
		},
		{
			name:     "one minute and 30 seconds",
			duration: 1*time.Minute + 30*time.Second,
			want:     "1m 30s",
		},
		{
			name:     "two minutes and 34 seconds",
			duration: 2*time.Minute + 34*time.Second,
			want:     "2m 34s",
		},
		{
			name:     "over 10 minutes",
			duration: 15*time.Minute + 42*time.Second,
			want:     "15m 42s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			assert.Equal(t, tt.want, result, "Duration formatting mismatch")
		})
	}
}

func TestRenderMiniBar(t *testing.T) {
	tests := []struct {
		name    string
		percent float64
		width   int
	}{
		{"0 percent", 0, 20},
		{"50 percent", 50, 20},
		{"100 percent", 100, 20},
		{"over 100 percent", 150, 20},
		{"negative percent", -10, 20},
		{"small width", 25, 10},
		{"large width", 75, 40},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderMiniBar(tt.percent, tt.width)

			assert.NotEmpty(t, result, "Progress bar should not be empty")

			hasBlocks := strings.Contains(result, "█") || strings.Contains(result, "░")
			assert.True(t, hasBlocks, "Progress bar should contain block characters")
		})
	}
}

func TestRenderStatLine(t *testing.T) {
	result := renderStatLine(12, "completed", 80.0, doneStyle)

	assert.Contains(t, result, "12", "Should contain count")
	assert.Contains(t, result, "completed", "Should contain label")
	assert.Contains(t, result, "80%", "Should contain percentage")

	hasBlocks := strings.Contains(result, "█") || strings.Contains(result, "░")
	assert.True(t, hasBlocks, "Should contain progress bar")
}

func TestRenderStatistics(t *testing.T) {
	data := SummaryData{
		Done:    8,
		Skipped: 1,
		Failed:  1,
		Total:   10,
	}

	result := renderStatistics(data)

	assert.Contains(t, result, "Summary", "Should contain section title")

	assert.Contains(t, result, "8", "Should show done count")
	assert.Contains(t, result, "completed", "Should show completed label")
	assert.Contains(t, result, "1", "Should show skipped count")
	assert.Contains(t, result, "skipped", "Should show skipped label")
	assert.Contains(t, result, "failed", "Should show failed label")

	assert.Contains(t, result, "80%", "Should show 80% for done")
	assert.Contains(t, result, "10%", "Should show 10% for skipped")
	assert.Contains(t, result, "10%", "Should show 10% for failed")
}

func TestRenderHeaderBox(t *testing.T) {
	result := renderHeaderBox("✓ BOOSTER COMPLETE", 2*time.Minute+34*time.Second, 60, summarySuccessStyle)

	assert.Contains(t, result, "✓ BOOSTER COMPLETE", "Should contain title")
	assert.Contains(t, result, "2m 34s", "Should contain elapsed time")
	assert.Contains(t, result, "total", "Should contain 'total' label")
}

func TestRenderSlowestTasks(t *testing.T) {
	tasks := []TaskTiming{
		{Name: "mise: Install node@22", Duration: 45*time.Second + 200*time.Millisecond},
		{Name: "mise: Install python@3.12", Duration: 23*time.Second + 100*time.Millisecond},
	}

	result := renderSlowestTasks(tasks)

	assert.Contains(t, result, "Slowest Tasks", "Should contain section title")

	assert.Contains(t, result, "mise: Install node@22", "Should show first task")
	assert.Contains(t, result, "mise: Install python@3.12", "Should show second task")

	assert.Contains(t, result, "45", "Should show first task duration")
	assert.Contains(t, result, "23", "Should show second task duration")
}

func TestRenderSlowestTasks_OnlyOne(t *testing.T) {
	tasks := []TaskTiming{
		{Name: "single task", Duration: 30 * time.Second},
	}

	result := renderSlowestTasks(tasks)

	assert.Contains(t, result, "Slowest Tasks", "Should contain section title")
	assert.Contains(t, result, "single task", "Should show the task")
	assert.Contains(t, result, "30", "Should show duration")
}

func TestRenderStatLine_Alignment(t *testing.T) {
	completedLine := renderStatLine(12, "completed", 80.0, doneStyle)
	skippedLine := renderStatLine(3, "skipped", 20.0, skippedStyle)
	failedLine := renderStatLine(0, "failed", 0.0, failedStyle)

	assert.Contains(t, completedLine, "completed", "Should contain completed label")
	assert.Contains(t, skippedLine, "skipped", "Should contain skipped label")
	assert.Contains(t, failedLine, "failed", "Should contain failed label")

	extractMiddle := func(s string) string {
		start := strings.Index(s, " ")
		if start == -1 {
			return ""
		}

		pctIdx := strings.Index(s, "%")
		if pctIdx == -1 {
			return ""
		}

		i := pctIdx - 1
		for i >= 0 && (s[i] >= '0' && s[i] <= '9' || s[i] == ' ') {
			i--
		}
		return s[start : i+1]
	}

	completedMiddle := extractMiddle(completedLine)
	skippedMiddle := extractMiddle(skippedLine)
	failedMiddle := extractMiddle(failedLine)

	require.NotEmpty(t, completedMiddle, "Should extract middle section from completed line")
	require.NotEmpty(t, skippedMiddle, "Should extract middle section from skipped line")
	require.NotEmpty(t, failedMiddle, "Should extract middle section from failed line")

	completedLen := len(completedMiddle)
	skippedLen := len(skippedMiddle)
	failedLen := len(failedMiddle)

	assert.Equal(t, completedLen, skippedLen,
		"Skipped line middle section should have same length as completed\nCompleted: %q\nSkipped: %q",
		completedMiddle, skippedMiddle)
	assert.Equal(t, completedLen, failedLen,
		"Failed line middle section should have same length as completed\nCompleted: %q\nFailed: %q",
		completedMiddle, failedMiddle)
}

func TestRenderStatistics_Alignment(t *testing.T) {
	data := SummaryData{
		Done:    12,
		Skipped: 3,
		Failed:  0,
		Total:   15,
	}

	result := renderStatistics(data)
	lines := strings.Split(result, "\n")

	extractMiddle := func(s string) string {
		start := strings.Index(s, " ")
		if start == -1 {
			return ""
		}
		pctIdx := strings.Index(s, "%")
		if pctIdx == -1 {
			return ""
		}

		i := pctIdx - 1
		for i >= 0 && (s[i] >= '0' && s[i] <= '9' || s[i] == ' ') {
			i--
		}
		return s[start : i+1]
	}

	var middleSections []string
	for _, line := range lines {
		if strings.Contains(line, "Summary") || strings.Contains(line, "─") {
			continue
		}
		middle := extractMiddle(line)
		if middle != "" {
			middleSections = append(middleSections, middle)
		}
	}

	require.Len(t, middleSections, 3, "Should have 3 stat lines")

	firstLen := len(middleSections[0])
	for i := 1; i < len(middleSections); i++ {
		assert.Len(t, middleSections[i], firstLen,
			"All stat lines should have same structure for alignment\nLine 0: %q\nLine %d: %q",
			middleSections[0], i, middleSections[i])
	}
}

func TestSummaryData_Integration(t *testing.T) {
	tests := []struct {
		name     string
		data     SummaryData
		useFail  bool
		wantText []string
	}{
		{
			name: "typical success scenario",
			data: SummaryData{
				Done:    15,
				Skipped: 5,
				Failed:  0,
				Total:   20,
				Elapsed: 3*time.Minute + 15*time.Second,
				SlowestTasks: []TaskTiming{
					{Name: "slow task 1", Duration: 60 * time.Second},
					{Name: "slow task 2", Duration: 45 * time.Second},
				},
			},
			useFail: false,
			wantText: []string{
				"✓ BOOSTER COMPLETE",
				"3m 15s",
				"15",
				"completed",
				"5",
				"skipped",
				"0",
				"failed",
				"slow task 1",
				"slow task 2",
			},
		},
		{
			name: "typical failure scenario",
			data: SummaryData{
				Done:    8,
				Skipped: 2,
				Failed:  5,
				Total:   15,
				Elapsed: 1*time.Minute + 45*time.Second,
				SlowestTasks: []TaskTiming{
					{Name: "failed task", Duration: 30 * time.Second},
				},
			},
			useFail: true,
			wantText: []string{
				"✗ BOOSTER FAILED",
				"1m 45s",
				"8",
				"completed",
				"2",
				"skipped",
				"5",
				"failed",
				"failed task",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			if tt.useFail {
				result = RenderFailedSummary(tt.data, 60)
			} else {
				result = RenderSummary(tt.data, 60)
			}

			require.NotEmpty(t, result, "Result should not be empty")

			for _, want := range tt.wantText {
				assert.Contains(t, result, want, "Should contain: %s", want)
			}
		})
	}
}
