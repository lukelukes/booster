package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

func TestRenderPanel(t *testing.T) {
	tests := []struct {
		name     string
		panel    Panel
		validate func(t *testing.T, result string)
	}{
		{
			name: "panel with title and content",
			panel: Panel{
				Title:       "Test Title",
				Content:     "Test content",
				Width:       20,
				Height:      5,
				BorderColor: DefaultBorderColor,
			},
			validate: func(t *testing.T, result string) {
				assert.Contains(t, result, "Test Title")
				assert.Contains(t, result, "Test content")
				assert.NotEmpty(t, result)
			},
		},
		{
			name: "panel without title",
			panel: Panel{
				Title:       "",
				Content:     "Content only",
				Width:       20,
				Height:      5,
				BorderColor: DefaultBorderColor,
			},
			validate: func(t *testing.T, result string) {
				assert.Contains(t, result, "Content only")
				assert.NotEmpty(t, result)
			},
		},
		{
			name: "panel with empty content",
			panel: Panel{
				Title:       "Empty Panel",
				Content:     "",
				Width:       20,
				Height:      5,
				BorderColor: DefaultBorderColor,
			},
			validate: func(t *testing.T, result string) {
				assert.Contains(t, result, "Empty Panel")
				assert.NotEmpty(t, result)
			},
		},
		{
			name: "content truncation when exceeds width",
			panel: Panel{
				Title:       "Wide",
				Content:     "This is a very long line that should be truncated when it exceeds the panel width",
				Width:       20,
				Height:      5,
				BorderColor: DefaultBorderColor,
			},
			validate: func(t *testing.T, result string) {
				lines := strings.SplitSeq(result, "\n")

				for line := range lines {
					visibleWidth := lipgloss.Width(line)
					assert.LessOrEqual(t, visibleWidth, 20, "line should not exceed panel width")
				}
			},
		},
		{
			name: "height constraints work correctly",
			panel: Panel{
				Title:       "Tall",
				Content:     "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\nLine 6\nLine 7\nLine 8",
				Width:       20,
				Height:      5,
				BorderColor: DefaultBorderColor,
			},
			validate: func(t *testing.T, result string) {
				lines := strings.Split(result, "\n")
				height := len(lines)
				assert.LessOrEqual(t, height, 5, "rendered height should not exceed specified height")
			},
		},
		{
			name: "custom border color",
			panel: Panel{
				Title:       "Custom",
				Content:     "Content",
				Width:       20,
				Height:      5,
				BorderColor: lipgloss.Color("#FF0000"),
			},
			validate: func(t *testing.T, result string) {
				assert.NotEmpty(t, result)
				assert.Contains(t, result, "Custom")
			},
		},
		{
			name: "zero width edge case",
			panel: Panel{
				Title:       "Zero",
				Content:     "Content",
				Width:       0,
				Height:      5,
				BorderColor: DefaultBorderColor,
			},
			validate: func(t *testing.T, result string) {
				assert.NotEmpty(t, result)
			},
		},
		{
			name: "negative dimensions edge case",
			panel: Panel{
				Title:       "Negative",
				Content:     "Content",
				Width:       -10,
				Height:      -5,
				BorderColor: DefaultBorderColor,
			},
			validate: func(t *testing.T, result string) {
				assert.NotEmpty(t, result)
			},
		},
		{
			name: "multiline content fits within panel",
			panel: Panel{
				Title:       "Multi",
				Content:     "Line 1\nLine 2\nLine 3",
				Width:       25,
				Height:      6,
				BorderColor: DefaultBorderColor,
			},
			validate: func(t *testing.T, result string) {
				assert.Contains(t, result, "Line 1")
				assert.Contains(t, result, "Line 2")
				assert.Contains(t, result, "Line 3")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderPanel(tt.panel)
			tt.validate(t, result)
		})
	}
}

func TestDefaultBorderColor(t *testing.T) {
	assert.Equal(t, lipgloss.Color("#808080"), DefaultBorderColor)
}

func TestPanelStructFields(t *testing.T) {
	p := Panel{
		Title:       "Test",
		Content:     "Content",
		Width:       30,
		Height:      10,
		BorderColor: lipgloss.Color("#AABBCC"),
	}

	assert.Equal(t, "Test", p.Title)
	assert.Equal(t, "Content", p.Content)
	assert.Equal(t, 30, p.Width)
	assert.Equal(t, 10, p.Height)
	assert.Equal(t, lipgloss.Color("#AABBCC"), p.BorderColor)
}
