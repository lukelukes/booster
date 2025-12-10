package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Panel represents a bordered panel with optional title and content.
type Panel struct {
	Title       string
	Content     string
	Width       int
	Height      int
	BorderColor lipgloss.Color
	Focused     bool // When false, content is dimmed
}

// DefaultBorderColor is the default panel border color.
var DefaultBorderColor = lipgloss.Color("#808080")

// RenderPanel renders a bordered panel with optional title.
// The title appears inline with the top border.
func RenderPanel(p Panel) string {
	// Handle edge cases for dimensions
	width := p.Width
	height := p.Height

	// Ensure minimum dimensions to avoid rendering issues
	if width < 0 {
		width = 10
	}
	if height < 0 {
		height = 3
	}
	if width == 0 {
		width = 10
	}
	if height == 0 {
		height = 3
	}

	// Calculate content dimensions
	// Border takes 2 characters width (left + right) and 2 lines height (top + bottom)
	contentWidth := max(width-2, 1)
	contentHeight := max(height-2, 1)

	// Process content: wrap and truncate as needed
	processedContent := processContent(p.Content, contentWidth, contentHeight)

	// Apply dimming to unfocused panel content (like Ghostty split panes)
	if !p.Focused {
		processedContent = lipgloss.NewStyle().Faint(true).Render(processedContent)
	}

	// Create the bordered box
	// lipgloss Width/Height on a bordered style controls the INNER content area
	// So we use contentWidth/contentHeight directly
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.BorderColor).
		Width(contentWidth).
		Height(contentHeight)

	// Render the panel - border style will constrain content to contentWidth x contentHeight
	rendered := borderStyle.Render(processedContent)

	// If we have a title, insert it into the top border
	if p.Title != "" {
		rendered = insertTitleInBorder(rendered, p.Title, width)
	}

	return rendered
}

// processContent handles content wrapping and truncation
// Uses lipgloss.Width() for ANSI-aware width measurement to avoid breaking escape sequences.
func processContent(content string, maxWidth, maxHeight int) string {
	if content == "" {
		return ""
	}

	lines := strings.Split(content, "\n")
	var processedLines []string

	// Create a truncation style that handles ANSI codes correctly
	truncateStyle := lipgloss.NewStyle().MaxWidth(maxWidth)

	for _, line := range lines {
		// Use lipgloss.Width for ANSI-aware width measurement
		visualWidth := lipgloss.Width(line)
		if visualWidth > maxWidth {
			// Use lipgloss MaxWidth to truncate - this handles ANSI codes correctly
			line = truncateStyle.Render(line)
		}
		processedLines = append(processedLines, line)

		// Stop if we've reached max height
		if len(processedLines) >= maxHeight {
			break
		}
	}

	// Truncate to max height
	if len(processedLines) > maxHeight {
		processedLines = processedLines[:maxHeight]
	}

	return strings.Join(processedLines, "\n")
}

// insertTitleInBorder inserts the title into the top border of the rendered panel
func insertTitleInBorder(rendered, title string, totalWidth int) string {
	lines := strings.Split(rendered, "\n")
	if len(lines) == 0 {
		return rendered
	}

	// Find the first line (top border)
	topBorder := lines[0]

	// Extract ANSI color codes
	colorPrefix := extractANSIPrefix(topBorder)
	colorSuffix := extractANSISuffix(topBorder)

	// Calculate title with spacing
	titleWithSpacing := " " + title + " "
	titleLen := lipgloss.Width(titleWithSpacing) // Use visual width for proper unicode handling

	// Calculate available space for title (total width - 2 corners - 2 minimum dashes)
	maxTitleLen := max(totalWidth-4, 1)

	// Truncate title if needed
	if titleLen > maxTitleLen {
		if maxTitleLen > 3 {
			titleWithSpacing = " " + title[:maxTitleLen-3] + "… "
		} else {
			titleWithSpacing = " "
		}
		titleLen = len(titleWithSpacing)
	}

	// Build new top border: "╭─" + title + remaining dashes + "╮"
	// Total width = corner(1) + dash(1) + title + dashes + corner(1)
	// So: dashes = totalWidth - 2 (corners) - 1 (first dash) - titleLen
	remainingDashes := max(totalWidth-titleLen-3, 0)

	newTopBorder := colorPrefix + "╭─" + titleWithSpacing + strings.Repeat("─", remainingDashes) + "╮" + colorSuffix

	// Replace first line
	lines[0] = newTopBorder

	return strings.Join(lines, "\n")
}

// extractANSIPrefix extracts ANSI escape codes from the beginning of a string
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

// extractANSISuffix extracts ANSI reset codes from the end of a string
func extractANSISuffix(s string) string {
	// Look for reset sequence at the end
	if strings.HasSuffix(s, "\x1b[0m") {
		return "\x1b[0m"
	}
	return ""
}
