package run

import (
	"booster/internal/action"
	"booster/internal/config"
	"booster/internal/expr"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func parseTasks(t *testing.T, src string) []config.Task {
	t.Helper()

	var cfg config.Config
	require.NoError(t, yaml.Unmarshal([]byte(src), &cfg))

	return cfg.Tasks
}

func testContext() *expr.Context {
	return &expr.Context{OS: "linux", Arch: "amd64", Home: "/home/test", Profile: "work"}
}

func stepNames(steps []Step) []string {
	names := make([]string, 0, len(steps))
	for _, s := range steps {
		names = append(names, s.Name)
	}
	return names
}

func TestCompile(t *testing.T) {
	tests := []struct {
		name         string
		yaml         string
		wantSteps    []string
		wantExcluded int
	}{
		{
			name: "keeps task order and defaults the step name to the action",
			yaml: `
tasks:
  - action: dir.create
    args: [/tmp/a]
  - name: dotfiles
    action: symlink.create
    args:
      - {source: /tmp/a, target: /tmp/b}
`,
			wantSteps: []string{"dir.create", "dotfiles"},
		},
		{
			name: "drops steps whose static guard is false",
			yaml: `
tasks:
  - name: mac dirs
    action: dir.create
    when: ${ os == "darwin" }
    args: [/tmp/a]
  - name: linux dirs
    action: dir.create
    when: ${ os == "linux" && profile == "work" }
    args: [/tmp/b]
  - name: personal dirs
    action: dir.create
    when: ${ profile == "personal" }
    args: [/tmp/c]
`,
			wantSteps:    []string{"linux dirs"},
			wantExcluded: 2,
		},
		{
			name: "resolves expressions in args against the context",
			yaml: `
tasks:
  - name: config
    action: dir.create
    args: ["${ home }/.config"]
`,
			wantSteps: []string{"config"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled, err := Compile(parseTasks(t, tt.yaml), testContext(), action.Registry())

			require.NoError(t, err)
			assert.Equal(t, tt.wantSteps, stepNames(compiled.Steps))
			assert.Equal(t, tt.wantExcluded, compiled.Excluded)
		})
	}
}

func TestCompile_BuildsActionsFromResolvedArgs(t *testing.T) {
	tasks := parseTasks(t, `
tasks:
  - action: dir.create
    args: ["${ home }/.config"]
`)

	compiled, err := Compile(tasks, testContext(), action.Registry())

	require.NoError(t, err)
	require.Len(t, compiled.Steps, 1)
	assert.Equal(t, &action.DirCreate{Paths: []string{"/home/test/.config"}}, compiled.Steps[0].Action)
}

func TestCompile_Errors(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr []string
	}{
		{
			name: "unknown action",
			yaml: `
tasks:
  - name: mystery
    action: nope.nothing
    args: []
`,
			wantErr: []string{`task 1 "mystery"`, `unknown action "nope.nothing"`},
		},
		{
			name: "bad args",
			yaml: `
tasks:
  - name: dirs
    action: dir.create
    args: {path: /tmp/a}
`,
			wantErr: []string{`task 1 "dirs"`, "args must be a list of paths"},
		},
		{
			name: "invalid when expression",
			yaml: `
tasks:
  - name: dirs
    action: dir.create
    when: ${ os == }
    args: [/tmp/a]
`,
			wantErr: []string{`task 1 "dirs"`, "when"},
		},
		{
			name: "non-boolean when",
			yaml: `
tasks:
  - name: dirs
    action: dir.create
    when: ${ os }
    args: [/tmp/a]
`,
			wantErr: []string{`task 1 "dirs"`, "must evaluate to bool"},
		},
		{
			name: "reports every broken task at once",
			yaml: `
tasks:
  - name: first
    action: nope.one
  - name: fine
    action: dir.create
    args: [/tmp/a]
  - name: third
    action: symlink.create
    args: [/tmp/a]
`,
			wantErr: []string{`task 1 "first"`, `task 3 "third"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Compile(parseTasks(t, tt.yaml), testContext(), action.Registry())

			require.Error(t, err)
			for _, want := range tt.wantErr {
				assert.Contains(t, err.Error(), want)
			}
		})
	}
}
