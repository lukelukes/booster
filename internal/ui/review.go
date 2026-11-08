package ui

import (
	"booster/internal/model"
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Mode int

const (
	ModeApply Mode = iota
	ModeDrift
)

type Header struct {
	OS      string
	Arch    string
	Profile string
	Config  string
}

type Decision struct {
	Skip  []int
	Apply bool
}

const (
	screenWidth   = 100
	minNameWidth  = 20
	chromeHeight  = 6
	itemIndent    = "        "
	cursorMarker  = " ▸ "
	noCursorSpace = "   "
)

func Review(in io.Reader, out io.Writer, h Header, p model.Plan, mode Mode) (Decision, error) {
	m := newReview(lipgloss.NewRenderer(out), h, p, mode)
	final, err := tea.NewProgram(m, tea.WithInput(in), tea.WithOutput(out)).Run()
	if err != nil {
		return Decision{}, err
	}
	return final.(review).decision, nil
}

type review struct {
	header   Header
	plan     model.Plan
	expanded []bool
	skipped  []bool
	decision Decision
	st       styles
	mode     Mode
	cursor   int
	offset   int
	width    int
	height   int
}

func newReview(r *lipgloss.Renderer, h Header, p model.Plan, mode Mode) review {
	return review{
		header:   h,
		plan:     p,
		mode:     mode,
		st:       newStyles(r),
		expanded: make([]bool, len(p.Steps)),
		skipped:  make([]bool, len(p.Steps)),
	}
}

func (m review) Init() tea.Cmd { return nil }

func (m review) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "enter":
			m.decision = m.decide()
			return m, tea.Quit
		case "up", "k":
			m.cursor = max(m.cursor-1, 0)
		case "down", "j":
			m.cursor = min(m.cursor+1, len(m.plan.Steps)-1)
		case "right", "l":
			if m.cursor < len(m.plan.Steps) && len(m.plan.Steps[m.cursor].Change.Items) > 0 {
				m.expanded[m.cursor] = true
			}
		case "left", "h":
			if m.cursor < len(m.plan.Steps) {
				m.expanded[m.cursor] = false
			}
		case " ":
			if m.cursor < len(m.plan.Steps) && m.plan.Steps[m.cursor].Change.Kind != model.ChangeNone {
				m.skipped[m.cursor] = !m.skipped[m.cursor]
			}
		}
	}
	m.offset = m.scroll()
	return m, nil
}

func (m review) decide() Decision {
	var d Decision
	for i, s := range m.plan.Steps {
		switch {
		case m.skipped[i]:
			d.Skip = append(d.Skip, i)
		case s.Change.Kind != model.ChangeNone:
			d.Apply = true
		}
	}
	return d
}

func (m review) scroll() int {
	lines, first, last := m.body()
	avail := m.height - chromeHeight
	if m.height == 0 || len(lines) <= avail {
		return 0
	}
	offset := min(m.offset, len(lines)-avail)
	if last-offset >= avail {
		offset = last - avail + 1
	}
	if first < offset {
		offset = first
	}
	return offset
}

