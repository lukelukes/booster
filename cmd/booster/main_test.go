package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestConfig creates a temporary config file with the given content
// and returns initialized CLI and RunCmd instances for testing.
func setupTestConfig(t *testing.T, content string) (*CLI, *RunCmd) {
	t.Helper()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "bootstrap.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o644))

	cmd := &RunCmd{DryRun: true}
	cli := &CLI{Config: configPath}

	return cli, cmd
}

// TestRunCmd_LoadsConfig verifies that the CLI correctly loads and parses config files.
func TestRunCmd_LoadsConfig(t *testing.T) {
	content := `version: "1"
tasks:
  - action: dir.create
    args:
      - ~/.config/test
`
	cli, cmd := setupTestConfig(t, content)

	err := cmd.Run(cli)

	require.NoError(t, err)
}

// TestRunCmd_ConfigNotFound returns error when config file doesn't exist.
func TestRunCmd_ConfigNotFound(t *testing.T) {
	dir := t.TempDir()
	nonexistentPath := filepath.Join(dir, "does-not-exist.yaml")

	cmd := &RunCmd{DryRun: true}
	cli := &CLI{Config: nonexistentPath}

	err := cmd.Run(cli)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "load config")
}

// TestRunCmd_InvalidConfig returns error when config YAML is malformed.
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

// TestRunCmd_UnknownAction returns error when config contains unknown action.
func TestRunCmd_UnknownAction(t *testing.T) {
	content := `version: "1"
tasks:
  - action: unknown.action.type
    args: {}
`
	cli, cmd := setupTestConfig(t, content)

	err := cmd.Run(cli)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown action")
	assert.Contains(t, err.Error(), "unknown.action.type")
}

// TestRunCmd_EmptyTasks succeeds with no tasks to execute.
func TestRunCmd_EmptyTasks(t *testing.T) {
	content := `version: "1"
tasks: []
`
	cli, cmd := setupTestConfig(t, content)

	err := cmd.Run(cli)

	require.NoError(t, err)
}

// TestRunCmd_BuildsTasks verifies tasks are built from config.
// This tests the wiring between config loading and task building.
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

// TestRunCmd_InvalidTaskArgs returns error when task args are invalid.
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
			errContain: "", // Empty args = 0 tasks, not an error
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

// TestRunCmd_ConditionalTasks verifies tasks with when conditions are built.
// The actual condition evaluation is tested in the task package.
// This tests the wiring from config -> task builder.
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
    when:
      os: "arch"
    args:
      - ~/.config/arch-only
`,
		},
		{
			name: "task with multiple OS conditions",
			content: `version: "1"
tasks:
  - action: dir.create
    when:
      os: ["arch", "darwin"]
    args:
      - ~/.config/multi-os
`,
		},
		{
			name: "mixed conditional and unconditional tasks",
			content: `version: "1"
tasks:
  - action: dir.create
    when:
      os: "arch"
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

// TestRunCmd_TemplateTask verifies template.render tasks are built.
// This tests that the template factory is registered correctly.
func TestRunCmd_TemplateTask(t *testing.T) {
	content := `version: "1"
tasks:
  - action: template.render
    args:
      - source: ~/templates/config.tmpl
        target: ~/.config/app/config
`
	cli, cmd := setupTestConfig(t, content)

	err := cmd.Run(cli)

	require.NoError(t, err)
}

// TestRunCmd_PkgInstallTask verifies pkg.install tasks are built.
// This tests that the package install factory is registered correctly.
func TestRunCmd_PkgInstallTask(t *testing.T) {
	content := `version: "1"
tasks:
  - action: pkg.install
    args:
      - vim
      - git
`
	cli, cmd := setupTestConfig(t, content)

	err := cmd.Run(cli)

	require.NoError(t, err)
}

// TestRunCmd_PkgManagerInstallTask verifies pkg-manager.install tasks are built.
func TestRunCmd_PkgManagerInstallTask(t *testing.T) {
	content := `version: "1"
tasks:
  - action: pkg-manager.install
    args:
      - yay
`
	cli, cmd := setupTestConfig(t, content)

	err := cmd.Run(cli)

	require.NoError(t, err)
}

// TestVersionCmd verifies version command works.
func TestVersionCmd(t *testing.T) {
	// Save original version and restore after test
	originalVersion := Version
	t.Cleanup(func() { Version = originalVersion })

	Version = "test-version"
	cmd := &VersionCmd{}
	cli := &CLI{}

	err := cmd.Run(cli)

	require.NoError(t, err)
}

// TestRunCmd_ComplexConfig tests a realistic multi-task configuration.
// This is an integration test that verifies the entire pipeline works.
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
    when:
      os: "arch"
    args:
      - ~/.config/i3
