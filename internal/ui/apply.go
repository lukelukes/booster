package ui

import (
	"booster/internal/model"
	"fmt"
	"io"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	tailLines        = 4
	tailIndent       = "      │ "
	applyChrome      = 5
	percentLabelSize = 6
)

func Apply(in io.Reader, out io.Writer, p model.Plan, log string, events <-chan model.Event, stop func()) error {
	defer stop()
	m := newApplyScreen(lipgloss.NewRenderer(out), p, log, events, stop, time.Now)
	_, err := tea.NewProgram(m, tea.WithInput(in), tea.WithOutput(out), tea.WithoutSignalHandler()).Run()
	return err
}

type (
	eventMsg struct{ event model.Event }
	runEnded struct{}
	tickMsg  struct{}
)

type applyScreen struct {
	started  []time.Time
	runStart time.Time
	clock    time.Time
	now      func() time.Time
	stop     func()
	events   <-chan model.Event
	results  []*model.Result
	summary  *model.Summary
	log      string
	plan     model.Plan
	tail     []string
	st       styles
	running  int
	offset   int
	width    int
	height   int
	hideTail bool
	stopping bool
}

func newApplyScreen(r *lipgloss.Renderer, p model.Plan, log string, events <-chan model.Event, stop func(), now func() time.Time) *applyScreen {
	return &applyScreen{
		plan:    p,
		log:     log,
		events:  events,
		stop:    stop,
		now:     now,
		clock:   now(),
		st:      newStyles(r),
		running: -1,
		started: make([]time.Time, len(p.Steps)),
		results: make([]*model.Result, len(p.Steps)),
	}
}

func (a *applyScreen) Init() tea.Cmd {
	return tea.Batch(wait(a.events), tick())
}

func (a *applyScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.resize(msg.Width, msg.Height)
	case eventMsg:
		a.event(msg.event, a.now())
		return a, wait(a.events)
	case runEnded:
		return a, tea.Quit
	case tickMsg:
		if !a.finished() {
			a.clock = a.now()
			return a, tick()
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "o":
			a.hideTail = !a.hideTail
			a.offset = a.scroll()
		case "ctrl+c":
			if !a.finished() {
				a.stopping = true
				a.stop()
			}
		}
	}
	return a, nil
}

func wait(events <-chan model.Event) tea.Cmd {
	return func() tea.Msg {
		e, ok := <-events
		if !ok {
			return runEnded{}
		}
		return eventMsg{e}
	}
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return tickMsg{} })
}

func (a *applyScreen) event(e model.Event, now time.Time) {
	a.clock = now
	switch e := e.(type) {
	case model.RunStart:
		a.runStart = now
	case model.TaskStart:
		if a.valid(e.Index) {
			a.running, a.started[e.Index], a.tail = e.Index, now, nil
		}
	case model.TaskOutput:
		if e.Index == a.running {
			a.tail = append(a.tail, e.Line)
			a.tail = a.tail[max(len(a.tail)-tailLines, 0):]
		}
	case model.TaskDone:
		if a.valid(e.Index) {
			a.results[e.Index] = &e.Result
		}
		if e.Index == a.running {
			a.running, a.tail = -1, nil
		}
	case model.RunDone:
		a.summary = &e.Summary
		a.running, a.tail = -1, nil
	}
	a.offset = a.scroll()
}

func (a *applyScreen) valid(i int) bool {
	return i >= 0 && i < len(a.plan.Steps)
}

func (a *applyScreen) resize(width, height int) {
	a.width, a.height = width, height
	a.offset = a.scroll()
}

func (a *applyScreen) finished() bool {
	return a.summary != nil
}

func (a *applyScreen) lineWidth() int {
	if a.width == 0 {
		return screenWidth
	}
	return min(a.width, screenWidth)
}

func (a *applyScreen) row(left, right []seg, selected bool) string {
	return renderRow(a.st, a.lineWidth(), left, right, selected)
}

func (a *applyScreen) scroll() int {
	lines, first, last := a.body()
	return scrollOffset(a.offset, len(lines), first, last, viewport(a.height, applyChrome))
}

