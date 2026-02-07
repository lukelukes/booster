package config

import (
	"booster/internal/pathutil"
	"errors"
	"fmt"
	"os"
	"strconv"
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

const whenMigrationExamples = "legacy mapping migration examples: `when: { os: \"darwin\" }` -> `when: ${ os == \"darwin\" }`, `when: { os: [\"arch\", \"darwin\"], profile: \"work\" }` -> `when: ${ os in [\"arch\", \"darwin\"] and profile == \"work\" }`"

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
			return fmt.Errorf("when must be an expression string in `${ ... }` form, got %q; %s", value, whenMigrationExamples)
		}
		*w = WhenExpr(value)
		return nil
	case yaml.MappingNode:
		exprValue, err := parseLegacyWhenMapping(node)
		if err != nil {
			return err
		}
		*w = WhenExpr(exprValue)
		return nil
	default:
		return fmt.Errorf("when must be an expression string in `${ ... }` form or legacy mapping with os/profile; %s", whenMigrationExamples)
	}
}

func parseLegacyWhenMapping(node *yaml.Node) (string, error) {
	var osValues []string
	var profileValues []string

	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]

		switch strings.TrimSpace(keyNode.Value) {
		case "os":
			values, err := parseLegacyWhenMappingValues(valueNode, "os")
			if err != nil {
				return "", err
			}
			osValues = values
		case "profile":
			values, err := parseLegacyWhenMappingValues(valueNode, "profile")
			if err != nil {
				return "", err
			}
			profileValues = values
		default:
			return "", fmt.Errorf("legacy when mapping only supports keys \"os\" and \"profile\", got %q; %s", strings.TrimSpace(keyNode.Value), whenMigrationExamples)
		}
	}

	if len(osValues) == 0 && len(profileValues) == 0 {
		return "${ true }", nil
	}

	parts := make([]string, 0, 2)
	if len(osValues) > 0 {
		parts = append(parts, buildWhenSelectorExpr("os", osValues))
	}
	if len(profileValues) > 0 {
		parts = append(parts, buildWhenSelectorExpr("profile", profileValues))
	}

	return "${ " + strings.Join(parts, " and ") + " }", nil
}

func parseLegacyWhenMappingValues(node *yaml.Node, field string) ([]string, error) {
	switch node.Kind {
	case yaml.ScalarNode:
		if node.Tag != "!!str" {
			return nil, fmt.Errorf("legacy when mapping field %q must be a string or list of strings; %s", field, whenMigrationExamples)
		}
		value := strings.TrimSpace(node.Value)
		if value == "" {
			return nil, fmt.Errorf("legacy when mapping field %q cannot be empty; %s", field, whenMigrationExamples)
		}
		return []string{value}, nil
	case yaml.SequenceNode:
		if len(node.Content) == 0 {
			return nil, fmt.Errorf("legacy when mapping field %q cannot be an empty list; %s", field, whenMigrationExamples)
		}
		values := make([]string, 0, len(node.Content))
		for _, item := range node.Content {
			if item.Kind != yaml.ScalarNode || item.Tag != "!!str" {
				return nil, fmt.Errorf("legacy when mapping field %q must be a string or list of strings; %s", field, whenMigrationExamples)
			}
			value := strings.TrimSpace(item.Value)
			if value == "" {
				return nil, fmt.Errorf("legacy when mapping field %q cannot include empty values; %s", field, whenMigrationExamples)
			}
			values = append(values, value)
		}
		return values, nil
	default:
		return nil, fmt.Errorf("legacy when mapping field %q must be a string or list of strings; %s", field, whenMigrationExamples)
	}
}

func buildWhenSelectorExpr(field string, values []string) string {
	if len(values) == 1 {
		return field + " == " + strconv.Quote(values[0])
	}

	quoted := make([]string, 0, len(values))
	for _, v := range values {
		quoted = append(quoted, strconv.Quote(v))
	}

	return field + " in [" + strings.Join(quoted, ", ") + "]"
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

	for i, task := range cfg.Tasks {
		if task.Action == "" {
			return nil, fmt.Errorf("task %d: action cannot be empty", i+1)
		}
	}

	return &cfg, nil
}
