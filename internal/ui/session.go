package ui

import (
	"booster/internal/model"
	"context"
	"io"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Job struct {
	Sudo func() error
	Run  func(ctx context.Context, emit func(model.Event))
	Log  string
	Plan model.Plan
}

type Start func(Decision) (Job, error)

func Review(ctx context.Context, in io.Reader, out io.Writer, h Header, p model.Plan, mode Mode, start Start) (Decision, error) {
	m := newSession(ctx, lipgloss.NewRenderer(out), h, p, mode, start, time.Now)
	final, err := tea.NewProgram(m, tea.WithInput(in), tea.WithOutput(out), tea.WithoutSignalHandler()).Run()
	if err != nil {
		return Decision{}, err
	}
	s := final.(session)
	return s.review.decision, s.err
}

type (
	decided  struct{}
	prepared struct {
		err error
		job Job
	}
	sudoDone struct{ err error }
	eventMsg struct{ event model.Event }
	runEnded struct{}
	tickMsg  struct{}
	ctxDone  struct{}
)

type session struct {
	ctx    context.Context
	err    error
	start  Start
	now    func() time.Time
	cancel context.CancelFunc
	events chan model.Event
	apply  *applyScreen
	job    Job
	review review
	width  int
	height int
}

func newSession(ctx context.Context, r *lipgloss.Renderer, h Header, p model.Plan, mode Mode, start Start, now func() time.Time) session {
	return session{ctx: ctx, start: start, now: now, review: newReview(r, h, p, mode)}
}

func (m session) Init() tea.Cmd {
	return func() tea.Msg {
		<-m.ctx.Done()
		return ctxDone{}
	}
}

func (m session) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.apply != nil {
			m.apply.resize(msg.Width, msg.Height)
			return m, nil
		}
	case decided:
		if !m.review.decision.Apply || m.start == nil {
			return m, tea.Quit
		}
		return m, m.prepare(m.review.decision)
	case prepared:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.job = msg.job
		if m.job.Sudo != nil {
			return m, tea.Exec(sudoCommand{m.job.Sudo}, func(err error) tea.Msg { return sudoDone{err} })
		}
		return m.begin()
	case sudoDone:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		return m.begin()
	case eventMsg:
		m.apply.event(msg.event, m.now())
		return m, wait(m.events)
	case runEnded:
		m.cancel()
		return m, tea.Quit
	case tickMsg:
		if m.apply != nil && !m.apply.finished() {
			m.apply.clock = m.now()
			return m, tick()
		}
		return m, nil
	case ctxDone:
		if m.apply == nil {
			return m, tea.Quit
		}
		m.apply.stopping = true
		return m, nil
	case tea.KeyMsg:
		if m.apply != nil {
			return m.applyKey(msg)
		}
	}

	if m.apply != nil {
		return m, nil
	}
	next, cmd := m.review.Update(msg)
	m.review = next.(review)
	return m, cmd
}

func (m session) applyKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "o":
		m.apply.hideTail = !m.apply.hideTail
		m.apply.offset = m.apply.scroll()
	case "ctrl+c":
		if !m.apply.finished() {
			m.apply.stopping = true
			m.cancel()
		}
	}
	return m, nil
}

func (m session) prepare(d Decision) tea.Cmd {
	start := m.start
	return func() tea.Msg {
		job, err := start(d)
		return prepared{job: job, err: err}
	}
}

func (m session) begin() (tea.Model, tea.Cmd) {
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancel = cancel
	m.events = make(chan model.Event)
	m.apply = newApplyScreen(m.review.st, m.job.Plan, m.job.Log)
	m.apply.resize(m.width, m.height)
	m.apply.clock = m.now()

	run, events := m.job.Run, m.events
	go func() {
		run(ctx, func(e model.Event) { events <- e })
		close(events)
	}()
	return m, tea.Batch(wait(m.events), tick())
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

func (m session) View() string {
	if m.apply != nil {
		return m.apply.View()
	}
	return m.review.View()
}

type sudoCommand struct{ run func() error }

func (c sudoCommand) Run() error        { return c.run() }
func (sudoCommand) SetStdin(io.Reader)  {}
func (sudoCommand) SetStdout(io.Writer) {}
func (sudoCommand) SetStderr(io.Writer) {}
