package action

import (
	"booster/internal/cmdexec"
	"booster/internal/model"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeShell struct {
	done    map[string]bool
	failRun bool
	ran     []string
}

func (s *fakeShell) runner() *cmdexec.MockRunner {
	return &cmdexec.MockRunner{
		RunFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
			line := strings.Join(append([]string{name}, args...), " ")
			if s.done[line] {
				return []byte("/Library/Developer/CommandLineTools\n"), nil
			}
			if strings.HasPrefix(line, "check") || strings.HasPrefix(line, "xcode-select -p") {
				return nil, errors.New("exit status 2")
			}
			if s.failRun {
				return []byte("boom\n"), errors.New("exit status 1")
			}
			s.ran = append(s.ran, line)
			return []byte("installing\n"), nil
		},
	}
}

func newTestCmd(t *testing.T, shell *fakeShell, args string) Action {
	t.Helper()

	act, err := NewCmd(argsNode(t, args), testContext(), shell.runner())
	require.NoError(t, err)
	return act
}

func TestNewCmd_Args(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		want    *Cmd
		wantErr string
	}{
		{
			name: "check and run",
			args: "{check: [xcode-select, -p], run: [xcode-select, --install]}",
			want: &Cmd{CheckCmd: []string{"xcode-select", "-p"}, RunCmd: []string{"xcode-select", "--install"}},
		},
		{
			name: "expressions resolve in check and run",
			args: `{check: [test, -d, "${ vars.config_dir }/nvim"], run: [git, clone, repo, "${ home }/.config/nvim"], sudo: true}`,
			want: &Cmd{
				CheckCmd: []string{"test", "-d", "/home/tester/.config/nvim"},
				RunCmd:   []string{"git", "clone", "repo", "/home/tester/.config/nvim"},
				Sudo:     true,
			},
		},
		{
			name: "check is optional",
			args: "{run: [chsh, -s, /bin/zsh]}",
			want: &Cmd{RunCmd: []string{"chsh", "-s", "/bin/zsh"}},
		},
		{name: "args must be a mapping", args: "[echo, hi]", wantErr: "args must be a mapping"},
		{name: "run is required", args: "{check: [true]}", wantErr: "missing 'run'"},
		{name: "run must be a list", args: "{run: echo hi}", wantErr: "run"},
		{name: "a bad expression fails", args: `{run: [echo, "${ nope( }"]}`, wantErr: "run[1]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			act, err := NewCmd(argsNode(t, tt.args), testContext(), &cmdexec.MockRunner{})

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			got := act.(*Cmd)
			got.Runner = nil
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCmd_Check(t *testing.T) {
	tests := []struct {
		name string
		args string
		done map[string]bool
		want model.Change
	}{
		{
			name: "check exiting 0 is up to date",
			args: "{check: [xcode-select, -p], run: [xcode-select, --install]}",
			done: map[string]bool{"xcode-select -p": true},
			want: model.Change{Verb: "run", Total: 1, Kind: model.ChangeNone},
		},
		{
			name: "check failing plans a create",
			args: "{check: [xcode-select, -p], run: [xcode-select, --install]}",
			want: model.Change{Verb: "run", Total: 1, Kind: model.ChangeCreate, Items: []model.Item{{Name: "xcode-select --install"}}},
		},
		{
			name: "no check is unknown",
			args: "{run: [chsh, -s, /bin/zsh]}",
			want: model.Change{Verb: "run", Total: 1, Kind: model.ChangeUnknown, Items: []model.Item{{Name: "chsh -s /bin/zsh"}}},
		},
		{
			name: "sudo marks a pending change",
			args: "{check: [check-shell], run: [chsh, -s, /bin/zsh], sudo: true}",
			want: model.Change{Verb: "run", Total: 1, Kind: model.ChangeCreate, NeedsSudo: true, Items: []model.Item{{Name: "chsh -s /bin/zsh"}}},
		},
		{
			name: "sudo is not needed when up to date",
			args: "{check: [check-shell], run: [chsh, -s, /bin/zsh], sudo: true}",
			done: map[string]bool{"check-shell": true},
			want: model.Change{Verb: "run", Total: 1, Kind: model.ChangeNone},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shell := &fakeShell{done: tt.done}

			got, err := newTestCmd(t, shell, tt.args).Check(context.Background())

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Empty(t, shell.ran)
		})
	}
}

func TestCmd_Apply(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		wantRan []string
	}{
		{
			name:    "runs the command",
			args:    "{check: [xcode-select, -p], run: [xcode-select, --install]}",
			wantRan: []string{"xcode-select --install"},
		},
		{
			name:    "sudo prefixes the command",
			args:    "{run: [chsh, -s, /bin/zsh], sudo: true}",
			wantRan: []string{"sudo chsh -s /bin/zsh"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shell := &fakeShell{}

			require.NoError(t, newTestCmd(t, shell, tt.args).Apply(context.Background()))

			assert.Equal(t, tt.wantRan, shell.ran)
		})
	}
}

func TestCmd_ApplyFailureIncludesOutput(t *testing.T) {
	shell := &fakeShell{failRun: true}

	err := newTestCmd(t, shell, "{run: [mas, install, '497799835']}").Apply(context.Background())

	require.ErrorContains(t, err, "mas install 497799835")
	require.ErrorContains(t, err, "boom")
}
