// Package variable handles variable definition, resolution, and persistence.
package variable

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Definition represents a variable that can be prompted at runtime.
type Definition struct {
	Name    string
	Prompt  string
	Default string
}

// FileStore handles persistence of variable values to a YAML file.
type FileStore struct {
	path string
}

// NewFileStore creates a new FileStore at the given path.
func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

// Path returns the storage file path.
func (s *FileStore) Path() string {
	return s.path
}

// Load reads stored values from disk.
// Returns an empty map if the file doesn't exist.
func (s *FileStore) Load() (map[string]string, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, err
	}

	var values map[string]string
	if err := yaml.Unmarshal(data, &values); err != nil {
		return nil, err
	}

	if values == nil {
		return make(map[string]string), nil
	}
	return values, nil
}

// Save persists values to disk, creating parent directories if needed.
func (s *FileStore) Save(values map[string]string) error {
	// Create parent directories
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := yaml.Marshal(values)
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0o644)
}
