package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRenderFailure(t *testing.T) {
	tests := []struct {
		name     string
		info     FailureInfo
		width    int
		wantText []string
	}{
		{
			name: "basic failure with error",
			info: FailureInfo{
				TaskName: "Install python@3.12",
				Error:    errors.New("build failed"),
			},
			width: 60,
			wantText: []string{
				"FAILED",
				"✗ Install python@3.12",
				"Error: build failed",
			},
		},
		{
			name: "failure with output",
			info: FailureInfo{
				TaskName: "Install python@3.12",
				Error:    errors.New("build failed"),
				Output:   "configure: error: OpenSSL not found\nmake: *** [Makefile:123] Error 1",
			},
			width: 60,
			wantText: []string{
				"FAILED",
				"✗ Install python@3.12",
				"Error: build failed",
				"─── Last output ───",
				"configure: error: OpenSSL not found",
				"make: *** [Makefile:123] Error 1",
			},
		},
		{
			name: "failure with long error message",
			info: FailureInfo{
				TaskName: "Install package",
				Error:    errors.New("this is a very long error message that should be wrapped to fit within the specified width constraints of the terminal display"),
			},
			width: 40,
			wantText: []string{
				"FAILED",
				"✗ Install package",
				"Error:",
			},
		},
		{
			name: "failure with many output lines",
			info: FailureInfo{
				TaskName: "Build project",
				Error:    errors.New("compilation failed"),
				Output: strings.Join([]string{
					"line 1",
					"line 2",
					"line 3",
					"line 4",
					"line 5",
					"line 6",
					"line 7",
					"line 8",
					"line 9",
					"line 10",
					"line 11",
					"line 12",
					"line 13",
					"line 14",
					"line 15",
				}, "\n"),
			},
			width: 60,
			wantText: []string{
				"FAILED",
				"✗ Build project",
				"─── Last output ───",
				"line 6",
				"line 15",
			},
		},
		{
			name: "failure with nil error",
			info: FailureInfo{
				TaskName: "Test task",
				Error:    nil,
				Output:   "some output",
			},
			width: 60,
			wantText: []string{
				"FAILED",
				"✗ Test task",
				"─── Last output ───",
				"some output",
			},
		},
		{
			name: "narrow terminal width",
			info: FailureInfo{
				TaskName: "Install something with a very long name",
				Error:    errors.New("error occurred"),
			},
			width: 30,
			wantText: []string{
				"FAILED",
				"✗",
				"Error:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := RenderFailure(tt.info, tt.width)

			assert.NotEmpty(t, output, "output should not be empty")

			for _, want := range tt.wantText {
				assert.Contains(t, output, want,
					"output should contain %q", want)
			}

			lines := strings.SplitSeq(output, "\n")
			for line := range lines {

				stripped := stripAnsiSimple(line)

				assert.LessOrEqual(t, len(stripped), tt.width*3,
					"line should be reasonable width: %q", stripped)
			}
		})
	}
}

func TestTruncateLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		maxWidth int
		want     string
	}{
		{
			name:     "line shorter than max",
			line:     "hello",
			maxWidth: 10,
			want:     "hello",
		},
		{
			name:     "line equal to max",
			line:     "helloworld",
			maxWidth: 10,
			want:     "helloworld",
		},
		{
			name:     "line longer than max",
			line:     "hello world, this is a long line",
			maxWidth: 20,
			want:     "hello world, this...",
		},
		{
			name:     "very small max width",
			line:     "hello",
			maxWidth: 3,
			want:     "...",
		},
		{
			name:     "max width less than ellipsis",
			line:     "hello",
			maxWidth: 2,
			want:     "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateLine(tt.line, tt.maxWidth)
			assert.Equal(t, tt.want, got)

			if tt.maxWidth > 3 {
				assert.LessOrEqual(t, len(got), tt.maxWidth)
			}
		})
	}
}

