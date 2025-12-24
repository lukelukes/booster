package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Panel struct {
	Title       string
	Content     string
	Width       int
	Height      int
	BorderColor lipgloss.Color
	Focused     bool
}

var DefaultBorderColor = lipgloss.Color("#808080")

func RenderPanel(p Panel) string {
	width := p.Width
	height := p.Height

	if width <= 0 {
		width = 10
	}
	if height <= 0 {
		height = 3
	}

	contentWidth := max(width-2, 1)
	contentHeight := max(height-2, 1)

	processedContent := processContent(p.Content, contentWidth, contentHeight)

	if !p.Focused {
		processedContent = lipgloss.NewStyle().Faint(true).Render(processedContent)
	}

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.BorderColor).
		Width(contentWidth).
		Height(contentHeight)

	rendered := borderStyle.Render(processedContent)

	if p.Title != "" {
		rendered = insertTitleInBorder(rendered, p.Title, width)
	}

	return rendered
}

func processContent(content string, maxWidth, maxHeight int) string {
	if content == "" {
		return ""
	}

	lines := strings.Split(content, "\n")
	var processedLines []string

	truncateStyle := lipgloss.NewStyle().MaxWidth(maxWidth)

	for _, line := range lines {
		if lipgloss.Width(line) > maxWidth {
			line = truncateStyle.Render(line)
		}
		processedLines = append(processedLines, line)

		if len(processedLines) >= maxHeight {
			break
		}
	}

	if len(processedLines) > maxHeight {
		processedLines = processedLines[:maxHeight]
	}

	return strings.Join(processedLines, "\n")
}

func insertTitleInBorder(rendered, title string, totalWidth int) string {
	lines := strings.Split(rendered, "\n")
	if len(lines) == 0 {
		return rendered
	}

	topBorder := lines[0]
	colorPrefix := extractANSIPrefix(topBorder)
	colorSuffix := extractANSISuffix(topBorder)

	titleWithSpacing := " " + title + " "
	titleLen := lipgloss.Width(titleWithSpacing)
	maxTitleLen := max(totalWidth-4, 1)

	if titleLen > maxTitleLen {
		if maxTitleLen > 3 {
			titleWithSpacing = " " + title[:maxTitleLen-3] + "… "
		} else {
			titleWithSpacing = " "
		}
		titleLen = len(titleWithSpacing)
	}

	remainingDashes := max(totalWidth-titleLen-3, 0)
	newTopBorder := colorPrefix + "╭─" + titleWithSpacing + strings.Repeat("─", remainingDashes) + "╮" + colorSuffix

	lines[0] = newTopBorder
	return strings.Join(lines, "\n")
}

func extractANSIPrefix(s string) string {
	var prefix strings.Builder
	inEscape := false

	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			inEscape = true
		}
		if inEscape {
			prefix.WriteByte(s[i])
			if s[i] == 'm' {
				inEscape = false
			}
		} else {
			break
		}
	}

	return prefix.String()
}

func extractANSISuffix(s string) string {
	if strings.HasSuffix(s, "\x1b[0m") {
		return "\x1b[0m"
	}
	return ""
}
