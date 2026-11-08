package action_test

import (
	"booster/internal/action"
	"booster/internal/config"
	"booster/internal/expr"
	"booster/internal/model"
	"booster/internal/run"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type exitError struct {
	code int
}

func (e exitError) Error() string { return fmt.Sprintf("exit status %d", e.code) }

func (e exitError) ExitCode() int { return e.code }

type defaultsFailure struct {
	output string
	err    error
}

type fakeDefaults struct {
	t        *testing.T
	values   map[string]string
	failures map[string]defaultsFailure
	writeErr error
	writes   [][]string
}

func (f *fakeDefaults) LookPath(name string) (string, error) {
	return "/usr/bin/" + name, nil
}

func (f *fakeDefaults) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	if name != "defaults" || len(args) < 3 {
		f.t.Errorf("unexpected command: %s %v", name, args)
		return nil, errors.New("unexpected command")
	}

	key := args[1] + " " + args[2]
	switch {
	case args[0] == "read" && len(args) == 3:
		if failure, ok := f.failures[key]; ok {
			return []byte(failure.output), failure.err
		}
		value, ok := f.values[key]
		if !ok {
			return []byte("The domain/default pair of (" + args[1] + ", " + args[2] + ") does not exist\n"), exitError{code: 1}
		}
		return []byte(value + "\n"), nil
	case args[0] == "write" && len(args) == 5:
		if f.writeErr != nil {
			return []byte("Could not write domain\n"), f.writeErr
		}
		f.writes = append(f.writes, args[1:])
		f.values[key] = args[4]
		return nil, nil
	}

	f.t.Errorf("unexpected command: %s %v", name, args)
	return nil, errors.New("unexpected command")
}

const sampleDefaults = `defaults:
  - {domain: NSGlobalDomain, key: KeyRepeat, type: int, value: 2}
  - {domain: NSGlobalDomain, key: AppleShowAllExtensions, type: bool, value: true}
  - {domain: com.apple.dock, key: tilesize, type: float, value: 48}
  - {domain: com.apple.screencapture, key: type, type: string, value: png}
`

const defaultsTask = `
tasks:
  - name: macos
    action: set.darwin.defaults
    args: {file: macos.yaml}
`

func compileDefaults(t *testing.T, machine *fakeDefaults, file, tasks string) (run.Compiled, error) {
	t.Helper()

	dir := t.TempDir()
	if file != "" {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "macos.yaml"), []byte(file), 0o644))
	}

	var cfg config.Config
	require.NoError(t, yaml.Unmarshal([]byte(tasks), &cfg))

	ctx := &expr.Context{OS: "darwin", Arch: "arm64", Home: "/Users/test", ConfigDir: dir}
	return run.Compile(cfg.Tasks, ctx, action.NewRegistry(machine))
}

func planDefaults(t *testing.T, machine *fakeDefaults, file string) (run.PlanResult, error) {
	t.Helper()

	compiled, err := compileDefaults(t, machine, file, defaultsTask)
	require.NoError(t, err)

	return run.Plan(context.Background(), compiled)
}

