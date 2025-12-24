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

	cfg, err := config.Load(configPath)
	require.NoError(t, err)

	condCtx := condition.Context{OS: "linux"}
	builder := task.DefaultBuilder(condCtx)
	tasks, err := builder.Build(cfg.Tasks)
	require.NoError(t, err)
	require.Len(t, tasks, 1)

	exec := executor.New(tasks)
	result, ok := exec.RunNext(context.Background())

	require.True(t, ok, "should have a task to run")
	assert.Equal(t, task.StatusDone, result.Status)
	assert.NoError(t, result.Error)

	info, err := os.Stat(targetDir)
	require.NoError(t, err, "directory should exist after task execution")
	assert.True(t, info.IsDir(), "should be a directory")
}

func TestDirCreate_Idempotent(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "idempotent-test")

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

	condCtx := condition.Context{OS: "linux"}
	builder1 := task.DefaultBuilder(condCtx)
	tasks1, err := builder1.Build(cfg.Tasks)
	require.NoError(t, err)

	exec1 := executor.New(tasks1)
	result1, ok1 := exec1.RunNext(context.Background())
	require.True(t, ok1)
	assert.Equal(t, task.StatusDone, result1.Status)

	builder2 := task.DefaultBuilder(condCtx)
	tasks2, err := builder2.Build(cfg.Tasks)
	require.NoError(t, err)

	exec2 := executor.New(tasks2)
	result2, ok2 := exec2.RunNext(context.Background())
	require.True(t, ok2)
	assert.Equal(t, task.StatusSkipped, result2.Status)
}

func TestSymlinkCreate_ExecutesForReal(t *testing.T) {
	dir := t.TempDir()

	sourceFile := filepath.Join(dir, "source.txt")
	require.NoError(t, os.WriteFile(sourceFile, []byte("test content"), 0o644))

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

	linkTarget, err := os.Readlink(targetLink)
	require.NoError(t, err, "symlink should exist")
	assert.Equal(t, sourceFile, linkTarget)

	content2, err := os.ReadFile(targetLink)
	require.NoError(t, err)
	assert.Equal(t, "test content", string(content2))
}

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

	exec := executor.New(tasks)
	ctx := context.Background()

	for i := range 3 {
		result, ok := exec.RunNext(ctx)
		require.True(t, ok, "task %d should exist", i+1)
		assert.Equal(t, task.StatusDone, result.Status, "task %d should succeed", i+1)
	}

	for _, d := range []string{dir1, dir2, dir3} {
		info, err := os.Stat(d)
		require.NoError(t, err, "directory %s should exist", d)
		assert.True(t, info.IsDir())
	}
}

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

	condCtx := condition.Context{OS: "arch"}
	builder := task.DefaultBuilder(condCtx)

	tasks, err := builder.Build(cfg.Tasks)
	require.NoError(t, err)
	require.Len(t, tasks, 2)

	exec := executor.New(tasks)
	ctx := context.Background()

	result1, ok1 := exec.RunNext(ctx)
	require.True(t, ok1)
	assert.Equal(t, task.StatusDone, result1.Status, "arch task should execute")

	result2, ok2 := exec.RunNext(ctx)
	require.True(t, ok2)
	assert.Equal(t, task.StatusSkipped, result2.Status, "darwin task should be skipped")

	_, err = os.Stat(archDir)
	assert.NoError(t, err, "arch directory should exist")

	_, err = os.Stat(darwinDir)
	assert.True(t, os.IsNotExist(err), "darwin directory should NOT exist")
}

func TestVariableResolution_WithPromptCollector(t *testing.T) {
	dir := t.TempDir()

	storePath := filepath.Join(dir, "values.yaml")
	store := variable.NewFileStore(storePath)

	input := strings.NewReader("Alice\n\n")
	collector := tui.NewPromptCollector().WithInput(input)

	resolver := variable.NewResolver(store, variable.WithCollector(collector))

	defs := []variable.Definition{
		{Name: "Name", Prompt: "Your name"},
		{Name: "Email", Prompt: "Your email", Default: "default@example.com"},
	}

	result, err := resolver.Resolve(defs)

	require.NoError(t, err)
	assert.Equal(t, "Alice", result["Name"])
	assert.Equal(t, "default@example.com", result["Email"])

	stored, err := store.Load()
	require.NoError(t, err)
	assert.Equal(t, "Alice", stored["Name"])
	assert.Equal(t, "default@example.com", stored["Email"])
}

func TestVariableResolution_ReusesStoredValues(t *testing.T) {
	dir := t.TempDir()

	storePath := filepath.Join(dir, "values.yaml")
	store := variable.NewFileStore(storePath)
	require.NoError(t, store.Save(map[string]string{
		"Name":  "Bob",
		"Email": "bob@example.com",
	}))

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
