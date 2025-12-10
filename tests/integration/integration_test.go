// Package integration contains end-to-end tests that execute real tasks.
// These tests verify the full pipeline: config -> tasks -> execution -> side effects.
package integration

import (
	"booster/internal/condition"
	"booster/internal/config"
	"booster/internal/executor"
	"booster/internal/task"
	"booster/internal/tui"
	"booster/internal/variable"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDirCreate_ExecutesForReal verifies dir.create actually creates directories.
// This is the P0 integration test that proves the full pipeline works.
func TestDirCreate_ExecutesForReal(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "created-by-booster", "nested", "path")
	configPath := filepath.Join(dir, "bootstrap.yaml")

	content := fmt.Sprintf(`version: "1"
tasks:
  - action: dir.create
    args:
      - %s
`, targetDir)
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o644))

	// Load config
	cfg, err := config.Load(configPath)
	require.NoError(t, err)

	// Build tasks
	condCtx := condition.Context{OS: "linux"}
	builder := task.DefaultBuilder(condCtx)
	tasks, err := builder.Build(cfg.Tasks)
	require.NoError(t, err)
	require.Len(t, tasks, 1)

	// Execute (no dry-run, no TUI - direct execution)
	exec := executor.New(tasks)
	result, ok := exec.RunNext(context.Background())

	// Verify task succeeded
	require.True(t, ok, "should have a task to run")
	assert.Equal(t, task.StatusDone, result.Status)
	assert.NoError(t, result.Error)

	// Verify the directory was actually created
	info, err := os.Stat(targetDir)
	require.NoError(t, err, "directory should exist after task execution")
	assert.True(t, info.IsDir(), "should be a directory")
}

// TestDirCreate_Idempotent verifies running dir.create twice is safe.
func TestDirCreate_Idempotent(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "idempotent-test")

	// Create config
	configPath := filepath.Join(dir, "bootstrap.yaml")
	content := fmt.Sprintf(`version: "1"
tasks:
  - action: dir.create
    args:
      - %s
`, targetDir)
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o644))

	cfg, err := config.Load(configPath)
	require.NoError(t, err)

	// First run - creates directory
	condCtx := condition.Context{OS: "linux"}
	builder1 := task.DefaultBuilder(condCtx)
	tasks1, err := builder1.Build(cfg.Tasks)
	require.NoError(t, err)

	exec1 := executor.New(tasks1)
	result1, ok1 := exec1.RunNext(context.Background())
	require.True(t, ok1)
	assert.Equal(t, task.StatusDone, result1.Status)

	// Second run - should skip (directory exists)
	builder2 := task.DefaultBuilder(condCtx)
	tasks2, err := builder2.Build(cfg.Tasks)
	require.NoError(t, err)

	exec2 := executor.New(tasks2)
	result2, ok2 := exec2.RunNext(context.Background())
	require.True(t, ok2)
	assert.Equal(t, task.StatusSkipped, result2.Status)
}

// TestSymlinkCreate_ExecutesForReal verifies symlink.create creates actual symlinks.
func TestSymlinkCreate_ExecutesForReal(t *testing.T) {
	dir := t.TempDir()

	// Create source file
	sourceFile := filepath.Join(dir, "source.txt")
	require.NoError(t, os.WriteFile(sourceFile, []byte("test content"), 0o644))

	// Target symlink location
	targetLink := filepath.Join(dir, "link.txt")

	configPath := filepath.Join(dir, "bootstrap.yaml")
	content := fmt.Sprintf(`version: "1"
tasks:
  - action: symlink.create
    args:
      - source: %s
        target: %s
`, sourceFile, targetLink)
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o644))

	cfg, err := config.Load(configPath)
	require.NoError(t, err)

	condCtx := condition.Context{OS: "linux"}
	builder := task.DefaultBuilder(condCtx)
	tasks, err := builder.Build(cfg.Tasks)
	require.NoError(t, err)
	require.Len(t, tasks, 1)

	exec := executor.New(tasks)
	result, ok := exec.RunNext(context.Background())

	require.True(t, ok)
	assert.Equal(t, task.StatusDone, result.Status)
	assert.NoError(t, result.Error)

	// Verify symlink exists and points to source
	linkTarget, err := os.Readlink(targetLink)
	require.NoError(t, err, "symlink should exist")
	assert.Equal(t, sourceFile, linkTarget)

	// Verify content is accessible through symlink
	content2, err := os.ReadFile(targetLink)
	require.NoError(t, err)
	assert.Equal(t, "test content", string(content2))
}

