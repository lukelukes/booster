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

func miseRunner(versions map[string]string, found bool) *cmdexec.MockRunner {
	return &cmdexec.MockRunner{
		LookPathFunc: func(string) (string, error) {
			if !found {
				return "", errors.New("not found")
			}
			return "/usr/bin/mise", nil
		},
		RunFunc: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			if len(args) == 2 && args[0] == "current" {
				version, ok := versions[args[1]]
				if !ok {
					return nil, errors.New("no version set")
				}
				return []byte(version + "\n"), nil
			}
			return nil, nil
		},
	}
}

func TestMiseUse_Check(t *testing.T) {
	t.Run("reports unknown when mise is not installed yet", func(t *testing.T) {
		action := &MiseUse{
			Runner: miseRunner(nil, false),
			Tools:  []ToolSpec{{Name: "node", Version: "22"}},
		}

		change, err := action.Check(context.Background())

		require.NoError(t, err)
		assert.Equal(t, model.ChangeUnknown, change.Kind)
		assert.Equal(t, 1, change.Total)
		assert.Empty(t, change.Items)
	})

	t.Run("reports no change when every tool is at its version", func(t *testing.T) {
		action := &MiseUse{
			Runner: miseRunner(map[string]string{"node": "22"}, true),
			Tools:  []ToolSpec{{Name: "node", Version: "22"}},
		}

		change, err := action.Check(context.Background())

		require.NoError(t, err)
		assert.Equal(t, model.ChangeNone, change.Kind)
		assert.Empty(t, change.Items)
	})

	t.Run("reports a different version as an update", func(t *testing.T) {
		action := &MiseUse{
			Runner: miseRunner(map[string]string{"node": "20"}, true),
			Tools:  []ToolSpec{{Name: "node", Version: "22"}},
		}

		change, err := action.Check(context.Background())

		require.NoError(t, err)
		assert.Equal(t, model.ChangeUpdate, change.Kind)
		assert.Equal(t, []model.Item{{Name: "node", From: "20", To: "22"}}, change.Items)
	})

	t.Run("reports an unconfigured tool as a create", func(t *testing.T) {
		action := &MiseUse{
			Runner: miseRunner(map[string]string{"node": "22"}, true),
			Tools:  []ToolSpec{{Name: "node", Version: "22"}, {Name: "go", Version: "1.26"}},
		}

		change, err := action.Check(context.Background())

		require.NoError(t, err)
		assert.Equal(t, model.ChangeCreate, change.Kind)
		assert.Equal(t, []model.Item{{Name: "go", To: "1.26"}}, change.Items)
	})
}

func TestMiseUse_Apply(t *testing.T) {
	t.Run("configures only the tools that differ", func(t *testing.T) {
		runner := miseRunner(map[string]string{"node": "22", "go": "1.25"}, true)
		action := &MiseUse{
			Runner: runner,
			Tools:  []ToolSpec{{Name: "node", Version: "22"}, {Name: "go", Version: "1.26"}},
		}

		require.NoError(t, action.Apply(context.Background()))

		var used []string
		for _, call := range runner.Calls {
			if len(call.Args) > 0 && call.Args[0] == "use" {
				used = append(used, strings.Join(call.Args, " "))
			}
		}
		assert.Equal(t, []string{"use --global go@1.26"}, used)
	})

	t.Run("fails when mise is still missing", func(t *testing.T) {
		action := &MiseUse{
			Runner: miseRunner(nil, false),
			Tools:  []ToolSpec{{Name: "node", Version: "22"}},
		}

		err := action.Apply(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "mise not found in PATH")
	})

	t.Run("propagates a failure from mise", func(t *testing.T) {
		runner := &cmdexec.MockRunner{
			RunFunc: func(_ context.Context, _ string, args ...string) ([]byte, error) {
				if args[0] == "use" {
					return []byte("no such version"), errors.New("exit status 1")
				}
				return nil, errors.New("no version set")
			},
		}
		action := &MiseUse{Runner: runner, Tools: []ToolSpec{{Name: "node", Version: "99"}}}

		err := action.Apply(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "mise use node@99")
	})
}

func TestNewMiseUse(t *testing.T) {
	t.Run("parses specs after resolving them", func(t *testing.T) {
		ctx := testContext()
		ctx.Vars["node_version"] = "22"

		created, err := NewMiseUse(argsNode(t, "- node@${ vars.node_version }\n- go@1.26"), ctx, nil)

		require.NoError(t, err)
		assert.Equal(t, []ToolSpec{
			{Name: "node", Version: "22"},
			{Name: "go", Version: "1.26"},
		}, created.(*MiseUse).Tools)
	})

	t.Run("rejects a spec without a version", func(t *testing.T) {
		_, err := NewMiseUse(argsNode(t, "- node@22\n- go"), testContext(), nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "[1]: invalid tool spec")
	})

	t.Run("rejects a scalar", func(t *testing.T) {
		_, err := NewMiseUse(argsNode(t, "node@22"), testContext(), nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "args must be a list")
	})
}
