package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRenderProgress(t *testing.T) {
	tests := []struct {
		name     string
		current  int
		total    int
		elapsed  time.Duration
		width    int
		wantText string
	}{
		{
			name:     "0% progress",
			current:  0,
			total:    10,
			elapsed:  5 * time.Second,
			width:    20,
			wantText: "0 of 10 tasks  •  0%  •  5s",
		},
		{
			name:     "50% progress",
			current:  5,
			total:    10,
			elapsed:  30 * time.Second,
			width:    20,
			wantText: "5 of 10 tasks  •  50%  •  30s",
		},
		{
			name:     "100% progress",
			current:  10,
			total:    10,
			elapsed:  2 * time.Minute,
			width:    20,
			wantText: "10 of 10 tasks  •  Complete  •  2m0s",
		},
		{
			name:     "partial progress with minutes",
			current:  3,
			total:    12,
			elapsed:  83 * time.Second,
			width:    30,
			wantText: "3 of 12 tasks  •  25%  •  1m23s",
		},
		{
			name:     "progress with hours",
			current:  50,
			total:    100,
			elapsed:  2*time.Hour + 15*time.Minute,
			width:    30,
			wantText: "50 of 100 tasks  •  50%  •  2h15m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderProgress(tt.current, tt.total, tt.elapsed, tt.width)

			lines := strings.Split(result, "\n")
			assert.Len(t, lines, 2, "should have 2 lines: stats and bar")

			assert.Contains(t, lines[0], tt.wantText, "stats line should contain expected text")

			barLine := lines[1]
			hasFilled := strings.Contains(barLine, "█")
			hasEmpty := strings.Contains(barLine, "░")
			assert.True(t, hasFilled || hasEmpty, "bar should contain progress characters")
		})
	}
}

func TestRenderProgress_BarWidth(t *testing.T) {
	tests := []struct {
		name       string
		current    int
		total      int
		width      int
		wantFilled int
		wantTotal  int
	}{
		{
			name:       "50% with width 20",
			current:    5,
			total:      10,
			width:      20,
			wantFilled: 5,
			wantTotal:  10,
		},
		{
			name:       "25% with width 40",
			current:    1,
			total:      4,
			width:      40,
			wantFilled: 5,
			wantTotal:  20,
		},
		{
			name:       "100% with width 10",
			current:    10,
			total:      10,
			width:      10,
			wantFilled: 5,
			wantTotal:  5,
		},
		{
			name:       "0% with width 10",
			current:    0,
			total:      10,
			width:      10,
			wantFilled: 0,
			wantTotal:  5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderProgress(tt.current, tt.total, 0, tt.width)
			lines := strings.Split(result, "\n")

			barLine := stripANSI(lines[1])

			filledCount := strings.Count(barLine, "█")
			emptyCount := strings.Count(barLine, "░")

			totalRunes := len([]rune(barLine))

			assert.Equal(t, tt.wantFilled, filledCount, "filled blocks count mismatch")
			assert.Equal(t, tt.wantTotal-tt.wantFilled, emptyCount, "empty blocks count mismatch")
			assert.Equal(t, tt.wantTotal, totalRunes, "total bar width mismatch")
		})
	}
}

func TestRenderProgress_WidthConstraint(t *testing.T) {
	tests := []struct {
		name          string
		terminalWidth int
		wantMaxBar    int
	}{
		{
			name:          "very wide terminal (200 cols)",
			terminalWidth: 200,
			wantMaxBar:    100,
		},
		{
			name:          "wide terminal (120 cols)",
			terminalWidth: 120,
			wantMaxBar:    60,
		},
		{
			name:          "normal terminal (80 cols)",
			terminalWidth: 80,
			wantMaxBar:    40,
		},
		{
			name:          "narrow terminal (40 cols)",
			terminalWidth: 40,
			wantMaxBar:    20,
		},
		{
			name:          "very narrow terminal (20 cols)",
			terminalWidth: 20,
			wantMaxBar:    10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderProgress(5, 10, 0, tt.terminalWidth)
			lines := strings.Split(result, "\n")

			barLine := stripANSI(lines[1])
			barWidth := len([]rune(barLine))

			assert.Equal(t, tt.wantMaxBar, barWidth, "bar should be constrained to 50%% of terminal width")
			assert.LessOrEqual(t, barWidth, tt.terminalWidth/2, "bar should not exceed half terminal width")
		})
	}
}

