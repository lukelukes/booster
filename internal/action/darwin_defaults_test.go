package action

import (
	"booster/internal/cmdexec"
	"booster/internal/expr"
	"booster/internal/model"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type fakeDefaults struct {
	values map[string]string
	writes [][]string
}

func (f *fakeDefaults) runner() *cmdexec.MockRunner {
	return &cmdexec.MockRunner{
		RunFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
			if name != "defaults" {
				return nil, errors.New("unexpected command " + name)
			}
			key := args[1] + " " + args[2]
			switch args[0] {
			case "read":
				value, ok := f.values[key]
				if !ok {
					return []byte("The domain/default pair does not exist\n"), errors.New("exit status 1")
				}
				return []byte(value + "\n"), nil
			case "write":
				f.writes = append(f.writes, args[1:])
				f.values[key] = args[4]
				return nil, nil
			}
			return nil, errors.New("unexpected defaults verb " + args[0])
		},
	}
}

func writeDefaultsFile(t *testing.T, dir, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "macos.yaml"), []byte(content), 0o644))
}

func compileDarwinDefaults(t *testing.T, dir string, runner cmdexec.Runner) (Action, error) {
	t.Helper()
	var args yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte("file: macos.yaml"), &args))
	ctx := &expr.Context{OS: "darwin", ConfigDir: dir}
	return newDarwinDefaults(*args.Content[0], ctx, runner)
}

const sampleDefaults = `defaults:
  - {domain: NSGlobalDomain, key: KeyRepeat, type: int, value: 2}
  - {domain: NSGlobalDomain, key: AppleShowAllExtensions, type: bool, value: true}
  - {domain: com.apple.dock, key: tilesize, type: float, value: 48}
  - {domain: com.apple.screencapture, key: type, type: string, value: png}
`

func TestDarwinDefaults_Check(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDefaultsFile(t, dir, sampleDefaults)
			fake := &fakeDefaults{values: tt.current}
			act, err := compileDarwinDefaults(t, dir, fake.runner())
			require.NoError(t, err)

			change, err := act.Check(context.Background())

			require.NoError(t, err)
			assert.Equal(t, tt.wantKind, change.Kind)
			assert.Equal(t, tt.wantItems, change.Items)
			assert.Equal(t, 4, change.Total)
			assert.Empty(t, fake.writes)
		})
	}
}

func TestDarwinDefaults_Apply(t *testing.T) {
	dir := t.TempDir()
	writeDefaultsFile(t, dir, sampleDefaults)
	fake := &fakeDefaults{values: map[string]string{
		"NSGlobalDomain KeyRepeat":              "6",
		"NSGlobalDomain AppleShowAllExtensions": "1",
		"com.apple.dock tilesize":               "48",
	}}
	act, err := compileDarwinDefaults(t, dir, fake.runner())
	require.NoError(t, err)

	require.NoError(t, act.Apply(context.Background()))

	assert.Equal(t, [][]string{
		{"NSGlobalDomain", "KeyRepeat", "-int", "2"},
		{"com.apple.screencapture", "type", "-string", "png"},
	}, fake.writes)

	change, err := act.Check(context.Background())
	require.NoError(t, err)
	assert.Equal(t, model.ChangeNone, change.Kind)
}

func TestDarwinDefaults_ApplyReportsWriteFailure(t *testing.T) {
	dir := t.TempDir()
	writeDefaultsFile(t, dir, sampleDefaults)
	runner := &cmdexec.MockRunner{
		RunFunc: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			if args[0] == "write" {
				return []byte("Could not write domain\n"), errors.New("exit status 1")
			}
			return nil, errors.New("missing")
		},
	}
	act, err := compileDarwinDefaults(t, dir, runner)
	require.NoError(t, err)

	err = act.Apply(context.Background())

	require.ErrorContains(t, err, "NSGlobalDomain KeyRepeat")
	assert.ErrorContains(t, err, "Could not write domain")
}

func TestNewDarwinDefaults(t *testing.T) {
	t.Run("resolves the file relative to the config dir", func(t *testing.T) {
		dir := t.TempDir()
		writeDefaultsFile(t, dir, sampleDefaults)

		act, err := compileDarwinDefaults(t, dir, &cmdexec.MockRunner{})

		require.NoError(t, err)
		assert.Equal(t, "set.darwin.defaults", act.Name())
	})

	t.Run("is not applicable off macOS, even without a defaults file", func(t *testing.T) {
		var args yaml.Node
		require.NoError(t, yaml.Unmarshal([]byte("file: missing.yaml"), &args))

		_, err := NewDarwinDefaults(*args.Content[0], &expr.Context{OS: "linux", ConfigDir: t.TempDir()})

		require.ErrorIs(t, err, ErrNotApplicable)
	})

	tests := []struct {
		name    string
		args    string
		file    string
		wantErr string
	}{
		{name: "args not a mapping", args: "[a]", wantErr: "args must be a mapping"},
		{name: "missing file", args: "{}", wantErr: "missing 'file'"},
		{name: "unreadable file", args: "file: nope.yaml", wantErr: "nope.yaml"},
		{name: "bad yaml", args: "file: macos.yaml", file: "defaults: [", wantErr: "parse"},
		{
			name:    "missing domain",
			args:    "file: macos.yaml",
			file:    "defaults:\n  - {key: K, type: int, value: 1}",
			wantErr: "entry 1: missing 'domain'",
		},
		{
			name:    "missing key",
			args:    "file: macos.yaml",
			file:    "defaults:\n  - {domain: D, type: int, value: 1}",
			wantErr: "entry 1: missing 'key'",
		},
		{
			name:    "missing value",
			args:    "file: macos.yaml",
			file:    "defaults:\n  - {domain: D, key: K, type: int}",
			wantErr: "entry 1: missing 'value'",
		},
		{
			name:    "unknown type",
			args:    "file: macos.yaml",
			file:    "defaults:\n  - {domain: D, key: K, type: date, value: 1}",
			wantErr: `entry 1: invalid type "date"`,
		},
		{
			name:    "value does not fit its type",
			args:    "file: macos.yaml",
			file:    "defaults:\n  - {domain: D, key: K, type: int, value: 1}\n  - {domain: D, key: B, type: bool, value: maybe}",
			wantErr: "entry 2: invalid bool value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.file != "" {
				writeDefaultsFile(t, dir, tt.file)
			}
			var args yaml.Node
			require.NoError(t, yaml.Unmarshal([]byte(tt.args), &args))

			_, err := newDarwinDefaults(*args.Content[0], &expr.Context{OS: "darwin", ConfigDir: dir}, &cmdexec.MockRunner{})

			require.Error(t, err)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}
