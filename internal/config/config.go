// Package config handles loading and parsing bootstrap configuration files.
package config

import (
	"booster/internal/pathutil"
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level bootstrap configuration.
type Config struct {
	Version   string                 `yaml:"version"`
	Profiles  []string               `yaml:"profiles,omitempty"`
	Variables map[string]VariableDef `yaml:"variables,omitempty"`
	Tasks     []Task                 `yaml:"tasks"`
}

// VariableDef defines a variable that can be prompted at runtime.
type VariableDef struct {
	Prompt  string `yaml:"prompt"`
	Default string `yaml:"default,omitempty"`
}

// Task represents a single action to execute.
type Task struct {
	Args   any    `yaml:"args"`
	When   *When  `yaml:"when,omitempty"`
	Action string `yaml:"action"`
}

// When defines conditions for task execution.
type When struct {
	OS      StringOrSlice `yaml:"os,omitempty"`
	Profile StringOrSlice `yaml:"profile,omitempty"`
}

// StringOrSlice handles YAML that can be either a string or []string.
// Allows: os: "arch" OR os: ["arch", "darwin"]
type StringOrSlice []string

// UnmarshalYAML implements custom unmarshaling for flexible YAML input.
func (s *StringOrSlice) UnmarshalYAML(unmarshal func(any) error) error {
	// Try single string first
	var single string
	if err := unmarshal(&single); err == nil {
		*s = []string{single}
		return nil
	}

	// Try slice of strings
	var slice []string
	if err := unmarshal(&slice); err != nil {
		return err
	}
	*s = slice
	return nil
}

// Load reads and parses a config file from the given path.
func Load(path string) (*Config, error) {
	expanded := pathutil.Expand(path)

	data, err := os.ReadFile(expanded)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.Version == "" {
		return nil, errors.New("config missing version field")
	}

	if cfg.Version != "1" {
		return nil, fmt.Errorf("unsupported config version: %s", cfg.Version)
	}

	// Validate tasks
	for i, task := range cfg.Tasks {
		if task.Action == "" {
			return nil, fmt.Errorf("task %d: action cannot be empty", i+1)
		}
	}

	return &cfg, nil
}
