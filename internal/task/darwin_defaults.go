package task

import (
	"booster/internal/cmdexec"
	"booster/internal/pathutil"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultsEntry represents a single macOS defaults setting.
type DefaultsEntry struct {
	Value  any    `yaml:"value"`
	Domain string `yaml:"domain"`
	Key    string `yaml:"key"`
	Type   string `yaml:"type"` // "bool", "int", "float", "string"
}

// DefaultsFile represents the structure of an external defaults YAML file.
type DefaultsFile struct {
	Defaults []DefaultsEntry `yaml:"defaults"`
}

// DarwinDefaults sets macOS defaults from an external YAML file.
type DarwinDefaults struct {
	Runner  cmdexec.Runner
	OS      string
	Entries []DefaultsEntry
}

// NeedsSudo returns false - git config operates on user config files.
func (t *DarwinDefaults) NeedsSudo() bool {
	return false
}

// Name returns a human-readable description for display.
func (t *DarwinDefaults) Name() string {
	if len(t.Entries) == 0 {
		return "set macOS defaults: (none)"
	}
	if len(t.Entries) == 1 {
		e := t.Entries[0]
		return fmt.Sprintf("set macOS defaults: %s %s", e.Domain, e.Key)
	}
	return fmt.Sprintf("set macOS defaults: %d entries", len(t.Entries))
}

// Run executes the defaults setting. It is idempotent.
func (t *DarwinDefaults) Run(ctx context.Context) Result {
	// Skip on non-darwin OS
	if t.OS != "darwin" {
		return Result{Status: StatusSkipped, Message: "not macOS"}
	}

	if len(t.Entries) == 0 {
		return Result{Status: StatusSkipped, Message: "no defaults to set"}
	}

	var changed int
	var alreadySet int
	var allOutput strings.Builder

	for _, entry := range t.Entries {
		// Read current value
		current, err := t.readDefault(ctx, entry.Domain, entry.Key)
		if err != nil {
			// If key doesn't exist, current will be empty and we'll write it
			current = ""
		}

		// Normalize and compare values
		desired := t.normalizeValue(entry.Type, entry.Value)
		normalized := t.normalizeValue(entry.Type, current)

		if normalized == desired && current != "" {
			alreadySet++
			continue
		}

		// Write the default
		output, err := t.writeDefault(ctx, entry.Domain, entry.Key, entry.Type, entry.Value)
		if output != "" {
			if allOutput.Len() > 0 {
				allOutput.WriteString("\n")
			}
			allOutput.WriteString(output)
		}
		if err != nil {
			return Result{
				Status: StatusFailed,
				Error:  fmt.Errorf("write %s %s: %w", entry.Domain, entry.Key, err),
				Output: allOutput.String(),
			}
		}
		changed++
	}

	// Build result message
	if changed == 0 {
		return Result{
			Status:  StatusSkipped,
			Message: fmt.Sprintf("all %d settings already configured", alreadySet),
			Output:  allOutput.String(),
		}
	}

	msg := fmt.Sprintf("configured %d settings", changed)
	if alreadySet > 0 {
		msg = fmt.Sprintf("configured %d settings, %d already set", changed, alreadySet)
	}

	return Result{Status: StatusDone, Message: msg, Output: allOutput.String()}
}

// readDefault reads the current value of a default.
func (t *DarwinDefaults) readDefault(ctx context.Context, domain, key string) (string, error) {
	output, err := t.Runner.Run(ctx, "defaults", "read", domain, key)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// writeDefault writes a default value.
func (t *DarwinDefaults) writeDefault(ctx context.Context, domain, key, typ string, value any) (string, error) {
	// Convert type to defaults flag
	var typeFlag string
	var valueStr string

	switch typ {
	case "bool":
		typeFlag = "-bool"
		// Convert value to string
		switch v := value.(type) {
		case bool:
			valueStr = strconv.FormatBool(v)
		case int:
			valueStr = strconv.FormatBool(v != 0)
		case string:
			// Accept "true"/"false" or "1"/"0"
			valueStr = v
		default:
			return "", fmt.Errorf("invalid bool value: %v", value)
		}
	case "int":
		typeFlag = "-int"
		valueStr = fmt.Sprintf("%v", value)
	case "float":
		typeFlag = "-float"
		valueStr = fmt.Sprintf("%v", value)
	case "string":
		typeFlag = "-string"
		valueStr = fmt.Sprintf("%v", value)
	default:
		return "", fmt.Errorf("unsupported type: %s", typ)
	}

	output, err := t.Runner.Run(ctx, "defaults", "write", domain, key, typeFlag, valueStr)
	return string(output), err
}

// normalizeValue normalizes values for comparison.
// For booleans: converts true/false/1/0 to "1" or "0"
// For other types: returns string representation
func (t *DarwinDefaults) normalizeValue(typ string, value any) string {
	if typ == "bool" {
		switch v := value.(type) {
		case bool:
			if v {
				return "1"
			}
			return "0"
		case int:
			if v != 0 {
				return "1"
			}
			return "0"
		case string:
			// Handle both "true"/"false" and "1"/"0"
			lower := strings.ToLower(strings.TrimSpace(v))
			if lower == "true" || lower == "1" {
				return "1"
			}
			if lower == "false" || lower == "0" {
				return "0"
			}
			return lower
		default:
			return fmt.Sprintf("%v", v)
		}
	}
	return fmt.Sprintf("%v", value)
}

// DarwinDefaultsConfig holds the factory configuration.
type DarwinDefaultsConfig struct {
	Runner    cmdexec.Runner
	OS        string
	ConfigDir string // Directory containing bootstrap.yaml (for resolving relative paths)
}

// NewDarwinDefaultsFactory creates a factory for DarwinDefaults tasks.
// The factory expects args in the format:
//
//	args:
//	  file: "macos-defaults.yaml"
func NewDarwinDefaultsFactory(cfg DarwinDefaultsConfig) Factory {
	return func(args any) ([]Task, error) {
		m, ok := args.(map[string]any)
		if !ok {
			return nil, errors.New("args must be a map with 'file' key")
		}

		fileRaw, hasFile := m["file"]
		if !hasFile {
			return nil, errors.New("missing required 'file' argument")
		}

		filePath, ok := fileRaw.(string)
		if !ok {
			return nil, errors.New("'file' must be a string")
		}

		// Resolve relative path relative to config directory
		resolvedPath := filePath
		if !strings.HasPrefix(filePath, "/") && !strings.HasPrefix(filePath, "~") {
			// Relative path - resolve relative to config directory
			if cfg.ConfigDir != "" {
				resolvedPath = pathutil.Expand(cfg.ConfigDir) + "/" + filePath
			}
		}

		// Load the external defaults file
		entries, err := loadDefaultsFile(resolvedPath)
		if err != nil {
			return nil, fmt.Errorf("load defaults file: %w", err)
		}

		// Validate entries
		for i, entry := range entries {
			if entry.Domain == "" {
				return nil, fmt.Errorf("entry %d: missing 'domain'", i+1)
			}
			if entry.Key == "" {
				return nil, fmt.Errorf("entry %d: missing 'key'", i+1)
			}
			if entry.Type == "" {
				return nil, fmt.Errorf("entry %d: missing 'type'", i+1)
			}
			if entry.Value == nil {
				return nil, fmt.Errorf("entry %d: missing 'value'", i+1)
			}
			// Validate type
			validTypes := map[string]bool{"bool": true, "int": true, "float": true, "string": true}
			if !validTypes[entry.Type] {
				return nil, fmt.Errorf("entry %d: invalid type %q (must be bool, int, float, or string)", i+1, entry.Type)
			}
		}

		runner := cfg.Runner
		if runner == nil {
			runner = cmdexec.DefaultRunner()
		}

		return []Task{&DarwinDefaults{
			Runner:  runner,
			Entries: entries,
			OS:      cfg.OS,
		}}, nil
	}
}

// loadDefaultsFile loads and parses a defaults YAML file.
func loadDefaultsFile(path string) ([]DefaultsEntry, error) {
	expanded := pathutil.Expand(path)

	data, err := os.ReadFile(expanded)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var file DefaultsFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}

	return file.Defaults, nil
}
