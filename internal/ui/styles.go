package ui

import "github.com/charmbracelet/lipgloss"

var (
	colorText      = lipgloss.AdaptiveColor{Dark: "#E8E6E1", Light: "#1A1B1E"}
	colorSecondary = lipgloss.AdaptiveColor{Dark: "#8A8D93", Light: "#5A5D63"}
	colorDim       = lipgloss.AdaptiveColor{Dark: "#5A5D63", Light: "#8A8D93"}
	colorAccent    = lipgloss.AdaptiveColor{Dark: "#FFB224", Light: "#B26B00"}
	colorOK        = lipgloss.AdaptiveColor{Dark: "#5FD38D", Light: "#1E8A4C"}
	colorFault     = lipgloss.AdaptiveColor{Dark: "#FF5A4E", Light: "#C8352B"}
	colorCursor    = lipgloss.AdaptiveColor{Dark: "#1B1D22", Light: "#ECEAE4"}
)

type styles struct {
	cursorBackground lipgloss.TerminalColor
	text             lipgloss.Style
	textBold         lipgloss.Style
	secondary        lipgloss.Style
	secondaryBold    lipgloss.Style
	dim              lipgloss.Style
	dimBold          lipgloss.Style
	accent           lipgloss.Style
	accentBold       lipgloss.Style
	ok               lipgloss.Style
	okBold           lipgloss.Style
	fault            lipgloss.Style
	faultBold        lipgloss.Style
}

func newStyles(r *lipgloss.Renderer) styles {
	fg := func(c lipgloss.TerminalColor) lipgloss.Style { return r.NewStyle().Foreground(c) }
	return styles{
		cursorBackground: colorCursor,
		text:             fg(colorText),
		textBold:         fg(colorText).Bold(true),
		secondary:        fg(colorSecondary),
		secondaryBold:    fg(colorSecondary).Bold(true),
		dim:              fg(colorDim),
		dimBold:          fg(colorDim).Bold(true),
		accent:           fg(colorAccent),
		accentBold:       fg(colorAccent).Bold(true),
		ok:               fg(colorOK),
		okBold:           fg(colorOK).Bold(true),
		fault:            fg(colorFault),
		faultBold:        fg(colorFault).Bold(true),
	}
}
