package action

import (
	"booster/internal/expr"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func argsNode(t *testing.T, src string) yaml.Node {
	t.Helper()

	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(src), &doc))
	require.NotEmpty(t, doc.Content)

	return *doc.Content[0]
}

func testContext() *expr.Context {
	return &expr.Context{
		OS:      "arch",
		Home:    "/home/tester",
		Profile: "personal",
		Env:     map[string]string{"EDITOR": "nvim"},
		Vars:    map[string]any{"config_dir": "/home/tester/.config"},
	}
}

func TestDecodeSourceTargets(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		want    []SourceTarget
		wantErr string
	}{
		{
			name: "resolves expressions in both fields",
			args: `
- source: dotfiles/zshrc
  target: ${ vars.config_dir }/zshrc
`,
			want: []SourceTarget{{Source: "dotfiles/zshrc", Target: "/home/tester/.config/zshrc"}},
		},
		{
			name:    "rejects a scalar",
			args:    `not a list`,
			wantErr: "args must be a list of {source, target} entries",
		},
		{
			name:    "rejects a non-mapping entry",
			args:    `- just a string`,
			wantErr: "[0]: must be a mapping",
		},
		{
			name:    "rejects a missing source",
			args:    `- target: ~/.zshrc`,
			wantErr: "[0]: missing 'source'",
		},
		{
			name:    "rejects a missing target",
			args:    `- source: dotfiles/zshrc`,
			wantErr: "[0]: missing 'target'",
		},
		{
			name:    "reports which field failed to resolve",
			args:    `- {source: dotfiles/zshrc, target: "${ nope() }"}`,
			wantErr: "[0].target",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeSourceTargets(argsNode(t, tt.args), testContext())

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
