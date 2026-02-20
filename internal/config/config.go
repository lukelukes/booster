package config

import (
	"booster/internal/expr"
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version   string                 `yaml:"version"`
	Profiles  []string               `yaml:"profiles,omitempty"`
	Variables map[string]VariableDef `yaml:"variables,omitempty"`
	Tasks     []Task                 `yaml:"tasks"`
}

type VariableDef struct {
	Prompt  string `yaml:"prompt"`
	Default string `yaml:"default,omitempty"`
}

type Task struct {
	Args   any       `yaml:"args"`
	When   *WhenExpr `yaml:"when,omitempty"`
	Action string    `yaml:"action"`
}

type WhenExpr string

func (w *WhenExpr) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == 0 {
		return nil
	}

	switch node.Kind {
	case yaml.ScalarNode:
		value := strings.TrimSpace(node.Value)
		if value == "" {
			return errors.New("when cannot be empty; use an expression like `${ os == \"darwin\" }`")
		}
		if !strings.HasPrefix(value, "${") || !strings.HasSuffix(value, "}") {
			return fmt.Errorf("when must be an expression string in `${ ... }` form, got %q", value)
		}
		*w = WhenExpr(value)
		return nil
	case yaml.MappingNode:
		return errors.New("when must be an expression string in `${ ... }` form")
	default:
		return errors.New("when must be an expression string in `${ ... }` form")
	}
}

type StringOrSlice []string

func (s *StringOrSlice) UnmarshalYAML(unmarshal func(any) error) error {
	var single string
	if err := unmarshal(&single); err == nil {
		*s = []string{single}
		return nil
	}

	var slice []string
	if err := unmarshal(&slice); err != nil {
		return err
	}
	*s = slice
	return nil
}

func Load(path string) (*Config, error) {
	expanded := expr.ExpandPath(path)

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

	for i, task := range cfg.Tasks {
		if task.Action == "" {
			return nil, fmt.Errorf("task %d: action cannot be empty", i+1)
		}
	}

	return &cfg, nil
}