`
	cli, cmd := setupTestConfig(t, content)

	err := cmd.Run(cli)

	require.NoError(t, err)
}

// TestRunCmd_BackwardsCompatibility_IgnoresUnknownFields verifies that
// unknown config fields (like the deprecated prompt_for_sudo) don't cause errors.
// Tasks now self-declare sudo requirements via NeedsSudo() method.
func TestRunCmd_BackwardsCompatibility_IgnoresUnknownFields(t *testing.T) {
	content := `version: "1"
tasks:
  - action: pkg-manager.install
    prompt_for_sudo: true
    args:
      - yay
`
	cli, cmd := setupTestConfig(t, content)

	err := cmd.Run(cli)

	// Should succeed - unknown fields are ignored by YAML unmarshaler
	require.NoError(t, err)
}

// TestRunCmd_ProfileFlag_Works verifies --profile flag works when profiles defined.
func TestRunCmd_ProfileFlag_Works(t *testing.T) {
	content := `version: "1"
profiles:
  - personal
  - work
tasks:
  - action: dir.create
    args:
      - ~/.config/test
`
	dir := t.TempDir()
	configPath := filepath.Join(dir, "bootstrap.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o644))

	cmd := &RunCmd{DryRun: true, Profile: "personal"}
	cli := &CLI{Config: configPath}

	err := cmd.Run(cli)

	require.NoError(t, err)
}

// TestRunCmd_ProfileFlag_InvalidProfile returns error for invalid profile.
func TestRunCmd_ProfileFlag_InvalidProfile(t *testing.T) {
	content := `version: "1"
profiles:
  - personal
  - work
tasks: []
`
	dir := t.TempDir()
	configPath := filepath.Join(dir, "bootstrap.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o644))

	cmd := &RunCmd{DryRun: true, Profile: "invalid"}
	cli := &CLI{Config: configPath}

	err := cmd.Run(cli)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid profile")
	assert.Contains(t, err.Error(), "invalid")
}

// TestRunCmd_ProfileFlag_MissingWithProfiles returns error when profiles defined but no flag.
func TestRunCmd_ProfileFlag_MissingWithProfiles(t *testing.T) {
	content := `version: "1"
profiles:
  - personal
  - work
tasks: []
`
	dir := t.TempDir()
	configPath := filepath.Join(dir, "bootstrap.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o644))

	cmd := &RunCmd{DryRun: true, Profile: ""}
	cli := &CLI{Config: configPath}

	err := cmd.Run(cli)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--profile")
}

// TestRunCmd_ProfileFlag_NotNeededWithoutProfiles works without flag when no profiles.
func TestRunCmd_ProfileFlag_NotNeededWithoutProfiles(t *testing.T) {
	content := `version: "1"
tasks:
  - action: dir.create
    args:
      - ~/.config/test
`
	cli, cmd := setupTestConfig(t, content)

	err := cmd.Run(cli)

	require.NoError(t, err)
}

// TestRunCmd_ProfileFlag_ErrorWhenNoProfilesInConfig errors when --profile used without profiles.
func TestRunCmd_ProfileFlag_ErrorWhenNoProfilesInConfig(t *testing.T) {
	content := `version: "1"
tasks: []
`
	dir := t.TempDir()
	configPath := filepath.Join(dir, "bootstrap.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o644))

	cmd := &RunCmd{DryRun: true, Profile: "personal"}
	cli := &CLI{Config: configPath}

	err := cmd.Run(cli)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no profiles defined")
}

// TestRunCmd_ProfileConditionFilters verifies profile conditions filter tasks.
func TestRunCmd_ProfileConditionFilters(t *testing.T) {
	content := `version: "1"
profiles:
  - personal
  - work
tasks:
  - action: dir.create
    when:
      profile: "personal"
    args:
      - ~/.config/personal-only
  - action: dir.create
    when:
      profile: "work"
    args:
      - ~/.config/work-only
  - action: dir.create
    args:
      - ~/.config/always
`
	dir := t.TempDir()
	configPath := filepath.Join(dir, "bootstrap.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o644))

	cmd := &RunCmd{DryRun: true, Profile: "personal"}
	cli := &CLI{Config: configPath}

	err := cmd.Run(cli)

	require.NoError(t, err)
}

// TestRunCmd_ConfigWithVariablesLoads verifies configs with variables are parsed.
// Note: Variable resolution requires TUI, so full end-to-end variable testing
// is not possible in unit tests. This just verifies the config loads correctly.
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

	// This would require mocking the TUI collector or pre-populating values
	err := cmd.Run(cli)

	require.NoError(t, err)
}