func TestRenderProgress_EdgeCases(t *testing.T) {
	t.Run("current exceeds total", func(t *testing.T) {
		result := RenderProgress(15, 10, 0, 20)
		assert.Contains(t, result, "Complete", "should show Complete when capped at 100%")
		assert.Contains(t, result, "10 of 10 tasks", "should clamp current to total")
	})

	t.Run("negative current", func(t *testing.T) {
		result := RenderProgress(-5, 10, 0, 20)
		assert.Contains(t, result, "0%", "should treat negative as 0%")
	})

	t.Run("zero total", func(t *testing.T) {
		result := RenderProgress(5, 0, 0, 20)

		assert.NotEmpty(t, result, "should return non-empty result")
	})

	t.Run("negative elapsed time", func(t *testing.T) {
		result := RenderProgress(5, 10, -10*time.Second, 20)
		assert.Contains(t, result, "0s", "should treat negative elapsed as 0s")
	})

	t.Run("zero width uses auto width", func(t *testing.T) {
		result := RenderProgress(5, 10, 30*time.Second, 0)
		lines := strings.Split(result, "\n")
		assert.Len(t, lines, 2, "should still render two lines")

		barLine := stripANSI(lines[1])
		assert.NotEmpty(t, barLine, "bar should have some width")
	})
}

func TestFormatElapsedCompact(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "seconds only",
			duration: 45 * time.Second,
			want:     "45s",
		},
		{
			name:     "minutes and seconds",
			duration: 1*time.Minute + 23*time.Second,
			want:     "1m23s",
		},
		{
			name:     "hours and minutes",
			duration: 2*time.Hour + 3*time.Minute,
			want:     "2h3m",
		},
		{
			name:     "hours with seconds (seconds ignored)",
			duration: 2*time.Hour + 3*time.Minute + 45*time.Second,
			want:     "2h3m",
		},
		{
			name:     "zero duration",
			duration: 0,
			want:     "0s",
		},
		{
			name:     "exactly 1 minute",
			duration: 1 * time.Minute,
			want:     "1m0s",
		},
		{
			name:     "exactly 1 hour",
			duration: 1 * time.Hour,
			want:     "1h0m",
		},
		{
			name:     "negative duration",
			duration: -10 * time.Second,
			want:     "0s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatElapsedCompact(tt.duration)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRenderProgress_ProgressCalculation(t *testing.T) {
	tests := []struct {
		current int
		total   int
		wantPct string
	}{
		{0, 10, "0%"},
		{1, 10, "10%"},
		{3, 12, "25%"},
		{5, 10, "50%"},
		{7, 10, "70%"},
		{1, 3, "33%"},
		{2, 3, "66%"},
	}

	for _, tt := range tests {
		t.Run(tt.wantPct, func(t *testing.T) {
			result := RenderProgress(tt.current, tt.total, 0, 20)
			assert.Contains(t, result, tt.wantPct)
		})
	}
}

func TestRenderProgressWithOptions_ETA(t *testing.T) {
	tests := []struct {
		name            string
		current         int
		total           int
		elapsed         time.Duration
		avgTaskDuration time.Duration
		wantText        string
	}{
		{
			name:            "ETA with 5 tasks remaining",
			current:         5,
			total:           10,
			elapsed:         2 * time.Minute,
			avgTaskDuration: 30 * time.Second,
			wantText:        "5 of 10 tasks  •  ~2m30s remaining  •  2m0s",
		},
		{
			name:            "ETA with 9 tasks remaining",
			current:         1,
			total:           10,
			elapsed:         1 * time.Minute,
			avgTaskDuration: 1 * time.Minute,
			wantText:        "1 of 10 tasks  •  ~9m0s remaining  •  1m0s",
		},
		{
			name:            "ETA with 1 task remaining",
			current:         9,
			total:           10,
			elapsed:         4 * time.Minute,
			avgTaskDuration: 30 * time.Second,
			wantText:        "9 of 10 tasks  •  ~30s remaining  •  4m0s",
		},
		{
			name:            "ETA with hours",
			current:         2,
			total:           10,
			elapsed:         30 * time.Minute,
			avgTaskDuration: 15 * time.Minute,
			wantText:        "2 of 10 tasks  •  ~2h0m remaining  •  30m0s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := ProgressOptions{
				AvgTaskDuration: tt.avgTaskDuration,
			}
			result := RenderProgressWithOptions(tt.current, tt.total, tt.elapsed, 80, opts)

			lines := strings.Split(result, "\n")
			assert.Len(t, lines, 2, "should have 2 lines: stats and bar")

			assert.Contains(t, lines[0], tt.wantText, "stats line should contain expected text")
		})
	}
}

