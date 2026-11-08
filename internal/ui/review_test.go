package ui

import (
	"booster/internal/model"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	down  = tea.KeyMsg{Type: tea.KeyDown}
	up    = tea.KeyMsg{Type: tea.KeyUp}
	right = tea.KeyMsg{Type: tea.KeyRight}
	left  = tea.KeyMsg{Type: tea.KeyLeft}
	space = tea.KeyMsg{Type: tea.KeySpace}
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

func TestReview_Decisions(t *testing.T) {
	const keyDown, keySpace, keyEnter = "\x1b[B", " ", "\r"
	tests := []struct {
		name string
		plan model.Plan
		mode Mode
		keys []string
		want Decision
	}{
		{name: "enter applies everything", plan: bootstrapPlan(), keys: []string{keyEnter}, want: Decision{Apply: true}},
		{name: "skipped steps are left out", plan: bootstrapPlan(), keys: []string{keyDown, keySpace, keyDown, keyDown, keySpace, keyEnter}, want: Decision{Apply: true, Skip: []int{1, 3}}},
		{name: "space twice keeps the step", plan: bootstrapPlan(), keys: []string{keySpace, keySpace, keyEnter}, want: Decision{Apply: true}},
		{name: "up-to-date steps cannot be skipped", plan: bootstrapPlan(), keys: []string{keyDown, keyDown, keyDown, keyDown, keySpace, keyEnter}, want: Decision{Apply: true}},
		{name: "skipping every change applies nothing", plan: bootstrapPlan(), keys: []string{keySpace, keyDown, keySpace, keyDown, keySpace, keyDown, keySpace, keyDown, keyDown, keySpace, keyEnter}, want: Decision{Skip: []int{0, 1, 2, 3, 5}}},
		{name: "q quits without applying", plan: bootstrapPlan(), keys: []string{keyDown, keySpace, "q"}, want: Decision{}},
		{name: "ctrl+c quits without applying", plan: bootstrapPlan(), keys: []string{"\x03"}, want: Decision{}},
		{name: "enter applies the drift fixes", plan: driftPlan(), mode: ModeDrift, keys: []string{keyDown, keySpace, keyEnter}, want: Decision{Apply: true, Skip: []int{1}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := strings.NewReader(strings.Join(tt.keys, ""))
			var started []Decision
			start := func(d Decision) (Job, error) {
				started = append(started, d)
				return Job{Plan: tt.plan, Run: func(context.Context, func(model.Event)) {}}, nil
			}

			got, err := Review(t.Context(), in, io.Discard, header, tt.plan, tt.mode, start)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			if tt.want.Apply {
				assert.Equal(t, []Decision{tt.want}, started)
			} else {
				assert.Empty(t, started)
			}
		})
	}
}

func tallPlan(n int) model.Plan {
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf("item-%02d", i)
	}
	return model.Plan{Steps: []model.PlannedStep{
		{Name: "packages", Change: model.Change{Kind: model.ChangeCreate, Verb: "install", Total: n, Items: items(names...)}},
		{Name: "links", Change: model.Change{Kind: model.ChangeCreate, Verb: "link", Total: 1, Items: items("~/.zshrc")}},
	}}
}

func TestReview_EveryItemOfATallStepCanBeScrolledIntoView(t *testing.T) {
	const height, count = 30, 50
	m := press(newTestReview(tallPlan(count), ModeApply), tea.WindowSizeMsg{Width: 80, Height: height}, right)

	seen := map[string]bool{}
	look := func() {
		rows := strings.Split(strings.TrimSuffix(m.View(), "\n"), "\n")
		require.LessOrEqual(t, len(rows), height)
		for _, r := range rows {
			seen[strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(r), "+"))] = true
		}
	}
	look()
	for range count {
		m = press(m, down)
		look()
	}

	for i := range count {
		assert.True(t, seen[fmt.Sprintf("item-%02d", i)], "item-%02d never came into view", i)
	}
	assert.Equal(t, 1, m.cursor, "the cursor moves on once the last item is visible")

	seen = map[string]bool{}
	for range count {
		m = press(m, up)
		look()
	}
	assert.True(t, seen["item-00"], "scrolling back up reaches the first item")
	assert.Equal(t, 0, m.cursor)
}

func TestReview_RowsFitANarrowTerminal(t *testing.T) {
	long := strings.Repeat("very-long-path-segment/", 6)
	plan := model.Plan{Excluded: 2, Steps: []model.PlannedStep{
		{Name: "a-task-name-much-longer-than-the-terminal-is-wide", Change: model.Change{Kind: model.ChangeCreate, Verb: "link", Total: 3, NeedsSudo: true, Items: []model.Item{
			{Name: "~/" + long + "zshrc", To: long + "dotfiles/zshrc"},
			{Name: "パッケージマネージャーの設定ファイル", From: long, To: long},
			{Name: "fd"},
		}}},
		{Name: "git-config", Change: model.Change{Kind: model.ChangeUpdate, Verb: "set", Total: 1, Items: items("user.name")}},
	}}
	const width, height = 40, 10

	for _, mode := range []Mode{ModeApply, ModeDrift} {
		m := press(newTestReview(plan, mode), tea.WindowSizeMsg{Width: width, Height: height}, right, space)
		for _, key := range []tea.Msg{nil, down, down, down, down} {
			if key != nil {
				m = press(m, key)
			}
			view := m.View()
			rows := strings.Split(strings.TrimSuffix(view, "\n"), "\n")

			assert.LessOrEqual(t, len(rows), height)
			for _, r := range rows {
				assert.LessOrEqual(t, ansi.StringWidth(r), width, "row %q", r)
			}
			assert.Contains(t, view, "…")
		}
	}
}
