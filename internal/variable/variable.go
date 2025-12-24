package variable

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Definition struct {
	Name    string
	Prompt  string
	Default string
}

type FileStore struct {
	path string
}

func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

func (s *FileStore) Path() string {
	return s.path
}

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

func (s *FileStore) Save(values map[string]string) error {
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