func TestDarwinDefaults_Plan(t *testing.T) {
	tests := []struct {
		name      string
		current   map[string]string
		wantKind  model.ChangeKind
		wantItems []model.Item
	}{
		{
			name: "reports nothing when every key already matches after normalization",
			current: map[string]string{
				"NSGlobalDomain KeyRepeat":              "2",
				"NSGlobalDomain AppleShowAllExtensions": "1",
				"com.apple.dock tilesize":               "48.0",
				"com.apple.screencapture type":          "png",
			},
			wantKind: model.ChangeNone,
		},
		{
			name: "lists each drifted key with its current and desired value",
			current: map[string]string{
				"NSGlobalDomain KeyRepeat":              "6",
				"NSGlobalDomain AppleShowAllExtensions": "0",
				"com.apple.dock tilesize":               "36.5",
				"com.apple.screencapture type":          "png",
			},
			wantKind: model.ChangeUpdate,
			wantItems: []model.Item{
				{Name: "NSGlobalDomain KeyRepeat", From: "6", To: "2"},
				{Name: "NSGlobalDomain AppleShowAllExtensions", From: "false", To: "true"},
				{Name: "com.apple.dock tilesize", From: "36.5", To: "48"},
			},
		},
		{
			name:     "reports missing keys as created",
			current:  map[string]string{},
			wantKind: model.ChangeCreate,
			wantItems: []model.Item{
				{Name: "NSGlobalDomain KeyRepeat", To: "2"},
				{Name: "NSGlobalDomain AppleShowAllExtensions", To: "true"},
				{Name: "com.apple.dock tilesize", To: "48"},
				{Name: "com.apple.screencapture type", To: "png"},
			},
		},
		{
			name: "shows a current value of the wrong type as is",
			current: map[string]string{
				"NSGlobalDomain KeyRepeat":              "fast",
				"NSGlobalDomain AppleShowAllExtensions": "1",
				"com.apple.dock tilesize":               "48",
				"com.apple.screencapture type":          "png",
			},
			wantKind:  model.ChangeUpdate,
			wantItems: []model.Item{{Name: "NSGlobalDomain KeyRepeat", From: "fast", To: "2"}},
		},
		{
			name: "compares strings exactly, whitespace included",
			current: map[string]string{
				"NSGlobalDomain KeyRepeat":              "2",
				"NSGlobalDomain AppleShowAllExtensions": "1",
				"com.apple.dock tilesize":               "48",
				"com.apple.screencapture type":          "png ",
			},
			wantKind:  model.ChangeUpdate,
			wantItems: []model.Item{{Name: "com.apple.screencapture type", From: "png ", To: "png"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			machine := &fakeDefaults{t: t, values: tt.current}

			result, err := planDefaults(t, machine, sampleDefaults)

			require.NoError(t, err)
			require.Len(t, result.Steps, 1)
			change := result.Steps[0].Change
			assert.Equal(t, tt.wantKind, change.Kind)
			assert.Equal(t, tt.wantItems, change.Items)
			assert.Equal(t, 4, change.Total)
			assert.False(t, change.NeedsSudo)
			assert.Empty(t, machine.writes)
		})
	}
}

func TestDarwinDefaults_Apply(t *testing.T) {
	t.Run("writes only drifted keys and converges", func(t *testing.T) {
		machine := &fakeDefaults{t: t, values: map[string]string{
			"NSGlobalDomain KeyRepeat":              "6",
			"NSGlobalDomain AppleShowAllExtensions": "1",
			"com.apple.dock tilesize":               "48",
		}}

		result, err := planDefaults(t, machine, sampleDefaults)
		require.NoError(t, err)
		apply(t, result)

		assert.Equal(t, [][]string{
			{"NSGlobalDomain", "KeyRepeat", "-int", "2"},
			{"com.apple.screencapture", "type", "-string", "png"},
		}, machine.writes)

		again, err := planDefaults(t, machine, sampleDefaults)
		require.NoError(t, err)
		assert.Equal(t, model.ChangeNone, again.Steps[0].Change.Kind)
	})

	t.Run("a string with significant whitespace converges after one write", func(t *testing.T) {
		file := `defaults:
  - {domain: com.apple.finder, key: Title, type: string, value: " padded "}
  - {domain: com.apple.finder, key: Suffix, type: string, value: "tail  "}
`
		machine := &fakeDefaults{t: t, values: map[string]string{
			"com.apple.finder Title": "padded",
		}}

		result, err := planDefaults(t, machine, file)
		require.NoError(t, err)
		assert.Equal(t, []model.Item{
			{Name: "com.apple.finder Title", From: "padded", To: " padded "},
			{Name: "com.apple.finder Suffix", To: "tail  "},
		}, result.Steps[0].Change.Items)
		apply(t, result)

		assert.Equal(t, [][]string{
			{"com.apple.finder", "Title", "-string", " padded "},
			{"com.apple.finder", "Suffix", "-string", "tail  "},
		}, machine.writes)

		again, err := planDefaults(t, machine, file)
		require.NoError(t, err)
		assert.Equal(t, model.ChangeNone, again.Steps[0].Change.Kind)
	})

	t.Run("reports a failed write with the key and the command output", func(t *testing.T) {
		machine := &fakeDefaults{t: t, values: map[string]string{}}
		result, err := planDefaults(t, machine, sampleDefaults)
		require.NoError(t, err)

		machine.writeErr = exitError{code: 1}
		err = applyErr(result)

		require.ErrorContains(t, err, "NSGlobalDomain KeyRepeat")
		assert.ErrorContains(t, err, "Could not write domain")
	})
}

func TestDarwinDefaults_ReadFailure(t *testing.T) {
	failures := []struct {
		name    string
		failure defaultsFailure
	}{
		{
			name:    "defaults cannot be started",
			failure: defaultsFailure{err: errors.New(`exec: "defaults": executable file not found in $PATH`)},
		},
		{
			name:    "defaults exits with another error",
			failure: defaultsFailure{output: "Could not read domain\n", err: exitError{code: 1}},
		},
		{
			name:    "defaults is killed",
			failure: defaultsFailure{output: "The domain/default pair of (com.apple.dock, tilesize) does not exist\n", err: exitError{code: -1}},
		},
	}

	for _, tt := range failures {
		t.Run(tt.name+" fails the plan", func(t *testing.T) {
			machine := &fakeDefaults{t: t, values: map[string]string{}, failures: map[string]defaultsFailure{
				"com.apple.dock tilesize": tt.failure,
			}}

			_, err := planDefaults(t, machine, sampleDefaults)

			require.ErrorContains(t, err, "com.apple.dock tilesize")
			assert.ErrorContains(t, err, tt.failure.err.Error())
			assert.Empty(t, machine.writes)
		})

		t.Run(tt.name+" during apply writes nothing further", func(t *testing.T) {
			machine := &fakeDefaults{t: t, values: map[string]string{}}
			result, err := planDefaults(t, machine, sampleDefaults)
			require.NoError(t, err)

			machine.failures = map[string]defaultsFailure{"NSGlobalDomain KeyRepeat": tt.failure}
			err = applyErr(result)

			require.ErrorContains(t, err, "NSGlobalDomain KeyRepeat")
			assert.Empty(t, machine.writes)
		})
	}
}

func TestDarwinDefaults_Compile(t *testing.T) {
	tests := []struct {
		name    string
		tasks   string
		file    string
		wantErr string
	}{
		{name: "args not a mapping", tasks: "tasks:\n  - action: set.darwin.defaults\n    args: [a]", wantErr: "args must be a mapping"},
		{name: "missing file", tasks: "tasks:\n  - action: set.darwin.defaults\n    args: {}", wantErr: "missing 'file'"},
		{name: "unreadable file", tasks: defaultsTask, wantErr: "macos.yaml"},
		{name: "bad yaml", tasks: defaultsTask, file: "defaults: [", wantErr: "parse"},
		{
			name:    "missing domain",
			tasks:   defaultsTask,
			file:    "defaults:\n  - {key: K, type: int, value: 1}",
			wantErr: "entry 1: missing 'domain'",
		},
		{
			name:    "missing key",
			tasks:   defaultsTask,
			file:    "defaults:\n  - {domain: D, type: int, value: 1}",
			wantErr: "entry 1: missing 'key'",
		},
		{
			name:    "missing value",
			tasks:   defaultsTask,
			file:    "defaults:\n  - {domain: D, key: K, type: int}",
			wantErr: "entry 1: missing 'value'",
		},
		{
			name:    "unknown type",
			tasks:   defaultsTask,
			file:    "defaults:\n  - {domain: D, key: K, type: date, value: 1}",
			wantErr: `entry 1: invalid type "date"`,
		},
		{
			name:    "value does not fit its type",
			tasks:   defaultsTask,
			file:    "defaults:\n  - {domain: D, key: K, type: int, value: 1}\n  - {domain: D, key: B, type: bool, value: maybe}",
			wantErr: "entry 2: invalid bool value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := compileDefaults(t, &fakeDefaults{t: t}, tt.file, tt.tasks)

			require.Error(t, err)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}
