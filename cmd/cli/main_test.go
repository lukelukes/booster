package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestConfig(t *testing.T, content string) (*CLI, *RunCmd) {
	t.Helper()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "bootstrap.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o644))

	cmd := &RunCmd{DryRun: true}
	cli := &CLI{Config: configPath}

	return cli, cmd
}

func setupTestConfigWithCmd(t *testing.T, content string, cmd RunCmd) (*CLI, *RunCmd) {
	t.Helper()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "bootstrap.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o644))

	cmdCopy := cmd
	cli := &CLI{Config: configPath}

	return cli, &cmdCopy
}

func TestRunCmd_CoreScenarios(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		configPathOnly bool
		errContain     string
	}{
		{
			name: "loads basic config",
			content: `version: "1"
tasks:
  - action: dir.create
    args:
      - ~/.config/test
`,
		},
		{
			name:           "config not found",
			configPathOnly: true,
			errContain:     "load config",
		},
		{
			name: "unknown action",
			content: `version: "1"
tasks:
  - action: unknown.action.type
    args: {}
`,
			errContain: "unknown action",
		},
		{
			name: "empty tasks succeeds",
			content: `version: "1"
tasks: []
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				cli *CLI
				cmd *RunCmd
			)

			if tt.configPathOnly {
				dir := t.TempDir()
				nonexistentPath := filepath.Join(dir, "does-not-exist.yaml")
				cmd = &RunCmd{DryRun: true}
				cli = &CLI{Config: nonexistentPath}
			} else {
				cli, cmd = setupTestConfig(t, tt.content)
			}

			err := cmd.Run(cli)
			if tt.errContain == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContain)
		})
	}
}

func TestRunCmd_InvalidConfig(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		errContain string
	}{
		{
			name: "invalid YAML syntax",
			content: `version: "1"
tasks:
  - action: [invalid yaml
`,
			errContain: "load config",
		},
		{
			name: "missing version",
			content: `tasks:
  - action: dir.create
    args: []
`,
			errContain: "load config",
		},
		{
			name: "unsupported version",
			content: `version: "2"
tasks: []
`,
			errContain: "load config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli, cmd := setupTestConfig(t, tt.content)

			err := cmd.Run(cli)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContain)
		})
	}
}

func TestRunCmd_BuildsTasks(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name: "single dir.create task",
			content: `version: "1"
tasks:
  - action: dir.create
    args:
      - ~/.config/app
`,
		},
		{
			name: "multiple tasks",
			content: `version: "1"
tasks:
  - action: dir.create
    args:
      - ~/.config/app1
  - action: dir.create
    args:
      - ~/.config/app2
`,
		},
		{
			name: "symlink task",
			content: `version: "1"
tasks:
  - action: symlink.create
    args:
      - source: ~/dotfiles/vimrc
        target: ~/.vimrc
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli, cmd := setupTestConfig(t, tt.content)

			err := cmd.Run(cli)

			require.NoError(t, err)
		})
	}
}

func TestRunCmd_InvalidTaskArgs(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		errContain string
	}{
		{
			name: "dir.create empty args returns no tasks",
			content: `version: "1"
tasks:
  - action: dir.create
    args: []
`,
			errContain: "",
		},
		{
			name: "symlink.create missing required fields",
			content: `version: "1"
tasks:
  - action: symlink.create
    args:
      - source: ~/source
`,
			errContain: "build tasks",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli, cmd := setupTestConfig(t, tt.content)

			err := cmd.Run(cli)

			if tt.errContain == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContain)
			}
		})
	}
}

func TestRunCmd_ConditionalTasks(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name: "task with single OS condition",
			content: `version: "1"
tasks:
  - action: dir.create
    when: ${ os == "arch" }
    args:
      - ~/.config/arch-only
`,
		},
		{
			name: "task with multiple OS conditions",
			content: `version: "1"
tasks:
  - action: dir.create
    when: ${ os in ["arch", "darwin"] }
    args:
      - ~/.config/multi-os
`,
		},
		{
			name: "mixed conditional and unconditional tasks",
			content: `version: "1"
tasks:
  - action: dir.create
    when: ${ os == "arch" }
    args:
      - ~/.config/arch-only
  - action: dir.create
    args:
      - ~/.config/everywhere
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli, cmd := setupTestConfig(t, tt.content)

			err := cmd.Run(cli)

			require.NoError(t, err)
		})
	}
}

func TestRunCmd_SupportedActionTasks(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name: "template.render",
			content: `version: "1"
tasks:
  - action: template.render
    args:
      - source: ~/templates/config.tmpl
        target: ~/.config/app/config
`,
		},
		{
			name: "pkg.install",
			content: `version: "1"
tasks:
  - action: pkg.install
    args:
      - vim
      - git
`,
		},
		{
			name: "pkg-manager.install",
			content: `version: "1"
tasks:
  - action: pkg-manager.install
    args:
      - yay
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli, cmd := setupTestConfig(t, tt.content)
			err := cmd.Run(cli)
			require.NoError(t, err)
		})
	}
}

