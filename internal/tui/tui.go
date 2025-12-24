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

	// Cached layout (updated on resize, avoids recalculation)
	layout Layout

	outputViewport viewport.Model
	logViewport    viewport.Model
	taskViewport   viewport.Model

	logCh        <-chan string
	logWriter    *logstream.ChannelWriter
	selectedTask int
	focusedPanel FocusPanel

	// Child model for spinner animation
	spinner SpinnerModel

	// Debug logging (enabled via BOOSTER_DEBUG env var)
	debugFile *os.File
}

func New(tasks []task.Task) Model {
	m := Model{
		exec:         executor.New(tasks),
		coord:        coordinator.New(),
		showLogs:     true,
		selectedTask: 0,
		focusedPanel: FocusTaskList,
		spinner:      NewSpinner(),
	}

	// Enable debug logging if BOOSTER_DEBUG is set to a file path
	if debugPath := os.Getenv("BOOSTER_DEBUG"); debugPath != "" {
		if f, err := os.OpenFile(debugPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644); err == nil {
			m.debugFile = f
		}
	}

	return m
}

// debugLog writes a formatted message to the debug log file if debugging is enabled.
// Enable by setting BOOSTER_DEBUG=/path/to/debug.log
func (m Model) debugLog(format string, args ...any) {
	if m.debugFile != nil {
		fmt.Fprintf(m.debugFile, format+"\n", args...)
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
			// Use cached layout for panel boundary detection
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
					m.taskViewport.ScrollUp(3)
				} else {
					m.logViewport.ScrollUp(3)
				}
				return m, nil

			case tea.MouseButtonWheelDown:
				if clickX < m.layout.LeftWidth {
					m.taskViewport.ScrollDown(3)
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
					if m.selectedTask < m.exec.Total()-1 {
						m.selectedTask++
						m.ensureTaskVisible()
					}

					if m.exec.Stopped() {
						m.updateLogViewportForSelectedTask()
					}
				} else {
					m.logViewport.ScrollDown(1)
				}
				return m, nil

			case "k", "up":
				if m.focusedPanel == FocusTaskList {
					if m.selectedTask > 0 {
						m.selectedTask--
						m.ensureTaskVisible()
					}

					if m.exec.Stopped() {
						m.updateLogViewportForSelectedTask()
					}
				} else {
					m.logViewport.ScrollUp(1)
				}
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
			// Use cached layout for viewport initialization
			m.logViewport = viewport.New(
				m.layout.RightWidth-2,
				m.layout.Height-5,
			)
			m.logViewport.SetContent("")

			taskViewportHeight := max(m.layout.Height-8, 3)
			m.taskViewport = viewport.New(
				m.layout.LeftWidth-4,
				taskViewportHeight,
			)
		}

		return m, cmd

	case spinnerTickMsg:
		m.spinner = m.spinner.Update(msg)

		if !m.exec.Stopped() {
			return m, m.spinner.Tick()
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
		// Delegate to coordinator - returns completion msg if logs already done
		if completeMsg := m.coord.TaskDone(msg.result); completeMsg != nil {
			return m.completeTask(completeMsg.Result)
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Calculate content dimensions and cache layout
		contentWidth, contentHeight := m.contentDimensions()
		m.layout = NewLayout(contentWidth, contentHeight)

		// Update viewport size if output is shown
		if m.showOutput {
			m.outputViewport.Width = contentWidth
			m.outputViewport.Height = min(outputViewHeight, contentHeight/2)
		}
		// Update viewport sizes for two-column mode
		if m.layout.IsTwoColumn() {
			m.logViewport.Width = m.layout.RightWidth - 2 // Panel border takes 2 chars (left + right)
			m.logViewport.Height = m.layout.Height - 5    // Panel border (2) + title (1) + help bar (2)

			// Task viewport
			taskViewportHeight := max(
				// border(2) + title(1) + progress(2) + blank(1) + help(2)
				m.layout.Height-8, 3)
			m.taskViewport.Width = m.layout.LeftWidth - 4
			m.taskViewport.Height = taskViewportHeight
		}
		return m, nil
	}

	return m, nil
}

// spinner returns the current spinner character for animation.
func (m Model) spinnerView() string {
	return m.spinner.View()
}

// contentDimensions calculates available space for content within the app container.
// Container border takes 2 chars (left + right) and 2 lines (top + bottom).
// Container padding takes 2 chars (left + right) horizontally.
// Total overhead: 4 chars width, 2 lines height.
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

