package variable

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileStore_Load_NonExistent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent", "values.yaml")

	store := NewFileStore(path)
	values, err := store.Load()

	require.NoError(t, err)
	assert.Empty(t, values)
}

func TestFileStore_Save_CreatesParentDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "nested", "values.yaml")

	store := NewFileStore(path)
	err := store.Save(map[string]string{"Name": "Alice"})

	require.NoError(t, err)

	// Verify file was created
	_, err = os.Stat(path)
	assert.NoError(t, err)
}

func TestFileStore_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "values.yaml")

	store := NewFileStore(path)

	// Save values
	original := map[string]string{
		"Name":  "Alice",
		"Email": "alice@example.com",
	}
	err := store.Save(original)
	require.NoError(t, err)

	// Load values back
	loaded, err := store.Load()
	require.NoError(t, err)

	assert.Equal(t, original, loaded)
}

func TestFileStore_Save_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "values.yaml")

	store := NewFileStore(path)

	// Save initial values
	err := store.Save(map[string]string{"Name": "Alice"})
	require.NoError(t, err)

	// Overwrite with new values
	err = store.Save(map[string]string{"Name": "Bob", "Email": "bob@example.com"})
	require.NoError(t, err)

	// Verify overwrite
	loaded, err := store.Load()
	require.NoError(t, err)

	assert.Equal(t, "Bob", loaded["Name"])
	assert.Equal(t, "bob@example.com", loaded["Email"])
}

func TestFileStore_Path(t *testing.T) {
	path := "/some/path/values.yaml"
	store := NewFileStore(path)

	assert.Equal(t, path, store.Path())
}
