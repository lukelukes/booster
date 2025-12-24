package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// SpinnerModel handles animated spinner display for running tasks.
// Extracted as a reusable component following the model tree pattern.
type SpinnerModel struct {
	frame  int
	frames []string
}

// Default Braille dots spinner frames for professional look.
var defaultSpinnerFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

// NewSpinner creates a new spinner with default Braille dot frames.
func NewSpinner() SpinnerModel {
	return SpinnerModel{
		frames: defaultSpinnerFrames,
	}
}

// Update handles spinner tick messages and advances the animation frame.
func (s SpinnerModel) Update(msg tea.Msg) SpinnerModel {
	if _, ok := msg.(spinnerTickMsg); ok {
		s.frame = (s.frame + 1) % len(s.frames)
	}
	return s
}

// View returns the current spinner character.
func (s SpinnerModel) View() string {
	if len(s.frames) == 0 {
		return ""
	}
	return s.frames[s.frame%len(s.frames)]
}

// Tick returns a command that triggers the next spinner frame after 80ms.
func (s SpinnerModel) Tick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}