func (m review) View() string {
	lines, _, _ := m.body()
	if m.height > 0 && len(lines) > m.height-chromeHeight {
		lines = lines[m.offset:min(m.offset+max(m.height-chromeHeight, 1), len(lines))]
	}

	var b strings.Builder
	b.WriteString(m.headerLine())
	b.WriteString("\n\n")
	for _, l := range lines {
		b.WriteString(l)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(m.summaryLine())
	b.WriteString("\n\n")
	b.WriteString(m.hintsLine())
	b.WriteString("\n")
	return b.String()
}

func (m review) lineWidth() int {
	if m.width == 0 {
		return screenWidth
	}
	return min(m.width, screenWidth)
}

func (m review) headerLine() string {
	st := m.st
	left := []seg{
		{" booster", st.accentBold},
		{fmt.Sprintf("  %s · %s", m.header.OS, m.header.Arch), st.secondary},
	}
	if m.header.Profile != "" {
		left = append(left, seg{" · profile ", st.secondary}, seg{m.header.Profile, st.text})
	}
	left = append(left, seg{" · " + m.header.Config, st.secondary})
	return m.row(left, []seg{{"plan", st.secondary}}, false)
}

func (m review) body() (lines []string, first, last int) {
	nameWidth := minNameWidth
	for _, s := range m.plan.Steps {
		nameWidth = max(nameWidth, len(s.Name)+2)
	}

	for i, s := range m.plan.Steps {
		if i == m.cursor {
			first = len(lines)
		}
		lines = append(lines, m.stepLine(i, s, nameWidth))
		if m.expanded[i] {
			lines = append(lines, m.itemLines(s.Change.Items)...)
		}
		if i == m.cursor {
			last = len(lines) - 1
		}
	}
	return lines, first, last
}

func (m review) stepLine(i int, s model.PlannedStep, nameWidth int) string {
	st := m.st
	selected := i == m.cursor
	prefix := noCursorSpace
	if selected {
		prefix = cursorMarker
	}

	symbol, symbolStyle, nameStyle, descStyle := "+", st.accentBold, st.textBold, st.secondary
	switch {
	case m.skipped[i]:
		symbol, symbolStyle, nameStyle, descStyle = "-", st.dimBold, st.dim, st.dim
	case s.Change.Kind == model.ChangeUpdate:
		symbol = "~"
	case s.Change.Kind == model.ChangeUnknown:
		symbol, symbolStyle, nameStyle, descStyle = "?", st.secondaryBold, st.dim, st.dim
	case s.Change.Kind == model.ChangeNone:
		symbol, symbolStyle, nameStyle, descStyle = "✓", st.okBold, st.dim, st.dim
	}

	left := []seg{
		{prefix, st.accentBold},
		{symbol + "  ", symbolStyle},
		{pad(s.Name, nameWidth), nameStyle},
		{stepDescription(s.Change), descStyle},
	}
	return m.row(left, m.annotations(i, s.Change), selected)
}

func stepDescription(c model.Change) string {
	switch c.Kind {
	case model.ChangeNone:
		return "up to date"
	case model.ChangeUnknown:
		return "known after apply"
	}
	if len(c.Items) == 0 {
		return c.Verb
	}
	return fmt.Sprintf("%s %d", c.Verb, len(c.Items))
}

func (m review) annotations(i int, c model.Change) []seg {
	st := m.st
	if m.skipped[i] {
		return []seg{{"skipped", st.dim}}
	}

	var notes []seg
	add := func(s seg) {
		if len(notes) > 0 {
			notes = append(notes, seg{" · ", st.dim})
		}
		notes = append(notes, s)
	}
	if c.Kind != model.ChangeNone {
		if ok := c.Total - len(c.Items); ok > 0 && len(c.Items) > 0 {
			add(seg{fmt.Sprintf("%d ok", ok), st.secondary})
		}
		if c.NeedsSudo {
			add(seg{"needs sudo", st.accent})
		}
	}
	if m.expanded[i] {
		add(seg{"← collapse", st.dim})
	}
	return notes
}

func (m review) itemLines(items []model.Item) []string {
	st := m.st
	width := 0
	for _, it := range items {
		width = max(width, len(it.Name)+2)
	}

	lines := make([]string, 0, len(items))
	for _, it := range items {
		symbol := "+ "
		if it.From != "" {
			symbol = "~ "
		}
		left := []seg{{itemIndent, st.text}, {symbol, st.accentBold}}
		switch {
		case it.From != "":
			left = append(left, seg{pad(it.Name, width), st.text}, seg{it.From, st.fault}, seg{" → ", st.dim}, seg{it.To, st.secondary})
		case it.To != "":
			left = append(left, seg{pad(it.Name, width), st.text}, seg{"→ ", st.dim}, seg{it.To, st.secondary})
		default:
			left = append(left, seg{it.Name, st.text})
		}
		lines = append(lines, m.row(left, nil, false))
	}
	return lines
}

func (m review) summaryLine() string {
	st := m.st
	var change, ok, unknown, skipped int
	for i, s := range m.plan.Steps {
		switch {
		case m.skipped[i]:
			skipped++
		case s.Change.Kind == model.ChangeNone:
			ok++
		case s.Change.Kind == model.ChangeUnknown:
			unknown++
		default:
			change++
		}
	}

	label := "to change"
	if m.mode == ModeDrift {
		label = "drifted"
	}
	left := []seg{
		{fmt.Sprintf(" %d %s", change, label), st.accentBold},
		{" · ", st.dim},
		{fmt.Sprintf("%d up to date", ok), st.ok},
	}
	if unknown > 0 {
		left = append(left, seg{" · ", st.dim}, seg{fmt.Sprintf("%d after apply", unknown), st.secondary})
	}
	if skipped > 0 {
		left = append(left, seg{" · ", st.dim}, seg{fmt.Sprintf("%d skipped", skipped), st.dim})
	}

	var right []seg
	if m.mode == ModeDrift {
		code := 0
		if m.plan.HasChanges() {
			code = 2
		}
		right = []seg{{fmt.Sprintf("exit %d", code), st.dim}}
	} else if m.plan.Excluded > 0 {
		right = []seg{{fmt.Sprintf("%d excluded", m.plan.Excluded), st.dim}}
	}
	return m.row(left, right, false)
}

func (m review) hintsLine() string {
	st := m.st
	gap := seg{"   ", st.secondary}
	if m.mode == ModeDrift {
		return m.row([]seg{
			{" enter", st.accentBold},
			{" apply fixes", st.secondary},
			gap,
			{"→", st.textBold},
			{" details", st.secondary},
			gap,
			{"q", st.textBold},
			{" quit", st.secondary},
		}, nil, false)
	}
	return m.row([]seg{
		{" ↑↓", st.textBold},
		{" move", st.secondary},
		gap,
		{"→", st.textBold},
		{" details", st.secondary},
		gap,
		{"space", st.textBold},
		{" skip", st.secondary},
		gap,
		{"enter", st.accentBold},
		{" apply", st.secondary},
		gap,
		{"q", st.textBold},
		{" quit", st.secondary},
	}, nil, false)
}

type seg struct {
	text  string
	style lipgloss.Style
}

func (m review) row(left, right []seg, selected bool) string {
	width := m.lineWidth()
	used := segWidth(left) + segWidth(right)

	segs := left
	if len(right) > 0 {
		segs = append(segs, seg{strings.Repeat(" ", max(width-1-used, 1)), m.st.text})
		segs = append(segs, right...)
	}
	if selected {
		if fill := width - segWidth(segs); fill > 0 {
			segs = append(segs, seg{strings.Repeat(" ", fill), m.st.text})
		}
	}

	var b strings.Builder
	for _, s := range segs {
		style := s.style
		if selected {
			style = style.Background(m.st.cursorBackground)
		}
		b.WriteString(style.Render(s.text))
	}
	return strings.TrimRight(b.String(), " ")
}

func segWidth(segs []seg) int {
	w := 0
	for _, s := range segs {
		w += lipgloss.Width(s.text)
	}
	return w
}

func pad(s string, width int) string {
	return s + strings.Repeat(" ", max(width-lipgloss.Width(s), 1))
}
