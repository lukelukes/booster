package ui

import (
	"booster/internal/model"
	"io"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/assert"
)

var (
	down  = tea.KeyMsg{Type: tea.KeyDown}
	up    = tea.KeyMsg{Type: tea.KeyUp}
	right = tea.KeyMsg{Type: tea.KeyRight}
	left  = tea.KeyMsg{Type: tea.KeyLeft}
	space = tea.KeyMsg{Type: tea.KeySpace}
	enter = tea.KeyMsg{Type: tea.KeyEnter}
	quit  = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}
)

var header = Header{OS: "darwin", Arch: "arm64", Profile: "personal", Config: "~/.dotfiles/bootstrap.yaml"}

func items(names ...string) []model.Item {
	out := make([]model.Item, 0, len(names))
	for _, n := range names {
		out = append(out, model.Item{Name: n})
	}
	return out
}

func bootstrapPlan() model.Plan {
	return model.Plan{
		Excluded: 1,
		Steps: []model.PlannedStep{
			{Name: "homebrew", Change: model.Change{Kind: model.ChangeCreate, Verb: "install", Total: 1, Items: items("homebrew"), NeedsSudo: true}},
			{Name: "packages", Change: model.Change{Kind: model.ChangeCreate, Verb: "install", Total: 5, Items: items("ripgrep", "fd", "lazygit")}},
			{Name: "macos-defaults", Change: model.Change{Kind: model.ChangeUpdate, Verb: "update", Total: 3, Items: []model.Item{
				{Name: "NSGlobalDomain KeyRepeat", From: "6", To: "2"},
			}}},
			{Name: "links", Change: model.Change{Kind: model.ChangeUpdate, Verb: "link", Total: 3, Items: []model.Item{
				{Name: "~/.zshrc", To: "dotfiles/config/zshrc"},
				{Name: "~/.gitconfig", To: "dotfiles/config/gitconfig"},
				{Name: "~/.config/starship.toml", From: "~/old/starship.toml", To: "dotfiles/config/starship.toml"},
			}}},
			{Name: "git-config", Change: model.Change{Verb: "set", Total: 2}},
			{Name: "login-shell", Change: model.Change{Kind: model.ChangeUnknown, Verb: "run"}},
		},
	}
}

func driftPlan() model.Plan {
	return model.Plan{Steps: []model.PlannedStep{
		{Name: "homebrew", Change: model.Change{Verb: "install", Total: 1}},
		{Name: "packages", Change: model.Change{Kind: model.ChangeCreate, Verb: "install", Total: 33, Items: items("lazygit")}},
		{Name: "macos-defaults", Change: model.Change{Kind: model.ChangeUpdate, Verb: "update", Total: 29, Items: []model.Item{
			{Name: "NSGlobalDomain KeyRepeat", From: "6", To: "2"},
		}}},
		{Name: "links", Change: model.Change{Verb: "link", Total: 6}},
		{Name: "git-config", Change: model.Change{Verb: "set", Total: 2}},
	}}
}

func newTestReview(p model.Plan, mode Mode) review {
	return newReview(lipgloss.NewRenderer(io.Discard), header, p, mode)
}

func press(m review, msgs ...tea.Msg) review {
	for _, msg := range msgs {
		next, _ := m.Update(msg)
		m = next.(review)
	}
	return m
}

func TestReview_Screens(t *testing.T) {
	tests := []struct {
		name string
		plan model.Plan
		mode Mode
		keys []tea.Msg
	}{
		{name: "plan review", plan: bootstrapPlan(), mode: ModeApply},
		{name: "details expanded", plan: bootstrapPlan(), mode: ModeApply, keys: []tea.Msg{down, down, down, right}},
		{name: "details collapsed again", plan: bootstrapPlan(), mode: ModeApply, keys: []tea.Msg{down, down, down, right, left}},
		{name: "step skipped", plan: bootstrapPlan(), mode: ModeApply, keys: []tea.Msg{down, space}},
		{name: "drift", plan: driftPlan(), mode: ModeDrift, keys: []tea.Msg{down, right, down, right, up}},
		{name: "scrolls to keep the cursor visible", plan: bootstrapPlan(), mode: ModeApply, keys: []tea.Msg{
			tea.WindowSizeMsg{Width: 80, Height: 10}, down, down, down, right, down,
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := press(newTestReview(tt.plan, tt.mode), tt.keys...)

			golden.RequireEqual(t, []byte(m.View()))
		})
	}
}

func runReview(t *testing.T, p model.Plan, keys ...tea.KeyMsg) Decision {
	t.Helper()

	tm := teatest.NewTestModel(t, newTestReview(p, ModeApply), teatest.WithInitialTermSize(100, 30))
	for _, k := range keys {
		tm.Send(k)
	}
	final := tm.FinalModel(t, teatest.WithFinalTimeout(2*time.Second))
	return final.(review).decision
}

func TestReview_Decisions(t *testing.T) {
	tests := []struct {
		name string
		keys []tea.KeyMsg
		want Decision
	}{
		{name: "enter applies everything", keys: []tea.KeyMsg{enter}, want: Decision{Apply: true}},
		{name: "skipped steps are left out", keys: []tea.KeyMsg{down, space, down, down, space, enter}, want: Decision{Apply: true, Skip: []int{1, 3}}},
		{name: "space twice keeps the step", keys: []tea.KeyMsg{space, space, enter}, want: Decision{Apply: true}},
		{name: "up-to-date steps cannot be skipped", keys: []tea.KeyMsg{down, down, down, down, space, enter}, want: Decision{Apply: true}},
		{name: "skipping every change applies nothing", keys: []tea.KeyMsg{space, down, space, down, space, down, space, down, down, space, enter}, want: Decision{Skip: []int{0, 1, 2, 3, 5}}},
		{name: "q quits without applying", keys: []tea.KeyMsg{down, space, quit}, want: Decision{}},
		{name: "ctrl+c quits without applying", keys: []tea.KeyMsg{{Type: tea.KeyCtrlC}}, want: Decision{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, runReview(t, bootstrapPlan(), tt.keys...))
		})
	}
}