func TestWrapText(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		width int
		want  []string
	}{
		{
			name:  "text shorter than width",
			text:  "hello",
			width: 10,
			want:  []string{"hello"},
		},
		{
			name:  "text equal to width",
			text:  "helloworld",
			width: 10,
			want:  []string{"helloworld"},
		},
		{
			name:  "text wraps at word boundary",
			text:  "hello world foo bar",
			width: 12,
			want:  []string{"hello world", "foo bar"},
		},
		{
			name:  "text wraps multiple times",
			text:  "this is a long text that needs to wrap multiple times",
			width: 15,
			want: []string{
				"this is a long",
				"text that",
				"needs to wrap",
				"multiple times",
			},
		},
		{
			name:  "zero width returns original",
			text:  "hello",
			width: 0,
			want:  []string{"hello"},
		},
		{
			name:  "no word boundaries forces break",
			text:  "verylongwordwithnobreaks",
			width: 10,
			want:  []string{"verylongwo", "rdwithnobr", "eaks"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapText(tt.text, tt.width)
			assert.Equal(t, tt.want, got)

			if tt.width > 0 {
				for i, line := range got {
					assert.LessOrEqual(t, len(line), tt.width,
						"line %d should not exceed width: %q", i, line)
				}
			}
		})
	}
}

func TestGetLastLines(t *testing.T) {
	tests := []struct {
		name string
		text string
		n    int
		want []string
	}{
		{
			name: "empty text",
			text: "",
			n:    5,
			want: nil,
		},
		{
			name: "fewer lines than n",
			text: "line1\nline2\nline3",
			n:    5,
			want: []string{"line1", "line2", "line3"},
		},
		{
			name: "exact n lines",
			text: "line1\nline2\nline3",
			n:    3,
			want: []string{"line1", "line2", "line3"},
		},
		{
			name: "more lines than n",
			text: "line1\nline2\nline3\nline4\nline5",
			n:    3,
			want: []string{"line3", "line4", "line5"},
		},
		{
			name: "text with trailing newline",
			text: "line1\nline2\nline3\n",
			n:    2,
			want: []string{"line2", "line3"},
		},
		{
			name: "single line",
			text: "single line",
			n:    5,
			want: []string{"single line"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getLastLines(tt.text, tt.n)
			assert.Equal(t, tt.want, got)
			if tt.want != nil {
				assert.LessOrEqual(t, len(got), tt.n,
					"should return at most n lines")
			}
		})
	}
}

func TestRenderFailureSummary(t *testing.T) {
	tests := []struct {
		name     string
		failures []FailureInfo
		width    int
		wantText []string
	}{
		{
			name:     "empty failures",
			failures: []FailureInfo{},
			width:    60,
			wantText: nil,
		},
		{
			name: "single failure",
			failures: []FailureInfo{
				{
					TaskName: "Install package",
					Error:    errors.New("not found"),
				},
			},
			width: 60,
			wantText: []string{
				"FAILURES (1)",
				"✗ Install package",
				"not found",
			},
		},
		{
			name: "multiple failures",
			failures: []FailureInfo{
				{
					TaskName: "Task 1",
					Error:    errors.New("error 1"),
				},
				{
					TaskName: "Task 2",
					Error:    errors.New("error 2"),
				},
				{
					TaskName: "Task 3",
					Error:    errors.New("error 3"),
				},
			},
			width: 60,
			wantText: []string{
				"FAILURES (3)",
				"✗ Task 1",
				"error 1",
				"✗ Task 2",
				"error 2",
				"✗ Task 3",
				"error 3",
			},
		},
		{
			name: "failure with multiline error",
			failures: []FailureInfo{
				{
					TaskName: "Complex task",
					Error:    errors.New("first line\nsecond line\nthird line"),
				},
			},
			width: 60,
			wantText: []string{
				"FAILURES (1)",
				"✗ Complex task",
				"first line",
			},
		},
		{
			name: "failure with nil error",
			failures: []FailureInfo{
				{
					TaskName: "Task without error",
					Error:    nil,
				},
			},
			width: 60,
			wantText: []string{
				"FAILURES (1)",
				"✗ Task without error",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := RenderFailureSummary(tt.failures, tt.width)

			if len(tt.failures) == 0 {
				assert.Empty(t, output, "empty failures should produce empty output")
				return
			}

			assert.NotEmpty(t, output, "output should not be empty")

			if tt.wantText != nil {
				for _, want := range tt.wantText {
					assert.Contains(t, output, want,
						"output should contain %q", want)
				}
			}
		})
	}
}

func TestRenderFailureWithDuration(t *testing.T) {
	info := FailureInfo{
		TaskName: "Long running task",
		Error:    errors.New("timeout"),
		Duration: 5 * time.Second,
	}

	output := RenderFailure(info, 60)

	assert.Contains(t, output, "FAILED")
	assert.Contains(t, output, "Long running task")
	assert.Contains(t, output, "timeout")

	assert.NotEmpty(t, output)
}

func stripAnsiSimple(s string) string {
	result := ""
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		result += string(r)
	}
	return result
}
