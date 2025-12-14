package task

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateRender_RendersTemplate(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "config.tmpl")
	target := filepath.Join(dir, "config")

	// Create template file
	require.NoError(t, os.WriteFile(source, []byte("Hello {{.Vars.Name}}!"), 0o644))

	task := &TemplateRender{
		Source: source,
		Target: target,
		Context: TemplateContext{
			Vars: map[string]string{"Name": "World"},
		},
	}
	result := task.Run(context.Background())

	assert.Equal(t, StatusDone, result.Status)
	assert.Equal(t, "rendered", result.Message)

	// Verify output
	content, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "Hello World!", string(content))
}

func TestTemplateRender_SkipsWhenUpToDate(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "config.tmpl")
	target := filepath.Join(dir, "config")

	// Create template and pre-existing output
	require.NoError(t, os.WriteFile(source, []byte("Hello {{.Vars.Name}}!"), 0o644))
	require.NoError(t, os.WriteFile(target, []byte("Hello World!"), 0o644))

	task := &TemplateRender{
		Source: source,
		Target: target,
		Context: TemplateContext{
			Vars: map[string]string{"Name": "World"},
		},
	}
	result := task.Run(context.Background())

	assert.Equal(t, StatusSkipped, result.Status)
	assert.Equal(t, "already up to date", result.Message)
}

func TestTemplateRender_UpdatesWhenChanged(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "config.tmpl")
	target := filepath.Join(dir, "config")

	// Create template and pre-existing output with OLD content
	require.NoError(t, os.WriteFile(source, []byte("Hello {{.Vars.Name}}!"), 0o644))
	require.NoError(t, os.WriteFile(target, []byte("Hello OldValue!"), 0o644))

	task := &TemplateRender{
		Source: source,
		Target: target,
		Context: TemplateContext{
			Vars: map[string]string{"Name": "NewValue"},
		},
	}
	result := task.Run(context.Background())

	assert.Equal(t, StatusDone, result.Status)

	// Verify updated content
	content, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "Hello NewValue!", string(content))
}

func TestTemplateRender_CreatesParentDirectories(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "config.tmpl")
	target := filepath.Join(dir, "nested", "deep", "config")

	require.NoError(t, os.WriteFile(source, []byte("content: {{.Vars.Value}}"), 0o644))

	task := &TemplateRender{
		Source: source,
		Target: target,
		Context: TemplateContext{
			Vars: map[string]string{"Value": "test"},
		},
	}
	result := task.Run(context.Background())

	assert.Equal(t, StatusDone, result.Status)

	// Verify file was created in nested directory
	content, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "content: test", string(content))
}

func TestTemplateRender_FailsOnInvalidTemplate(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "config.tmpl")
	target := filepath.Join(dir, "config")

	// Invalid template syntax
	require.NoError(t, os.WriteFile(source, []byte("{{.Vars.Name"), 0o644))

	task := &TemplateRender{
		Source: source,
		Target: target,
		Context: TemplateContext{
			Vars: map[string]string{"Name": "World"},
		},
	}
	result := task.Run(context.Background())

	assert.Equal(t, StatusFailed, result.Status)
	assert.Error(t, result.Error)
}

func TestTemplateRender_FailsOnMissingSource(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "nonexistent.tmpl")
	target := filepath.Join(dir, "config")

	task := &TemplateRender{
		Source:  source,
		Target:  target,
		Context: TemplateContext{},
	}
	result := task.Run(context.Background())

	assert.Equal(t, StatusFailed, result.Status)
	assert.Error(t, result.Error)
}

func TestTemplateRender_Idempotency(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "config.tmpl")
	target := filepath.Join(dir, "config")

	require.NoError(t, os.WriteFile(source, []byte("value: {{.Vars.X}}"), 0o644))

	task := &TemplateRender{
		Source: source,
		Target: target,
		Context: TemplateContext{
			Vars: map[string]string{"X": "42"},
		},
	}

	// First run: creates
	result1 := task.Run(context.Background())
	assert.Equal(t, StatusDone, result1.Status)

	// Second run: skips
	result2 := task.Run(context.Background())
	assert.Equal(t, StatusSkipped, result2.Status)

	// Third run: still skips
	result3 := task.Run(context.Background())
	assert.Equal(t, StatusSkipped, result3.Status)
}

