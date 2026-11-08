package action_test

import (
	"booster/internal/action"
	"booster/internal/config"
	"booster/internal/expr"
	"booster/internal/model"
	"booster/internal/run"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type fakeBrew struct {
	t        *testing.T
	formulae []string
	casks    []string
}

func (f *fakeBrew) LookPath(name string) (string, error) {
	if name == "brew" {
		return "/opt/homebrew/bin/brew", nil
	}
	return "", errors.New("not found")
}

func (f *fakeBrew) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	command := strings.Join(args, " ")
	if filepath.Base(name) == "brew" {
		switch command {
		case "list --formulae":
			return []byte(strings.Join(f.formulae, "\n")), nil
		case "list --casks":
			return []byte(strings.Join(f.casks, "\n")), nil
		}
	}

	f.t.Errorf("unexpected command: %s %s", name, command)
	return nil, errors.New("unexpected command: " + name + " " + command)
}

func planDarwin(t *testing.T, machine *fakeBrew, src string) run.PlanResult {
	t.Helper()

	var cfg config.Config
	require.NoError(t, yaml.Unmarshal([]byte(src), &cfg))

	ctx := &expr.Context{OS: "darwin", Arch: "arm64", Home: "/Users/test"}
	compiled, err := run.Compile(cfg.Tasks, ctx, action.NewRegistry(machine))
	require.NoError(t, err)

	result, err := run.Plan(context.Background(), compiled)
	require.NoError(t, err)
	return result
}

func TestDarwinPackages(t *testing.T) {
	tests := []struct {
		name      string
		args      string
		items     []model.Item
		needsSudo bool
	}{
		{
			name:  "missing formulae install without sudo",
			args:  "[fd, yq]",
			items: []model.Item{{Name: "yq"}},
		},
		{
			name:      "missing casks need sudo upfront for brew's own prompt",
			args:      "[{packages: [fd, yq], casks: [firefox, ghostty]}]",
			items:     []model.Item{{Name: "yq"}, {Name: "ghostty"}},
			needsSudo: true,
		},
		{
			name:  "installed casks with missing formulae stay without sudo",
			args:  "[{packages: [yq], casks: [firefox]}]",
			items: []model.Item{{Name: "yq"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			machine := &fakeBrew{t: t, formulae: []string{"fd"}, casks: []string{"firefox"}}

			result := planDarwin(t, machine, "tasks:\n  - action: pkg.install\n    args: "+tt.args+"\n")

			change := result.Steps[0].Change
			assert.Equal(t, model.ChangeCreate, change.Kind)
			assert.Equal(t, tt.items, change.Items)
			assert.Equal(t, tt.needsSudo, change.NeedsSudo)
			assert.Equal(t, tt.needsSudo, result.View().NeedsSudo())
		})
	}
}
