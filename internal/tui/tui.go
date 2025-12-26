package tui

import (
	"booster/internal/coordinator"
	"booster/internal/executor"
	"booster/internal/logstream"
	"booster/internal/task"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	maxLogLines      = 8
	outputViewHeight = 15

	// Viewport dimension adjustments for panel sizing.
	panelBorderWidth  = 2 // left + right border
	logPanelOverhead  = 5 // border(2) + title(1) + help(2)
	taskPanelOverhead = 8 // border(2) + title(1) + progress(2) + blank(1) + help(2)
	taskPanelPadding  = 4 // border(2) + padding(2)
)

type FocusPanel int

const (
	FocusTaskList FocusPanel = iota
	FocusLogs
)

type Model struct {
	exec  *executor.Executor
	coord *coordinator.Coordinator

	showOutput bool
	showLogs   bool
	width      int
	height     int

	layout Layout

	taskList        *TaskListModel
	selectedTaskIdx int

	outputViewport viewport.Model
	logViewport    viewport.Model

	logCh        <-chan string
	logWriter    *logstream.ChannelWriter
	focusedPanel FocusPanel

	debugFile *os.File
}

func New(tasks []task.Task) Model {
	exec := executor.New(tasks)
	tl := NewTaskList(exec)
	tl.SetCompactMode(true)
	tl.SetSize(80, exec.Total())

	m := Model{
		exec:            exec,
		coord:           coordinator.New(),
		showLogs:        true,
		focusedPanel:    FocusTaskList,
		taskList:        tl,
		selectedTaskIdx: 0,
	}

	if debugPath := os.Getenv("BOOSTER_DEBUG"); debugPath != "" {
		if f, err := os.OpenFile(debugPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644); err == nil {
			m.debugFile = f
		}
	}

	return m
}

func (m Model) debugLog(format string, args ...any) {
	if m.debugFile != nil {
		fmt.Fprintf(m.debugFile, format+"\n", args...)
	}
}

func (m *Model) Close() {
	if m.debugFile != nil {
		m.debugFile.Close()
		m.debugFile = nil
	}
}

func (m Model) Init() tea.Cmd {
	if m.exec.Done() {
		return nil
	}

	return func() tea.Msg {
		return startTaskMsg{}
	}
}

type startTaskMsg struct{}

type taskDoneMsg struct {
	result task.Result
}

type logLineMsg struct {
	line string
}

type logDoneMsg struct{}

