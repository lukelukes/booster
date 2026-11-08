package action

import (
	"booster/internal/cmdexec"
	"booster/internal/model"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type process struct {
	name string
	args []string
}

type fakeProcesses struct {
	failing map[string]bool
	calls   []process
}

func (f *fakeProcesses) runner() *cmdexec.MockRunner {
	return &cmdexec.MockRunner{
		RunFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
			f.calls = append(f.calls, process{name: name, args: args})
			if f.failing[name] {
				return []byte("boom\n"), errors.New("exit status 1")
			}
			return []byte("ok\n"), nil
		},
	}
}

func newTestCmd(t *testing.T, procs *fakeProcesses, args string) Action {
	t.Helper()

	act, err := NewCmd(argsNode(t, args), testContext(), procs.runner())
	require.NoError(t, err)
	return act
}

func TestNewCmd_InvalidArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		wantErr string
	}{
		{name: "args must be a mapping", args: "[echo, hi]", wantErr: "args must be a mapping"},
		{name: "run is required", args: "{check: [true]}", wantErr: "missing 'run'"},
		{name: "run must be a list", args: "{run: echo hi}", wantErr: "run"},
		{name: "a bad expression fails", args: `{run: [echo, "${ nope( }"]}`, wantErr: "run[1]"},
		{name: "run program must not be empty", args: `{run: [""]}`, wantErr: "run[0]: program must not be empty"},
		{name: "run program resolving to empty fails", args: `{run: ["${ env.UNSET_VAR }", x]}`, wantErr: "run[0]: program must not be empty"},
		{name: "check program must not be empty", args: `{check: ["", x], run: [echo]}`, wantErr: "check[0]: program must not be empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCmd(argsNode(t, tt.args), testContext(), &cmdexec.MockRunner{})

			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestCmd_PassesArgvUnchanged(t *testing.T) {
	tests := []struct {
		name string
		args string
		want []process
	}{
		{
			name: "check then run",
			args: "{check: [xcode-select, -p], run: [xcode-select, --install]}",
			want: []process{{"xcode-select", []string{"-p"}}, {"xcode-select", []string{"--install"}}},
		},
		{
			name: "expressions resolve in check and run",
			args: `{check: [test, -d, "${ vars.config_dir }/nvim"], run: [git, clone, repo, "${ home }/.config/nvim"]}`,
			want: []process{
				{"test", []string{"-d", "/home/tester/.config/nvim"}},
				{"git", []string{"clone", "repo", "/home/tester/.config/nvim"}},
			},
		},
		{
			name: "spaces, quotes and shell metacharacters stay inside one argument",
			args: `{check: [test, -f, "a b $HOME; * 'q' \"d\""], run: [touch, "a b $HOME; * 'q' \"d\"", ""]}`,
			want: []process{
				{"test", []string{"-f", `a b $HOME; * 'q' "d"`}},
				{"touch", []string{`a b $HOME; * 'q' "d"`, ""}},
			},
		},
		{
			name: "sudo prefixes only the run command",
			args: "{check: [check-shell], run: [chsh, -s, /bin/zsh], sudo: true}",
			want: []process{{"check-shell", []string{}}, {"sudo", []string{"chsh", "-s", "/bin/zsh"}}},
		},
		{
			name: "no check runs only the command",
			args: "{run: [chsh, -s, /bin/zsh]}",
			want: []process{{"chsh", []string{"-s", "/bin/zsh"}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			procs := &fakeProcesses{}
			act := newTestCmd(t, procs, tt.args)

			_, err := act.Check(context.Background())
			require.NoError(t, err)
			require.NoError(t, act.Apply(context.Background()))

			assert.Equal(t, tt.want, procs.calls)
		})
	}
}

func TestCmd_Check(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		failing map[string]bool
		want    model.Change
		ran     []process
	}{
		{
			name: "check exiting 0 is up to date",
			args: "{check: [xcode-select, -p], run: [xcode-select, --install]}",
			want: model.Change{Verb: "run", Total: 1, Kind: model.ChangeNone},
			ran:  []process{{"xcode-select", []string{"-p"}}},
		},
		{
			name:    "check failing plans a create",
			args:    "{check: [xcode-select, -p], run: [xcode-select, --install]}",
			failing: map[string]bool{"xcode-select": true},
			want:    model.Change{Verb: "run", Total: 1, Kind: model.ChangeCreate, Items: []model.Item{{Name: "xcode-select --install"}}},
			ran:     []process{{"xcode-select", []string{"-p"}}},
		},
		{
			name: "no check is unknown",
			args: "{run: [chsh, -s, /bin/zsh]}",
			want: model.Change{Verb: "run", Total: 1, Kind: model.ChangeUnknown, Items: []model.Item{{Name: "chsh -s /bin/zsh"}}},
		},
		{
			name:    "sudo marks a pending change",
			args:    "{check: [check-shell], run: [chsh, -s, /bin/zsh], sudo: true}",
			failing: map[string]bool{"check-shell": true},
			want:    model.Change{Verb: "run", Total: 1, Kind: model.ChangeCreate, NeedsSudo: true, Items: []model.Item{{Name: "chsh -s /bin/zsh"}}},
			ran:     []process{{"check-shell", []string{}}},
		},
		{
			name: "sudo is not needed when up to date",
			args: "{check: [check-shell], run: [chsh, -s, /bin/zsh], sudo: true}",
			want: model.Change{Verb: "run", Total: 1, Kind: model.ChangeNone},
			ran:  []process{{"check-shell", []string{}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			procs := &fakeProcesses{failing: tt.failing}

			got, err := newTestCmd(t, procs, tt.args).Check(context.Background())

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.ran, procs.calls)
		})
	}
}

func TestCmd_ApplyFailureIncludesOutput(t *testing.T) {
	procs := &fakeProcesses{failing: map[string]bool{"mas": true}}

	err := newTestCmd(t, procs, "{run: [mas, install, '497799835']}").Apply(context.Background())

	require.ErrorContains(t, err, "mas install 497799835")
	require.ErrorContains(t, err, "boom")
}