func TestRenderProgressWithOptions_RunningTask(t *testing.T) {
	tests := []struct {
		name            string
		current         int
		total           int
		elapsed         time.Duration
		runningTaskName string
		wantText        string
	}{
		{
			name:            "with running task name",
			current:         5,
			total:           10,
			elapsed:         1 * time.Minute,
			runningTaskName: "Installing node@22",
			wantText:        "5 of 10 tasks  •  Installing node@22  •  50%  •  1m0s",
		},
		{
			name:            "with long task name",
			current:         3,
			total:           12,
			elapsed:         2 * time.Minute,
			runningTaskName: "Compiling large C++ project",
			wantText:        "3 of 12 tasks  •  Compiling large C++ project  •  25%  •  2m0s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := ProgressOptions{
				RunningTaskName: tt.runningTaskName,
			}
			result := RenderProgressWithOptions(tt.current, tt.total, tt.elapsed, 100, opts)

			lines := strings.Split(result, "\n")
			assert.Len(t, lines, 2, "should have 2 lines: stats and bar")

			assert.Contains(t, lines[0], tt.wantText, "stats line should contain expected text")
		})
	}
}

func TestRenderProgressWithOptions_Combined(t *testing.T) {
	tests := []struct {
		name     string
		current  int
		total    int
		elapsed  time.Duration
		opts     ProgressOptions
		wantText string
	}{
		{
			name:    "ETA and running task combined",
			current: 5,
			total:   10,
			elapsed: 2 * time.Minute,
			opts: ProgressOptions{
				RunningTaskName: "Installing node@22",
				AvgTaskDuration: 30 * time.Second,
			},
			wantText: "5 of 10 tasks  •  Installing node@22  •  ~2m30s remaining  •  2m0s",
		},
		{
			name:    "complete with task name",
			current: 10,
			total:   10,
			elapsed: 5 * time.Minute,
			opts: ProgressOptions{
				RunningTaskName: "Final cleanup",
				AvgTaskDuration: 30 * time.Second,
			},
			wantText: "10 of 10 tasks  •  Complete  •  5m0s",
		},
		{
			name:     "no options (basic format)",
			current:  3,
			total:    12,
			elapsed:  1 * time.Minute,
			opts:     ProgressOptions{},
			wantText: "3 of 12 tasks  •  25%  •  1m0s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderProgressWithOptions(tt.current, tt.total, tt.elapsed, 100, tt.opts)

			lines := strings.Split(result, "\n")
			assert.Len(t, lines, 2, "should have 2 lines: stats and bar")

			assert.Contains(t, lines[0], tt.wantText, "stats line should contain expected text")
		})
	}
}

func TestRenderProgressWithOptions_Complete(t *testing.T) {
	t.Run("complete state shows 'Complete' instead of percentage", func(t *testing.T) {
		result := RenderProgressWithOptions(10, 10, 2*time.Minute+34*time.Second, 80, ProgressOptions{})

		lines := strings.Split(result, "\n")
		assert.Len(t, lines, 2)

		assert.Contains(t, lines[0], "Complete")
		assert.NotContains(t, lines[0], "100%")
		assert.Contains(t, lines[0], "10 of 10 tasks")
		assert.Contains(t, lines[0], "2m34s")
	})

	t.Run("complete state ignores running task name", func(t *testing.T) {
		opts := ProgressOptions{
			RunningTaskName: "This should not appear",
			AvgTaskDuration: 30 * time.Second,
		}
		result := RenderProgressWithOptions(10, 10, 2*time.Minute, 80, opts)

		lines := strings.Split(result, "\n")
		assert.Contains(t, lines[0], "Complete")
		assert.NotContains(t, lines[0], "This should not appear")
		assert.NotContains(t, lines[0], "remaining")
	})
}

func stripANSI(s string) string {
	var result strings.Builder
	inEscape := false

	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			inEscape = true
			i++
			continue
		}
		if inEscape {
			if (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z') {
				inEscape = false
			}
			continue
		}
		result.WriteByte(s[i])
	}

	return result.String()
}