func TestVersionCmd(t *testing.T) {
	originalVersion := Version
	t.Cleanup(func() { Version = originalVersion })

	Version = "test-version"
	cmd := &VersionCmd{}
	cli := &CLI{}

	err := cmd.Run(cli)

	require.NoError(t, err)
}

func TestRunCmd_ComplexConfig(t *testing.T) {
	content := `version: "1"
tasks:
  - action: dir.create
    args:
      - ~/.config/nvim
      - ~/.local/share/nvim
  - action: symlink.create
    args:
      - source: ~/dotfiles/nvim/init.lua
        target: ~/.config/nvim/init.lua
  - action: template.render
    args:
      - source: ~/templates/gitconfig.tmpl
        target: ~/.gitconfig
  - action: pkg.install
    args:
      - neovim
      - git
      - ripgrep
  - action: dir.create
    when: ${ os == "arch" }
    args:
      - ~/.config/i3
`
	cli, cmd := setupTestConfig(t, content)

	err := cmd.Run(cli)

	require.NoError(t, err)
}

func TestRunCmd_ProfileAndCompatibilityScenarios(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		cmd        RunCmd
		errContain string
	}{
		{
			name: "ignores unknown fields for compatibility",
			content: `version: "1"
tasks:
  - action: pkg-manager.install
    prompt_for_sudo: true
    args:
      - yay
`,
			cmd: RunCmd{DryRun: true},
		},
		{
			name: "profile works when valid",
			content: `version: "1"
profiles:
  - personal
  - work
tasks:
  - action: dir.create
    args:
      - ~/.config/test
`,
			cmd: RunCmd{DryRun: true, Profile: "personal"},
		},
		{
			name: "invalid profile returns error",
			content: `version: "1"
profiles:
  - personal
  - work
tasks: []
`,
			cmd:        RunCmd{DryRun: true, Profile: "invalid"},
			errContain: "invalid profile",
		},
		{
			name: "missing profile when profiles defined returns error",
			content: `version: "1"
profiles:
  - personal
  - work
tasks: []
`,
			cmd:        RunCmd{DryRun: true},
			errContain: "--profile",
		},
		{
			name: "profile not required without profiles",
			content: `version: "1"
tasks:
  - action: dir.create
    args:
      - ~/.config/test
`,
			cmd: RunCmd{DryRun: true},
		},
		{
			name: "profile specified but no profiles in config returns error",
			content: `version: "1"
tasks: []
`,
			cmd:        RunCmd{DryRun: true, Profile: "personal"},
			errContain: "no profiles defined",
		},
		{
			name: "profile condition filters are accepted",
			content: `version: "1"
profiles:
  - personal
  - work
tasks:
  - action: dir.create
    when: ${ profile == "personal" }
    args:
      - ~/.config/personal-only
  - action: dir.create
    when: ${ profile == "work" }
    args:
      - ~/.config/work-only
  - action: dir.create
    args:
      - ~/.config/always
`,
			cmd: RunCmd{DryRun: true, Profile: "personal"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli, cmd := setupTestConfigWithCmd(t, tt.content, tt.cmd)
			err := cmd.Run(cli)

			if tt.errContain == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContain)
		})
	}
}

func TestRunCmd_ConfigWithVariablesLoads(t *testing.T) {
	t.Skip("Variable resolution requires TUI interaction - tested manually")

	content := `version: "1"
variables:
  Name:
    prompt: "Your full name"
  Email:
    prompt: "Your email address"
    default: "user@example.com"
tasks:
  - action: dir.create
    args:
      - ~/.config/app
`
	cli, cmd := setupTestConfig(t, content)

	err := cmd.Run(cli)

	require.NoError(t, err)
}