type spinnerTickMsg struct{}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.debugLog("msg: %T", msg)

	switch msg := msg.(type) {
	case tea.MouseMsg:
		if m.isTwoColumn() && m.showLogs {
			clickX := msg.X - 2

			switch msg.Button {
			case tea.MouseButtonLeft:
				if clickX < m.layout.LeftWidth {
					m.focusedPanel = FocusTaskList
				} else {
					m.focusedPanel = FocusLogs
				}
				return m, nil

			case tea.MouseButtonWheelUp:
				if clickX < m.layout.LeftWidth {
					m.taskList.ScrollUp(3)
				} else {
					m.logViewport.ScrollUp(3)
				}
				return m, nil

			case tea.MouseButtonWheelDown:
				if clickX < m.layout.LeftWidth {
					m.taskList.ScrollDown(3)
				} else {
					m.logViewport.ScrollDown(3)
				}
				return m, nil
			}
		}

	case tea.KeyMsg:

		if m.isTwoColumn() {
			switch msg.String() {
			case "o":

				m.showLogs = !m.showLogs
				return m, nil

			case "tab":

				if m.focusedPanel == FocusTaskList {
					m.focusedPanel = FocusLogs
				} else {
					m.focusedPanel = FocusTaskList
				}
				return m, nil

			case "j", "down":
				if m.focusedPanel == FocusTaskList {
					return m, m.taskList.Update(msg)
				}
				m.logViewport.ScrollDown(1)
				return m, nil

			case "k", "up":
				if m.focusedPanel == FocusTaskList {
					return m, m.taskList.Update(msg)
				}
				m.logViewport.ScrollUp(1)
				return m, nil

			case "G":
				if m.focusedPanel == FocusLogs {
					m.logViewport.GotoBottom()
				}
				return m, nil

			case "q", "ctrl+c":
				return m, tea.Quit

			case "enter":
				if m.exec.Stopped() {
					return m, tea.Quit
				}
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			if m.exec.Stopped() {
				return m, tea.Quit
			}
		case "o":
			if m.exec.Stopped() {
				m.showOutput = !m.showOutput
				if m.showOutput {
					m.outputViewport = m.createOutputViewport()
				}
				return m, nil
			}
		default:

			if m.showOutput && m.exec.Stopped() {
				var cmd tea.Cmd
				m.outputViewport, cmd = m.outputViewport.Update(msg)
				return m, cmd
			}
		}

	case startTaskMsg:

		logWriter, logCh, cmd := m.startTask()
		m.logWriter = logWriter
		m.logCh = logCh

		m.coord.StartTask(m.exec.Current())

		if m.isTwoColumnRunning() {
			m.logViewport = viewport.New(
				m.layout.RightWidth-panelBorderWidth,
				m.layout.Height-logPanelOverhead,
			)
			m.logViewport.SetContent("")

			taskViewportHeight := max(m.layout.Height-taskPanelOverhead, 3)
			m.taskList.SetSize(m.layout.LeftWidth-taskPanelPadding, taskViewportHeight)
		}

		return m, cmd

	case spinnerTickMsg:
		_ = m.taskList.Update(msg)

		if !m.exec.Stopped() {
			return m, m.taskList.SpinnerTick()
		}
		return m, nil

	case logLineMsg:

		m.coord.AddLogLine(msg.line)

		if m.isTwoColumnRunning() {
			wasAtBottom := m.logViewport.AtBottom()
			m.logViewport.SetContent(strings.Join(m.coord.CurrentLogs(), "\n"))
			if wasAtBottom {
				m.logViewport.GotoBottom()
			}
		}

		return m, listenForLogs(m.logCh)

	case logDoneMsg:

		if completeMsg := m.coord.LogsDone(); completeMsg != nil {
			return m.completeTask(completeMsg.Result)
		}
		return m, nil

	case taskDoneMsg:

		if completeMsg := m.coord.TaskDone(msg.result); completeMsg != nil {
			return m.completeTask(completeMsg.Result)
		}
		return m, nil

	case TaskSelectedMsg:
		m.selectedTaskIdx = msg.Index
		if m.exec.Stopped() {
			m.updateLogViewportForTask(msg.Index)
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		contentWidth, contentHeight := m.contentDimensions()
		m.layout = NewLayout(contentWidth, contentHeight)

		if m.showOutput {
			m.outputViewport.Width = contentWidth
			m.outputViewport.Height = min(outputViewHeight, contentHeight/2)
		}

		if m.layout.IsTwoColumn() {
			m.taskList.SetCompactMode(false)
			m.logViewport.Width = m.layout.RightWidth - panelBorderWidth
			m.logViewport.Height = m.layout.Height - logPanelOverhead

			taskViewportHeight := max(m.layout.Height-taskPanelOverhead, 3)
			m.taskList.SetSize(m.layout.LeftWidth-taskPanelPadding, taskViewportHeight)
		} else {
			m.taskList.SetCompactMode(true)
			availableWidth := max(contentWidth, 40)
			m.taskList.SetSize(availableWidth, m.exec.Total())
		}
		return m, nil
	}

	return m, nil
}

func (m Model) contentDimensions() (width, height int) {
	width = m.width - 4
	height = m.height - 2

	if width < 10 {
		width = 10
	}
	if height < 3 {
		height = 3
	}

	return width, height
}

func (m Model) View() string {
	var content string

	if m.layout.IsTwoColumn() {
		content = m.renderTwoColumn(m.layout)
	} else {
		content = m.renderSingleColumn()
	}

	if m.width > 0 && m.height > 0 {
		return appContainerStyle.Render(content)
	}

	return content
}

func (m Model) renderSingleColumn() string {
	var s strings.Builder

	tasks := m.exec.Tasks()
	stopped := m.exec.Stopped()
	total := m.exec.Total()

	completed := 0
	for i := range tasks {
		r := m.exec.ResultAt(i)
		if r.Status != task.StatusPending {
			completed++
		}
	}

	s.WriteString(titleStyle.Render("BOOSTER"))
	s.WriteString("\n")

	barWidth := m.width - 4
	if barWidth < 20 {
		barWidth = 40
	}
	s.WriteString(RenderProgress(completed, total, m.exec.ElapsedTime(), barWidth))
	s.WriteString("\n\n")

	s.WriteString(m.taskList.View())
	s.WriteString("\n")

	var failedTask *FailureInfo
	for i, t := range tasks {
		r := m.exec.ResultAt(i)
		if r.Status == task.StatusFailed {
			failedTask = &FailureInfo{
				TaskName: t.Name(),
				Error:    r.Error,
				Output:   r.Output,
				Duration: r.Duration,
			}
			break
		}
	}

	currentLogs := m.coord.CurrentLogs()
	if !stopped && len(currentLogs) > 0 {
		s.WriteString("\n")
		s.WriteString(logHeaderStyle.Render("─── logs ───"))
		s.WriteString("\n")

		displayLogs := currentLogs
		if len(displayLogs) > maxLogLines {
			displayLogs = displayLogs[len(displayLogs)-maxLogLines:]
		}
		for _, line := range displayLogs {
			displayLine := line
			maxWidth := m.width - 4
			if maxWidth > 0 && len(displayLine) > maxWidth {
				displayLine = displayLine[:maxWidth-3] + "..."
			}
			s.WriteString(logLineStyle.Render(displayLine))
			s.WriteString("\n")
		}
	}

	if stopped {
		summary := m.exec.Summary()

		if failedTask != nil {
			s.WriteString("\n")
			failWidth := m.width
			if failWidth < 40 {
				failWidth = 60
			}
			s.WriteString(RenderFailure(*failedTask, failWidth))
		}

		s.WriteString("\n")
		summaryData := m.buildSummaryData()
		summaryWidth := m.width
		if summaryWidth < 40 {
			summaryWidth = 60
		}

		if summary.HasFailures {
			s.WriteString(RenderFailedSummary(summaryData, summaryWidth))
		} else {
			s.WriteString(RenderSummary(summaryData, summaryWidth))
		}

		if m.showOutput {
			s.WriteString("\n")
			scrollHint := ""
			if m.outputViewport.TotalLineCount() > m.outputViewport.Height {
				scrollHint = fmt.Sprintf(" (↑↓/j/k to scroll, %d%%)",
					int(m.outputViewport.ScrollPercent()*100))
			}
			s.WriteString(outputHeaderStyle.Render("─── Output" + scrollHint + " ───"))
			s.WriteString("\n")
			s.WriteString(m.outputViewport.View())
			s.WriteString("\n")
		}

		s.WriteString("\n")
		hasOutput := m.hasTaskOutput()
		if hasOutput {
			if m.showOutput {
				s.WriteString(helpStyle.Render("'o' hide • ↑↓/j/k scroll • Enter exit"))
			} else {
				s.WriteString(helpStyle.Render("'o' view output • Enter exit"))
			}
		} else {
			s.WriteString(helpStyle.Render("Enter exit"))
		}
	}

	return s.String()
}

func (m Model) renderTwoColumn(layout Layout) string {
	leftBorderColor := UnfocusedBorderColor
	rightBorderColor := UnfocusedBorderColor
	if m.showLogs && m.focusedPanel == FocusTaskList {
		leftBorderColor = FocusedBorderColor
	} else if m.showLogs && m.focusedPanel == FocusLogs {
		rightBorderColor = FocusedBorderColor
	} else if !m.showLogs {
		leftBorderColor = FocusedBorderColor
	}

	var leftPanel Panel
	leftFocused := m.focusedPanel == FocusTaskList || !m.showLogs
	if m.showLogs {
		leftContent := m.renderTaskListContent(layout.LeftWidth - 4)
		leftPanel = Panel{
			Title:       "BOOSTER",
			Content:     leftContent,
			Width:       layout.LeftWidth,
			Height:      layout.Height - 3,
			BorderColor: leftBorderColor,
			Focused:     leftFocused,
		}
	} else {
		leftContent := m.renderTaskListContent(layout.LeftWidth + layout.RightWidth - 2)
		leftPanel = Panel{
			Title:       "BOOSTER",
			Content:     leftContent,
			Width:       layout.LeftWidth + layout.RightWidth,
			Height:      layout.Height - 3,
			BorderColor: leftBorderColor,
			Focused:     leftFocused,
		}
	}

	var panels string
	if m.showLogs {
		logs := m.getDisplayLogs()
		rightContent := m.logViewport.View()
		if len(logs) == 0 {
			rightContent = m.renderEmptyLogContent()
		}

		var taskName string
		if m.exec.Stopped() {
			if m.selectedTaskIdx < len(m.exec.Tasks()) {
				taskName = m.exec.Tasks()[m.selectedTaskIdx].Name()
			}
		} else {
			if m.exec.Current() < len(m.exec.Tasks()) {
				taskName = m.exec.Tasks()[m.exec.Current()].Name()
			}
		}

		logTitle := taskName
		lineCount := m.logViewport.TotalLineCount()
		if lineCount > 0 {
			logTitle = fmt.Sprintf("%s • %d lines", taskName, lineCount)
		}
		if m.logViewport.TotalLineCount() > m.logViewport.Height {
			scrollPct := int(m.logViewport.ScrollPercent() * 100)
			logTitle = fmt.Sprintf("%s (%d%%)", logTitle, scrollPct)
		}

		if !m.logViewport.AtBottom() && m.logViewport.TotalLineCount() > 0 {
			logTitle += " ▼"
		}

		rightPanel := Panel{
			Title:       "Logs: " + logTitle,
			Content:     rightContent,
			Width:       layout.RightWidth,
			Height:      layout.Height - 3,
			BorderColor: rightBorderColor,
			Focused:     m.focusedPanel == FocusLogs,
		}

		leftRendered := RenderPanel(leftPanel)
		rightRendered := RenderPanel(rightPanel)
		panels = lipgloss.JoinHorizontal(lipgloss.Top, leftRendered, rightRendered)
	} else {
		panels = RenderPanel(leftPanel)
	}

	var helpText string
	if m.exec.Stopped() {
		if m.showLogs {
			helpText = helpStyle.Render("enter exit • o hide logs • tab switch • ↑↓/j/k navigate/scroll")
		} else {
			helpText = helpStyle.Render("enter exit • o show logs • ↑↓/j/k navigate")
		}
	} else {
		if m.showLogs {
			helpText = helpStyle.Render("q quit • o hide logs • tab switch panel • ↑↓/j/k navigate/scroll • G bottom")
		} else {
			helpText = helpStyle.Render("q quit • o show logs • ↑↓/j/k navigate")
		}
	}

	return panels + "\n" + helpText
}

func (m Model) renderTaskListContent(width int) string {
	var s strings.Builder

	tasks := m.exec.Tasks()
	total := m.exec.Total()

	completed := 0
	for i := range tasks {
		r := m.exec.ResultAt(i)
		if r.Status != task.StatusPending {
			completed++
		}
	}

	barWidth := max(width-4, 20)
	s.WriteString(RenderProgress(completed, total, m.exec.ElapsedTime(), barWidth))
	s.WriteString("\n\n")

	s.WriteString(m.taskList.View())

	return s.String()
}

func renderTaskWithLeader(prefix, name, suffix string, totalWidth int) string {
	prefixWidth := lipgloss.Width(prefix)
	nameWidth := lipgloss.Width(name)
	suffixWidth := lipgloss.Width(suffix)

	leaderSpace := max(totalWidth-prefixWidth-nameWidth-suffixWidth-2, 3)

	leaders := leaderStyle.Render(strings.Repeat("·", leaderSpace))

	return prefix + name + " " + leaders + " " + suffix
}

func (m Model) isTwoColumnRunning() bool {
	return m.layout.IsTwoColumn() && !m.exec.Stopped()
}

func (m Model) isTwoColumn() bool {
	return m.layout.IsTwoColumn()
}

func (m Model) getDisplayLogs() []string {
	if m.exec.Stopped() {
		return m.coord.LogsFor(m.selectedTaskIdx)
	}

	return m.coord.CurrentLogs()
}

func (m *Model) updateLogViewportForTask(idx int) {
	logs := m.coord.LogsFor(idx)
	if len(logs) > 0 {
		m.logViewport.SetContent(strings.Join(logs, "\n"))
	} else {
		m.logViewport.SetContent("")
	}
}

func (m *Model) initLogViewportForHistory() {
	if !m.isTwoColumn() {
		return
	}

	m.logViewport = viewport.New(
		m.layout.RightWidth-panelBorderWidth,
		m.layout.Height-logPanelOverhead,
	)

	taskViewportHeight := max(m.layout.Height-taskPanelOverhead, 3)
	m.taskList.SetSize(m.layout.LeftWidth-taskPanelPadding, taskViewportHeight)

	m.updateLogViewportForTask(m.selectedTaskIdx)
}

func (m Model) buildSummaryData() SummaryData {
	summary := m.exec.Summary()
	tasks := m.exec.Tasks()

	var timings []TaskTiming
	for i, t := range tasks {
		r := m.exec.ResultAt(i)
		if r.Duration > 0 && r.Status == task.StatusDone {
			timings = append(timings, TaskTiming{
				Name:     t.Name(),
				Duration: r.Duration,
			})
		}
	}

	sort.Slice(timings, func(i, j int) bool {
		return timings[i].Duration > timings[j].Duration
	})

	if len(timings) > 3 {
		timings = timings[:3]
	}

	return SummaryData{
		Done:         summary.Done,
		Skipped:      summary.Skipped,
		Failed:       summary.Failed,
		Total:        m.exec.Total(),
		Elapsed:      m.exec.ElapsedTime(),
		SlowestTasks: timings,
	}
}

func (m Model) startTask() (*logstream.ChannelWriter, <-chan string, tea.Cmd) {
	logWriter, logCh := logstream.NewChannelWriter(100)

	cmd := tea.Batch(
		runTask(m.exec, logWriter),
		listenForLogs(logCh),
		m.taskList.SpinnerTick(),
	)

	return logWriter, logCh, cmd
}

func (m Model) completeTask(result task.Result) (Model, tea.Cmd) {
	if result.Status == task.StatusFailed {
		m.exec.Abort()

		m.initLogViewportForHistory()
		return m, nil
	}
	if m.exec.Stopped() {
		m.initLogViewportForHistory()
		return m, nil
	}

	advanceCmd := m.taskList.Update(AdvanceSelectionMsg{})
	m.selectedTaskIdx = m.taskList.Selected()

	return m, tea.Batch(
		advanceCmd,
		func() tea.Msg {
			return startTaskMsg{}
		},
	)
}

func runTask(exec *executor.Executor, logWriter *logstream.ChannelWriter) tea.Cmd {
	return func() tea.Msg {
		ctx := logstream.WithWriter(context.Background(), logWriter)
		result, _ := exec.RunNext(ctx)
		logWriter.Close()
		return taskDoneMsg{result: result}
	}
}

func listenForLogs(ch <-chan string) tea.Cmd {
	if ch == nil {
		return nil
	}

	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return logDoneMsg{}
		}
		return logLineMsg{line: line}
	}
}