func TestTemplateRender_Name(t *testing.T) {
	task := &TemplateRender{
		Source:  "~/dotfiles/gitconfig.tmpl",
		Target:  "~/.gitconfig",
		Context: TemplateContext{},
	}

	name := task.Name()
	assert.Contains(t, name, "gitconfig.tmpl")
	assert.Contains(t, name, ".gitconfig")
}

func TestTemplateRender_MultipleVariables(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "config.tmpl")
	target := filepath.Join(dir, "config")

	tmpl := `[user]
    name = {{.Vars.Name}}
    email = {{.Vars.Email}}
[core]
    editor = {{.Vars.Editor}}`
	require.NoError(t, os.WriteFile(source, []byte(tmpl), 0o644))

	task := &TemplateRender{
		Source: source,
		Target: target,
		Context: TemplateContext{
			Vars: map[string]string{
				"Name":   "Alice",
				"Email":  "alice@example.com",
				"Editor": "vim",
			},
		},
	}
	result := task.Run(context.Background())

	assert.Equal(t, StatusDone, result.Status)

	content, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Contains(t, string(content), "name = Alice")
	assert.Contains(t, string(content), "email = alice@example.com")
	assert.Contains(t, string(content), "editor = vim")
}

func TestTemplateRender_MissingVariable(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "config.tmpl")
	target := filepath.Join(dir, "config")

	// Template uses undefined variable
	require.NoError(t, os.WriteFile(source, []byte("Hello {{.Vars.Undefined}}!"), 0o644))

	task := &TemplateRender{
		Source:  source,
		Target:  target,
		Context: TemplateContext{Vars: map[string]string{}}, // No variables provided
	}
	result := task.Run(context.Background())

	// Go templates render missing map keys as "<no value>" by default
	// This helps users see when a variable is missing
	assert.Equal(t, StatusDone, result.Status)

	content, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "Hello <no value>!", string(content))
}

func TestNewTemplateRenderFactory_ValidArgs(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "src.tmpl")
	target := filepath.Join(dir, "dst")

	// Create a template that uses all context fields to verify they're passed correctly
	tmplContent := "Name={{.Vars.Name}} OS={{.System.OS}} Profile={{.System.Profile}}"
	require.NoError(t, os.WriteFile(source, []byte(tmplContent), 0o644))

	cfg := TemplateRenderConfig{
		Vars:    map[string]string{"Name": "Test"},
		OS:      "arch",
		Profile: "work",
	}
	factory := NewTemplateRenderFactory(cfg)

	args := []any{
		map[string]any{
			"source": source,
			"target": target,
		},
	}

	tasks, err := factory(args)
	require.NoError(t, err)
	require.Len(t, tasks, 1)

	// Test through Name() - verifies source/target were parsed correctly
	name := tasks[0].Name()
	assert.Contains(t, name, "src.tmpl")
	assert.Contains(t, name, "dst")

	// Test through Run() - verifies context (Vars, OS, Profile) is correctly applied
	result := tasks[0].Run(context.Background())
	require.Equal(t, StatusDone, result.Status)

	content, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "Name=Test OS=arch Profile=work", string(content))
}

func TestNewTemplateRenderFactory_MultipleTemplates(t *testing.T) {
	cfg := TemplateRenderConfig{Vars: map[string]string{}}
	factory := NewTemplateRenderFactory(cfg)

	args := []any{
		map[string]any{"source": "a.tmpl", "target": "a"},
		map[string]any{"source": "b.tmpl", "target": "b"},
	}

	tasks, err := factory(args)
	require.NoError(t, err)
	assert.Len(t, tasks, 2)
}

