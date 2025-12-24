package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type SpinnerModel struct {
	frame  int
	frames []string
}

var defaultSpinnerFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

func NewSpinner() SpinnerModel {
	return SpinnerModel{
		frames: defaultSpinnerFrames,
	}
}

func (s SpinnerModel) Update(msg tea.Msg) SpinnerModel {
	if _, ok := msg.(spinnerTickMsg); ok && len(s.frames) > 0 {
		s.frame = (s.frame + 1) % len(s.frames)
	}
	return s
}

func (s SpinnerModel) View() string {
	if len(s.frames) == 0 {
		return ""
	}
	return s.frames[s.frame%len(s.frames)]
}

func (s SpinnerModel) Tick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}