func (m Model) hasTaskOutput() bool {
	tasks := m.exec.Tasks()
	for i := range tasks {
		if m.exec.ResultAt(i).Output != "" {
			return true
		}
	}
	return false
}

func (m Model) createOutputViewport() viewport.Model {
	var content strings.Builder
	tasks := m.exec.Tasks()
	for i, t := range tasks {
		r := m.exec.ResultAt(i)
		if r.Output != "" {
			content.WriteString("\n")
			content.WriteString(outputTaskStyle.Render(t.Name()))
			content.WriteString("\n")
			content.WriteString(outputContentStyle.Render(strings.TrimSpace(r.Output)))
			content.WriteString("\n")
		}
	}

	width := m.width
	if width == 0 {
		width = 80
	}
	height := min(outputViewHeight, m.height/2)
	if height == 0 {
		height = outputViewHeight
	}

	vp := viewport.New(width, height)
	vp.SetContent(content.String())
	return vp
}

func (m Model) renderEmptyLogContent() string {
	var s strings.Builder

	var taskIdx int
	if m.exec.Stopped() {
		taskIdx = m.selectedTaskIdx
	} else {
		taskIdx = m.exec.Current()
	}

	if taskIdx >= len(m.exec.Tasks()) {
		return "Waiting for output..."
	}

	t := m.exec.Tasks()[taskIdx]
	result := m.exec.ResultAt(taskIdx)

	s.WriteString(lipgloss.NewStyle().Bold(true).Render(t.Name()))
	s.WriteString("\n\n")

	switch result.Status {
	case task.StatusPending:
		s.WriteString(lipgloss.NewStyle().Faint(true).Render("Waiting for output..."))
	case task.StatusSkipped:
		s.WriteString(lipgloss.NewStyle().Faint(true).Render("Task was skipped"))
		if result.Message != "" {
			s.WriteString("\n")
			s.WriteString(lipgloss.NewStyle().Faint(true).Render(result.Message))
		}
	default:
		s.WriteString(lipgloss.NewStyle().Faint(true).Render("No output captured"))
	}

	return s.String()
}