func TestNewTemplateRenderFactory_InvalidArgs(t *testing.T) {
	cfg := TemplateRenderConfig{Vars: map[string]string{}}
	factory := NewTemplateRenderFactory(cfg)

	tests := []struct {
		name    string
		args    any
		wantErr string
	}{
		{
			name:    "not a list",
			args:    "invalid",
			wantErr: "must be a list",
		},
		{
			name:    "item not a map",
			args:    []any{"invalid"},
			wantErr: "must be a map",
		},
		{
			name:    "missing source",
			args:    []any{map[string]any{"target": "dst"}},
			wantErr: "missing 'source'",
		},
		{
			name:    "missing target",
			args:    []any{map[string]any{"source": "src"}},
			wantErr: "missing 'target'",
		},
		{
			name:    "source not string",
			args:    []any{map[string]any{"source": 123, "target": "dst"}},
			wantErr: "'source' must be a string",
		},
		{
			name:    "target not string",
			args:    []any{map[string]any{"source": "src", "target": 123}},
			wantErr: "'target' must be a string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := factory(tt.args)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestNewTemplateRenderFactory_ErrorIndices(t *testing.T) {
	// Tests ARITHMETIC_BASE mutations: i+1 vs i-1 in error messages
	cfg := TemplateRenderConfig{Vars: map[string]string{}}
	factory := NewTemplateRenderFactory(cfg)

	tests := []struct {
		name          string
		args          []any
		expectedIndex string
	}{
		{
			name: "first arg not a map shows arg 1",
			args: []any{
				"not a map",
			},
			expectedIndex: "arg 1:",
		},
		{
			name: "second arg missing source shows arg 2",
			args: []any{
				map[string]any{"source": "a.tmpl", "target": "a"},
				map[string]any{"target": "b"}, // missing source
			},
			expectedIndex: "arg 2:",
		},
		{
			name: "third arg source not string shows arg 3",
			args: []any{
				map[string]any{"source": "a.tmpl", "target": "a"},
				map[string]any{"source": "b.tmpl", "target": "b"},
				map[string]any{"source": 123, "target": "c"},
			},
			expectedIndex: "arg 3:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := factory(tt.args)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedIndex,
				"error message must show correct 1-indexed position")
		})
	}
}

func TestTemplateRender_SystemOS(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "config.tmpl")
	target := filepath.Join(dir, "config")

	// Template using System.OS
	require.NoError(t, os.WriteFile(source, []byte("OS: {{.System.OS}}"), 0o644))

	task := &TemplateRender{
		Source: source,
		Target: target,
		Context: TemplateContext{
			System: TemplateSystem{OS: "arch"},
		},
	}
	result := task.Run(context.Background())

	assert.Equal(t, StatusDone, result.Status)

	content, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "OS: arch", string(content))
}

func TestTemplateRender_SystemProfile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "config.tmpl")
	target := filepath.Join(dir, "config")

	// Template using System.Profile
	require.NoError(t, os.WriteFile(source, []byte("Profile: {{.System.Profile}}"), 0o644))

	task := &TemplateRender{
		Source: source,
		Target: target,
		Context: TemplateContext{
			System: TemplateSystem{Profile: "work"},
		},
	}
	result := task.Run(context.Background())

	assert.Equal(t, StatusDone, result.Status)

	content, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "Profile: work", string(content))
}

func TestTemplateRender_CombinesVarsAndSystem(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "config.tmpl")
	target := filepath.Join(dir, "config")

	// Template using both Vars and System
	tmpl := `# Generated for {{.System.OS}} ({{.System.Profile}})
user = {{.Vars.Username}}
email = {{.Vars.Email}}`
	require.NoError(t, os.WriteFile(source, []byte(tmpl), 0o644))

	task := &TemplateRender{
		Source: source,
		Target: target,
		Context: TemplateContext{
			Vars: map[string]string{
				"Username": "alice",
				"Email":    "alice@example.com",
			},
			System: TemplateSystem{
				OS:      "darwin",
				Profile: "personal",
			},
		},
	}
	result := task.Run(context.Background())

	assert.Equal(t, StatusDone, result.Status)

	content, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Contains(t, string(content), "# Generated for darwin (personal)")
	assert.Contains(t, string(content), "user = alice")
	assert.Contains(t, string(content), "email = alice@example.com")
}
