package ui

import (
	"booster/internal/model"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func render(t *testing.T, p model.Plan) string {
	t.Helper()

	var sb strings.Builder
	require.NoError(t, RenderPlan(&sb, p))
	return sb.String()
}

func TestRenderPlan(t *testing.T) {
	tests := []struct {
		name string
		plan model.Plan
		want string
	}{
		{
			name: "mixed creates, updates and up-to-date steps",
			plan: model.Plan{
				Steps: []model.PlannedStep{
					{Name: "dirs", Change: model.Change{
						Kind: model.ChangeCreate, Verb: "create", Total: 3,
						Items: []model.Item{{Name: "~/.config"}, {Name: "~/code"}},
					}},
					{Name: "dotfiles", Change: model.Change{
						Kind: model.ChangeUpdate, Verb: "link", Total: 2,
						Items: []model.Item{
							{Name: "/home/me/.zshrc", From: "/old/zshrc", To: "/dot/zshrc"},
							{Name: "/home/me/.vimrc", To: "/dot/vimrc"},
						},
					}},
					{Name: "symlink.create", Change: model.Change{Verb: "link", Total: 4}},
				},
				Excluded: 2,
			},
			want: `+ dirs            create 2 · 1 up to date
    ~/.config
    ~/code
~ dotfiles        link 2
    /home/me/.zshrc  /old/zshrc → /dot/zshrc
    /home/me/.vimrc  → /dot/vimrc
✓ symlink.create  up to date

2 to change · 1 up to date · 2 excluded
`,
		},
		{
			name: "everything up to date",
			plan: model.Plan{Steps: []model.PlannedStep{
				{Name: "dirs", Change: model.Change{Verb: "create", Total: 1}},
			}},
			want: `✓ dirs  up to date

0 to change · 1 up to date
`,
		},
		{
			name: "unknown change",
			plan: model.Plan{Steps: []model.PlannedStep{
				{Name: "tools", Change: model.Change{Kind: model.ChangeUnknown, Verb: "install"}},
			}},
			want: `? tools  known after apply

1 to change · 0 up to date
`,
		},
		{
			name: "no steps",
			plan: model.Plan{Excluded: 1},
			want: `0 to change · 0 up to date · 1 excluded
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, render(t, tt.plan))
		})
	}
}
