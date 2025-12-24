package task

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirCreate_CreatesNewDirectory(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "newdir")

	task := &DirCreate{Path: targetPath}
	result := task.Run(context.Background())

	assert.Equal(t, StatusDone, result.Status)
	assert.Equal(t, "created", result.Message)

	info, err := os.Stat(targetPath)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestDirCreate_SkipsExistingDirectory(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "existing")

	require.NoError(t, os.Mkdir(targetPath, 0o755))

	task := &DirCreate{Path: targetPath}
	result := task.Run(context.Background())

	assert.Equal(t, StatusSkipped, result.Status)
	assert.Equal(t, "already exists", result.Message)
}

func TestDirCreate_FailsWhenFileExists(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "file")

	require.NoError(t, os.WriteFile(filePath, []byte("content"), 0o644))

	task := &DirCreate{Path: filePath}
	result := task.Run(context.Background())

	assert.Equal(t, StatusFailed, result.Status)
	assert.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "not a directory")
}

func TestDirCreate_CreatesNestedDirectories(t *testing.T) {
	dir := t.TempDir()
	nestedPath := filepath.Join(dir, "a", "b", "c")

	task := &DirCreate{Path: nestedPath}
	result := task.Run(context.Background())

	assert.Equal(t, StatusDone, result.Status)

	info, err := os.Stat(nestedPath)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestDirCreate_Idempotency(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "idempotent")

	task := &DirCreate{Path: targetPath}

	result1 := task.Run(context.Background())
	assert.Equal(t, StatusDone, result1.Status)

	result2 := task.Run(context.Background())
	assert.Equal(t, StatusSkipped, result2.Status)

	result3 := task.Run(context.Background())
	assert.Equal(t, StatusSkipped, result3.Status)
}

func TestDirCreate_Name(t *testing.T) {
	task := &DirCreate{Path: "~/.config/myapp"}
	assert.Equal(t, "create ~/.config/myapp", task.Name())
}

func TestNewDirCreate_ValidArgs(t *testing.T) {
	args := []any{"~/dir1", "~/dir2"}

	tasks, err := NewDirCreate(args)

	require.NoError(t, err)
	assert.Len(t, tasks, 2)

	assert.Equal(t, "create ~/dir1", tasks[0].Name())
	assert.Equal(t, "create ~/dir2", tasks[1].Name())
}

func TestNewDirCreate_InvalidArgs_NotList(t *testing.T) {
	args := "not a list"

	_, err := NewDirCreate(args)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be a list")
}

func TestNewDirCreate_InvalidArgs_NotString(t *testing.T) {
	args := []any{"valid", 123, "also-valid"}

	_, err := NewDirCreate(args)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "arg 2")
	assert.Contains(t, err.Error(), "must be a string")
}

func TestNewDirCreate_EmptyList(t *testing.T) {
	args := []any{}

	tasks, err := NewDirCreate(args)

	require.NoError(t, err)
	assert.Empty(t, tasks)
}