// TestMultipleTasksExecution verifies multiple tasks execute in sequence.
func TestMultipleTasksExecution(t *testing.T) {
	dir := t.TempDir()

	dir1 := filepath.Join(dir, "first")
	dir2 := filepath.Join(dir, "second")
	dir3 := filepath.Join(dir, "third")

	configPath := filepath.Join(dir, "bootstrap.yaml")
	content := fmt.Sprintf(`version: "1"
tasks:
  - action: dir.create
    args:
      - %s
      - %s
      - %s
`, dir1, dir2, dir3)
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o644))

	cfg, err := config.Load(configPath)
	require.NoError(t, err)

	condCtx := condition.Context{OS: "linux"}
	builder := task.DefaultBuilder(condCtx)
	tasks, err := builder.Build(cfg.Tasks)
	require.NoError(t, err)
	require.Len(t, tasks, 3, "should create 3 tasks for 3 directories")

	// Execute all tasks
	exec := executor.New(tasks)
	ctx := context.Background()

	for i := range 3 {
		result, ok := exec.RunNext(ctx)
		require.True(t, ok, "task %d should exist", i+1)
		assert.Equal(t, task.StatusDone, result.Status, "task %d should succeed", i+1)
	}

	// Verify all directories exist
	for _, d := range []string{dir1, dir2, dir3} {
		info, err := os.Stat(d)
		require.NoError(t, err, "directory %s should exist", d)
		assert.True(t, info.IsDir())
	}
}

// TestConditionalTaskExecution verifies conditional tasks are evaluated correctly.
func TestConditionalTaskExecution(t *testing.T) {
	dir := t.TempDir()
	archDir := filepath.Join(dir, "arch-only")
	darwinDir := filepath.Join(dir, "darwin-only")

	configPath := filepath.Join(dir, "bootstrap.yaml")
	content := fmt.Sprintf(`version: "1"
tasks:
  - action: dir.create
    when:
      os: "arch"
    args:
      - %s
  - action: dir.create
    when:
      os: "darwin"
    args:
      - %s
`, archDir, darwinDir)
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o644))

	cfg, err := config.Load(configPath)
	require.NoError(t, err)

	// Use a controlled context to simulate arch OS
	condCtx := condition.Context{OS: "arch"}
	builder := task.DefaultBuilder(condCtx)

	tasks, err := builder.Build(cfg.Tasks)
	require.NoError(t, err)
	require.Len(t, tasks, 2)

	exec := executor.New(tasks)
	ctx := context.Background()

	// First task (arch) should run
	result1, ok1 := exec.RunNext(ctx)
	require.True(t, ok1)
	assert.Equal(t, task.StatusDone, result1.Status, "arch task should execute")

	// Second task (darwin) should skip
	result2, ok2 := exec.RunNext(ctx)
	require.True(t, ok2)
	assert.Equal(t, task.StatusSkipped, result2.Status, "darwin task should be skipped")

	// Verify only arch directory was created
	_, err = os.Stat(archDir)
	assert.NoError(t, err, "arch directory should exist")

	_, err = os.Stat(darwinDir)
	assert.True(t, os.IsNotExist(err), "darwin directory should NOT exist")
}

// TestVariableResolution_WithPromptCollector verifies variable resolution works end-to-end.
// This test was previously skipped because it required TUI interaction.
// Now PromptCollector supports io.Reader injection for testing.
func TestVariableResolution_WithPromptCollector(t *testing.T) {
	dir := t.TempDir()

	// Set up file store in temp directory
	storePath := filepath.Join(dir, "values.yaml")
	store := variable.NewFileStore(storePath)

	// Create a testable PromptCollector with injected input
	// User types "Alice" for Name and accepts default for Email
	input := strings.NewReader("Alice\n\n")
	collector := tui.NewPromptCollector().WithInput(input)

	// Create resolver with our test collector
	resolver := variable.NewResolver(store, variable.WithCollector(collector))

	// Define variables to resolve
	defs := []variable.Definition{
		{Name: "Name", Prompt: "Your name"},
		{Name: "Email", Prompt: "Your email", Default: "default@example.com"},
	}

	// Resolve variables
	result, err := resolver.Resolve(defs)

	require.NoError(t, err)
	assert.Equal(t, "Alice", result["Name"])
	assert.Equal(t, "default@example.com", result["Email"])

	// Verify values were persisted to store
	stored, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, "Alice", stored["Name"])
	assert.Equal(t, "default@example.com", stored["Email"])
}

// TestVariableResolution_ReusesStoredValues verifies stored values are used.
func TestVariableResolution_ReusesStoredValues(t *testing.T) {
	dir := t.TempDir()

	// Pre-populate the store with values
	storePath := filepath.Join(dir, "values.yaml")
	store := variable.NewFileStore(storePath)
	require.NoError(t, store.Save(map[string]string{
		"Name":  "Bob",
		"Email": "bob@example.com",
	}))

	// Collector should NOT be called since values are stored
	resolver := variable.NewResolver(store)

	defs := []variable.Definition{
		{Name: "Name", Prompt: "Your name"},
		{Name: "Email", Prompt: "Your email"},
	}

	result, err := resolver.Resolve(defs)

	require.NoError(t, err)
	assert.Equal(t, "Bob", result["Name"])
	assert.Equal(t, "bob@example.com", result["Email"])
}