// View renders the TUI.
func (m Model) View() string {
	var content string
	// Use cached layout for rendering decisions
	if m.layout.IsTwoColumn() {
		content = m.renderTwoColumn(m.layout)
	} else {
		// Single-column rendering for narrow terminals
		content = m.renderSingleColumn()
	}

	// Wrap content in app container
	// Don't set Width/Height on the container - let content flow naturally
	// The content is already sized correctly for the terminal
	if m.width > 0 && m.height > 0 {
		return appContainerStyle.Render(content)
	}

	return content
}

// renderSingleColumn renders the traditional single-column layout.
func (m Model) renderSingleColumn() string {
	var s strings.Builder

	tasks := m.exec.Tasks()
	current := m.exec.Current()
	stopped := m.exec.Stopped()
	total := m.exec.Total()

	// Count completed tasks for progress
	completed := 0
	for i := range tasks {
		r := m.exec.ResultAt(i)
		if r.Status != task.StatusPending {
			completed++
		}
	}

	// Header with progress bar (always show)
	s.WriteString(titleStyle.Render("BOOSTER"))
	s.WriteString("\n")

	// Progress bar while running or at completion
	barWidth := m.width - 4
	if barWidth < 20 {
		barWidth = 40
	}
	s.WriteString(RenderProgress(completed, total, m.exec.ElapsedTime(), barWidth))
	s.WriteString("\n\n")

	// Task list
	var failedTask *FailureInfo
	// Calculate available width for task lines
	availableWidth := max(
		// Account for container margins
		m.width-4, 40)

	for i, t := range tasks {
		var line string
		r := m.exec.ResultAt(i)

		if r.Status != task.StatusPending {
			// Completed task - show status with dotted leaders
			switch r.Status {
			case task.StatusDone:
				suffix := formatElapsedCompact(r.Duration)
				taskLine := renderTaskWithLeader("✓ ", t.Name(), suffix, availableWidth)
				line = doneStyle.Render(taskLine)
			case task.StatusSkipped:
				label := "exists"
				if strings.HasPrefix(r.Message, "condition not met:") {
					label = "skipped"
				}
				taskLine := renderTaskWithLeader("○ ", t.Name(), label, availableWidth)
				line = skippedStyle.Render(taskLine)
			case task.StatusFailed:
				// Store failure info for detailed display later
				failedTask = &FailureInfo{
					TaskName: t.Name(),
					Error:    r.Error,
					Output:   r.Output,
					Duration: r.Duration,
				}
				// Show simple line in task list
				line = failedStyle.Render("✗ " + t.Name())
			}
		} else if i == current && !stopped {
			// Currently running with animated spinner
			line = runningStyle.Render("→ " + t.Name() + " " + m.spinnerView())
		} else {
			// Pending
			line = pendingStyle.Render("  " + t.Name())
		}

		s.WriteString(line + "\n")
	}

	// Show streaming logs while task is running
	currentLogs := m.coord.CurrentLogs()
	if !stopped && len(currentLogs) > 0 {
		s.WriteString("\n")
		s.WriteString(logHeaderStyle.Render("─── logs ───"))
		s.WriteString("\n")
		// Display only the last maxLogLines to keep view manageable
		displayLogs := currentLogs
		if len(displayLogs) > maxLogLines {
			displayLogs = displayLogs[len(displayLogs)-maxLogLines:]
		}
		for _, line := range displayLogs {
			// Truncate long lines to terminal width
			displayLine := line
			maxWidth := m.width - 4 // Leave some margin
			if maxWidth > 0 && len(displayLine) > maxWidth {
				displayLine = displayLine[:maxWidth-3] + "..."
			}
			s.WriteString(logLineStyle.Render(displayLine))
			s.WriteString("\n")
		}
	}

	if stopped {
		summary := m.exec.Summary()

		// Show failure box if there was a failure
		if failedTask != nil {
			s.WriteString("\n")
			failWidth := m.width
			if failWidth < 40 {
				failWidth = 60
			}
			s.WriteString(RenderFailure(*failedTask, failWidth))
		}

		// Show summary screen
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

		// Show output section if toggled
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

		// Build help text
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

// renderTwoColumn renders the two-column layout for wide terminals (while running).
func (m Model) renderTwoColumn(layout Layout) string {
	// Determine border colors based on focus
	leftBorderColor := UnfocusedBorderColor
	rightBorderColor := UnfocusedBorderColor
	if m.showLogs && m.focusedPanel == FocusTaskList {
		leftBorderColor = FocusedBorderColor
	} else if m.showLogs && m.focusedPanel == FocusLogs {
		rightBorderColor = FocusedBorderColor
	} else if !m.showLogs {
		leftBorderColor = FocusedBorderColor
	}

	// Render left panel (task list)
	var leftPanel Panel
	leftFocused := m.focusedPanel == FocusTaskList || !m.showLogs
	if m.showLogs {
		leftContent := m.renderTaskListContent(layout.LeftWidth - 4)
		leftPanel = Panel{
			Title:       "BOOSTER",
			Content:     leftContent,
			Width:       layout.LeftWidth,
			Height:      layout.Height - 3, // Reserve 3 lines for help bar
			BorderColor: leftBorderColor,
			Focused:     leftFocused,
		}
	} else {
		// Expand task list to full width when logs hidden
		leftContent := m.renderTaskListContent(layout.LeftWidth + layout.RightWidth - 2) // -2 for panel borders
		leftPanel = Panel{
			Title:       "BOOSTER",
			Content:     leftContent,
			Width:       layout.LeftWidth + layout.RightWidth,
			Height:      layout.Height - 3,
			BorderColor: leftBorderColor,
			Focused:     leftFocused,
		}
	}

	// Build the view based on showLogs
	var panels string
	if m.showLogs {
		// Render right panel (logs)
		logs := m.getDisplayLogs()
		rightContent := m.logViewport.View()
		if len(logs) == 0 {
			rightContent = m.renderEmptyLogContent()
		}

		// Determine task name for title
		// When stopped: show selected task name
		// When running: show current task name
		var taskName string
		if m.exec.Stopped() {
			if m.selectedTask < len(m.exec.Tasks()) {
				taskName = m.exec.Tasks()[m.selectedTask].Name()
			}
		} else {
			if m.exec.Current() < len(m.exec.Tasks()) {
				taskName = m.exec.Tasks()[m.exec.Current()].Name()
			}
		}

		// Build title with scroll indicator
		logTitle := taskName
		lineCount := m.logViewport.TotalLineCount()
		if lineCount > 0 {
			logTitle = fmt.Sprintf("%s • %d lines", taskName, lineCount)
		}
		if m.logViewport.TotalLineCount() > m.logViewport.Height {
			scrollPct := int(m.logViewport.ScrollPercent() * 100)
			logTitle = fmt.Sprintf("%s (%d%%)", logTitle, scrollPct)
		}
		// Add arrow indicator if not at bottom
		if !m.logViewport.AtBottom() && m.logViewport.TotalLineCount() > 0 {
			logTitle += " ▼"
		}

		rightPanel := Panel{
			Title:       "Logs: " + logTitle,
			Content:     rightContent,
			Width:       layout.RightWidth,
			Height:      layout.Height - 3, // Reserve 3 lines for help bar
			BorderColor: rightBorderColor,
			Focused:     m.focusedPanel == FocusLogs,
		}

		// Join panels horizontally
		leftRendered := RenderPanel(leftPanel)
		rightRendered := RenderPanel(rightPanel)
		panels = lipgloss.JoinHorizontal(lipgloss.Top, leftRendered, rightRendered)
	} else {
		// Only show task list (full width)
		panels = RenderPanel(leftPanel)
	}

	// Add help bar at bottom
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

// renderTaskListContent renders just the task list content (header, progress, tasks).
func (m Model) renderTaskListContent(width int) string {
	var s strings.Builder

	tasks := m.exec.Tasks()
	total := m.exec.Total()

	// Count completed tasks for progress
	completed := 0
	for i := range tasks {
		r := m.exec.ResultAt(i)
		if r.Status != task.StatusPending {
			completed++
		}
	}

	// Progress bar
	barWidth := max(width-4, 20)
	s.WriteString(RenderProgress(completed, total, m.exec.ElapsedTime(), barWidth))
	s.WriteString("\n\n")

	// Render task list into viewport and display
	taskLines := m.renderTaskLines(width)
	m.taskViewport.SetContent(taskLines)
	s.WriteString(m.taskViewport.View())

	return s.String()
}

// renderTaskWithLeader renders a task line with dotted leaders connecting name to status.
// Example: "✓ Install Homebrew ············· 12.3s"
// The prefix includes the icon and spacing, name is the task name, suffix is the status/duration.
// totalWidth is the available width for the entire line.
func renderTaskWithLeader(prefix, name, suffix string, totalWidth int) string {
	// Calculate space for leaders
	// Account for prefix (icon + space) and suffix (status/duration)
	prefixWidth := lipgloss.Width(prefix)
	nameWidth := lipgloss.Width(name)
	suffixWidth := lipgloss.Width(suffix)

	// Calculate leader count (minimum 3 dots for readability)
	// 2 for spacing around dots (one space before dots, one space before suffix)
	leaderSpace := max(totalWidth-prefixWidth-nameWidth-suffixWidth-2, 3)

	leaders := leaderStyle.Render(strings.Repeat("·", leaderSpace))

	return prefix + name + " " + leaders + " " + suffix
}

// renderTaskLines renders just the task list lines (without progress bar) for viewport content.
func (m Model) renderTaskLines(width int) string {
	var s strings.Builder

	tasks := m.exec.Tasks()
	current := m.exec.Current()
	stopped := m.exec.Stopped()

	for i, t := range tasks {
		var line string
		r := m.exec.ResultAt(i)
		isSelected := i == m.selectedTask

		// Selection indicator prefix
		prefix := "○ "
		if isSelected {
			prefix = "▶ "
		}

		if r.Status != task.StatusPending {
			// Completed task - show status with dotted leaders
			switch r.Status {
			case task.StatusDone:
				suffix := formatElapsedCompact(r.Duration)
				taskLine := renderTaskWithLeader(prefix+"✓ ", t.Name(), suffix, width)
				line = doneStyle.Render(taskLine)
			case task.StatusSkipped:
				label := "exists"
				if strings.HasPrefix(r.Message, "condition not met:") {
					label = "skipped"
				}
				taskLine := renderTaskWithLeader(prefix+"○ ", t.Name(), label, width)
				line = skippedStyle.Render(taskLine)
			case task.StatusFailed:
				line = failedStyle.Render(prefix + "✗ " + t.Name())
			}
		} else if i == current && !stopped {
			// Currently running with animated spinner
			line = runningStyle.Render(prefix + "→ " + t.Name() + " " + m.spinnerView())
		} else {
			// Pending
			line = pendingStyle.Render(prefix + "  " + t.Name())
		}

		// Apply selection highlight (full row background)
		if isSelected {
			// Pad line to consistent width for full-row highlight effect
			lineWidth := lipgloss.Width(line)
			if lineWidth < width {
				line += strings.Repeat(" ", width-lineWidth-4)
			}
			line = selectedRowStyle.Render(line)
		}

		s.WriteString(line)
		// Don't add newline after last task to avoid extra blank line
		if i < len(tasks)-1 {
			s.WriteString("\n")
		}
	}

	return s.String()
}

// isTwoColumnRunning returns true if two-column mode is active (running only).
func (m Model) isTwoColumnRunning() bool {
	return m.layout.IsTwoColumn() && !m.exec.Stopped()
}

// isTwoColumn returns true if two-column mode is active (both running and stopped).
func (m Model) isTwoColumn() bool {
	return m.layout.IsTwoColumn()
}

// getDisplayLogs returns the logs to display based on execution state.
// When running: shows current task logs
// When stopped: shows historical logs for selected task
func (m Model) getDisplayLogs() []string {
	if m.exec.Stopped() {
		// Show historical logs for selected task (from coordinator)
		return m.coord.LogsFor(m.selectedTask)
	}
	// Show current logs during execution (from coordinator)
	return m.coord.CurrentLogs()
}

// updateLogViewportForSelectedTask updates the log viewport content
// to show the historical logs for the currently selected task.
func (m *Model) updateLogViewportForSelectedTask() {
	logs := m.getDisplayLogs()
	if len(logs) > 0 {
		m.logViewport.SetContent(strings.Join(logs, "\n"))
	} else {
		m.logViewport.SetContent("")
	}
}

// ensureTaskVisible scrolls the task viewport to keep the selected task visible.
// Uses a "scroll-into-view" pattern: only scrolls when selection moves outside visible area.
func (m *Model) ensureTaskVisible() {
	if m.taskViewport.Height == 0 {
		return
	}

	// Each task takes 1 line in the list
	visibleStart := m.taskViewport.YOffset
	visibleEnd := visibleStart + m.taskViewport.Height

	// If selected task is above visible area, scroll up
	if m.selectedTask < visibleStart {
		m.taskViewport.SetYOffset(m.selectedTask)
	}
	// If selected task is below visible area, scroll down
	if m.selectedTask >= visibleEnd {
		m.taskViewport.SetYOffset(m.selectedTask - m.taskViewport.Height + 1)
	}
}

// initLogViewportForHistory initializes the log viewport for browsing
// historical logs when execution has stopped.
func (m *Model) initLogViewportForHistory() {
	if !m.isTwoColumn() {
		return
	}

	// Initialize log viewport using cached layout
	m.logViewport = viewport.New(
		m.layout.RightWidth-2, // Panel border takes 2 chars (left + right)
		m.layout.Height-5,     // Panel border (2) + title (1) + help bar (2)
	)

	// Initialize task viewport
	taskViewportHeight := max(
		// border(2) + title(1) + progress(2) + blank(1) + help(2)
		m.layout.Height-8, 3)
	m.taskViewport = viewport.New(
		m.layout.LeftWidth-4, // Panel border(2) + padding(2)
		taskViewportHeight,
	)

	// Keep selectedTask at current position (last completed or failed task)
	// This preserves focus on the task that just finished
	m.updateLogViewportForSelectedTask()
	m.ensureTaskVisible()
}

// buildSummaryData constructs SummaryData from executor state.
func (m Model) buildSummaryData() SummaryData {
	summary := m.exec.Summary()
	tasks := m.exec.Tasks()

	// Collect task timings for slowest tasks
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

	// Sort by duration descending
	sort.Slice(timings, func(i, j int) bool {
		return timings[i].Duration > timings[j].Duration
	})

	// Take top 3
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

// startTask sets up log streaming and starts the next task.
// Returns the logWriter, logCh, and command - caller must merge into model.
// This pattern avoids BubbleTea's value semantics issue where pointer receiver
// mutations don't propagate through Update's return value.
func (m Model) startTask() (*logstream.ChannelWriter, <-chan string, tea.Cmd) {
	// Create log writer for this task
	logWriter, logCh := logstream.NewChannelWriter(100)

	// Return batch of commands: run task + listen for logs + spinner animation
	cmd := tea.Batch(
		runTask(m.exec, logWriter),
		listenForLogs(logCh),
		m.spinner.Tick(),
	)

	return logWriter, logCh, cmd
}

// completeTask handles task completion after both task execution and log streaming are done.
// This ensures logs are correctly attributed to tasks regardless of message arrival order.
func (m Model) completeTask(result task.Result) (Model, tea.Cmd) {
	// Log persistence and coordination state handled by coordinator
	// (logs already saved when TaskDone/LogsDone returned completion message)

	// Stop execution if task failed
	if result.Status == task.StatusFailed {
		m.exec.Abort()
		// Initialize log viewport for browsing history when stopped
		m.initLogViewportForHistory()
		return m, nil
	}
	if m.exec.Stopped() {
		// Initialize log viewport for browsing history when stopped
		m.initLogViewportForHistory()
		return m, nil
	}

	// Auto-advance selected task (if not at last task)
	if m.selectedTask < m.exec.Total()-1 {
		m.selectedTask++
		m.ensureTaskVisible()
	}

	// Trigger next task via message for proper state handling
	return m, func() tea.Msg {
		return startTaskMsg{}
	}
}

// runTask executes the current task with log streaming context.
// Takes exec and logWriter as parameters to avoid data races from capturing
// receiver fields in the goroutine.
func runTask(exec *executor.Executor, logWriter *logstream.ChannelWriter) tea.Cmd {
	return func() tea.Msg {
		ctx := logstream.WithWriter(context.Background(), logWriter)
		result, _ := exec.RunNext(ctx)
		logWriter.Close() // Signal end of logs
		return taskDoneMsg{result: result}
	}
}

// listenForLogs waits for the next log line from the channel.
// Takes channel as parameter to avoid data races from capturing receiver fields.
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

// hasTaskOutput returns true if any task has output to display.
func (m Model) hasTaskOutput() bool {
	tasks := m.exec.Tasks()
	for i := range tasks {
		if m.exec.ResultAt(i).Output != "" {
			return true
		}
	}
	return false
}

// createOutputViewport creates a viewport populated with task output.
func (m Model) createOutputViewport() viewport.Model {
	// Build output content
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

	// Calculate viewport dimensions
	width := m.width
	if width == 0 {
		width = 80 // Default width
	}
	height := min(outputViewHeight, m.height/2)
	if height == 0 {
		height = outputViewHeight
	}

	vp := viewport.New(width, height)
	vp.SetContent(content.String())
	return vp
}

// renderEmptyLogContent renders a richer display when the log panel is empty.
// Shows task context and status-specific messages instead of just "(no output)".
func (m Model) renderEmptyLogContent() string {
	var s strings.Builder

	// Get current or selected task
	var taskIdx int
	if m.exec.Stopped() {
		taskIdx = m.selectedTask
	} else {
		taskIdx = m.exec.Current()
	}

	if taskIdx >= len(m.exec.Tasks()) {
		return "Waiting for output..."
	}

	t := m.exec.Tasks()[taskIdx]
	result := m.exec.ResultAt(taskIdx)

	// Show task name
	s.WriteString(lipgloss.NewStyle().Bold(true).Render(t.Name()))
	s.WriteString("\n\n")

	// Show status-specific message
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