func (a *applyScreen) View() string {
	lines, _, _ := a.body()
	lines = visible(lines, a.offset, viewport(a.height, applyChrome))

	var b strings.Builder
	b.WriteString(a.headerLine())
	b.WriteString("\n\n")
	for _, l := range lines {
		b.WriteString(l)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(a.progressLine())
	b.WriteString("\n")
	b.WriteString(a.logLine())
	b.WriteString("\n")
	return b.String()
}

func (a *applyScreen) changes() (total, finished int) {
	for i, s := range a.plan.Steps {
		if s.Change.Kind == model.ChangeNone {
			continue
		}
		total++
		if r := a.results[i]; r != nil && r.Status != model.StatusSkipped {
			finished++
		}
	}
	return total, finished
}

func (a *applyScreen) elapsed() time.Duration {
	if a.summary != nil {
		return a.summary.Duration
	}
	if a.runStart.IsZero() {
		return 0
	}
	return a.clock.Sub(a.runStart)
}

func (a *applyScreen) headerLine() string {
	st := a.st
	total, finished := a.changes()
	left := []seg{{" booster", st.accentBold}, {"  ", st.text}}
	right := []seg{{formatDuration(a.elapsed()), st.secondary}}

	switch s := a.summary; {
	case s != nil && s.Failed > 0:
		left = append(left, seg{"stopped at ", st.text}, seg{a.failedName(), st.fault})
	case s != nil && s.Skipped > 0:
		left = append(left, seg{fmt.Sprintf("stopped after %d of %s", s.Changed, plural(total, "change")), st.text})
	case s != nil:
		left = append(left, seg{"applied " + plural(s.Changed, "change"), st.text})
	default:
		current := finished
		if a.running >= 0 && a.plan.Steps[a.running].Change.Kind != model.ChangeNone {
			current++
		}
		left = append(left, seg{"applying " + plural(total, "change"), st.text})
		right = append([]seg{{fmt.Sprintf("%d of %d · ", max(current, 1), total), st.secondary}}, right...)
	}
	return a.row(left, right, false)
}

func (a *applyScreen) failedName() string {
	for i, r := range a.results {
		if r != nil && r.Status == model.StatusFailed {
			return a.plan.Steps[i].Name
		}
	}
	return ""
}

func (a *applyScreen) body() (lines []string, first, last int) {
	nameWidth := minNameWidth
	for _, s := range a.plan.Steps {
		nameWidth = max(nameWidth, lipgloss.Width(s.Name)+2)
	}

	focus := a.running
	for i, s := range a.plan.Steps {
		row := len(lines)
		lines = append(lines, a.stepLine(i, s, nameWidth))
		switch r := a.results[i]; {
		case i == a.running && !a.hideTail:
			lines = append(lines, a.tailLines(a.tail, a.st.dim, a.st.secondary)...)
		case r != nil && r.Status == model.StatusFailed && r.Err != nil:
			lines = append(lines, a.tailLines(strings.Split(r.Err.Error(), "\n"), a.st.fault, a.st.text)...)
			if focus < 0 {
				focus = i
			}
		}
		if i == focus {
			first, last = row, len(lines)-1
		}
	}
	return lines, first, last
}

func (a *applyScreen) stepLine(i int, s model.PlannedStep, nameWidth int) string {
	st := a.st
	r := a.results[i]
	symbol, symbolStyle, nameStyle, descStyle := "·", st.dimBold, st.dim, st.dim
	var right []seg

	switch {
	case i == a.running:
		symbol, symbolStyle, nameStyle, descStyle = "●", st.accentBold, st.textBold, st.secondary
		right = []seg{{formatDuration(a.clock.Sub(a.started[i])), st.secondary}}
	case r != nil && r.Status == model.StatusSkipped:
	case s.Change.Kind == model.ChangeNone:
		symbol, symbolStyle = "✓", st.okBold
	case r != nil && r.Status == model.StatusFailed:
		symbol, symbolStyle, nameStyle, descStyle = "✗", st.faultBold, st.textBold, st.secondary
		right = []seg{{formatDuration(r.Duration), st.fault}}
	case r != nil:
		symbol, symbolStyle, nameStyle, descStyle = "✓", st.okBold, st.textBold, st.secondary
		right = []seg{{formatDuration(r.Duration), st.secondary}}
	}

	left := []seg{
		{noCursorSpace, st.text},
		{symbol + "  ", symbolStyle},
		{pad(s.Name, nameWidth), nameStyle},
		{stepDescription(s.Change), descStyle},
	}
	return a.row(left, right, i == a.running)
}

func (a *applyScreen) tailLines(lines []string, bar, text lipgloss.Style) []string {
	width := max(a.lineWidth()-ansi.StringWidth(tailIndent)-1, 1)
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		l = ansi.Truncate(strings.ReplaceAll(ansi.Strip(l), "\t", "    "), width, "…")
		out = append(out, a.row([]seg{{tailIndent, bar}, {l, text}}, nil, false))
	}
	return out
}

func (a *applyScreen) progressLine() string {
	st := a.st
	if s := a.summary; s != nil {
		left := []seg{
			{fmt.Sprintf(" %d changed", s.Changed), st.accentBold},
			{" · ", st.dim},
			{fmt.Sprintf("%d up to date", s.OK), st.ok},
		}
		if s.Failed > 0 {
			left = append(left, seg{" · ", st.dim}, seg{fmt.Sprintf("%d failed", s.Failed), st.fault})
		}
		if s.Skipped > 0 {
			left = append(left, seg{" · ", st.dim}, seg{fmt.Sprintf("%d not run", s.Skipped), st.dim})
		}
		return a.row(left, nil, false)
	}

	total, finished := a.changes()
	percent := 100
	if total > 0 {
		percent = finished * 100 / total
	}
	width := max(a.lineWidth()-2-percentLabelSize, 1)
	filled := width * percent / 100
	return a.row([]seg{
		{" ", st.text},
		{strings.Repeat("━", filled), st.accent},
		{strings.Repeat("─", width-filled), st.dim},
		{fmt.Sprintf("%*d%%", percentLabelSize-1, percent), st.textBold},
	}, nil, false)
}

func (a *applyScreen) logLine() string {
	st := a.st
	var right []seg
	switch {
	case a.finished():
	case a.stopping:
		right = []seg{{"stopping…", st.accent}}
	default:
		right = []seg{
			{"o", st.textBold},
			{" output", st.secondary},
			{"   ", st.secondary},
			{"ctrl+c", st.textBold},
			{" stop", st.secondary},
		}
	}
	label := " logs "
	room := a.lineWidth() - 1 - ansi.StringWidth(label) - segWidth(right) - 3
	path := a.log
	if over := ansi.StringWidth(path) - room; over > 0 {
		path = ansi.TruncateLeft(path, over+1, "…")
	}
	return a.row([]seg{{label, st.dim}, {path, st.secondary}}, right, false)
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	switch {
	case d >= time.Hour:
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	case d >= time.Minute:
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	default:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
}

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
