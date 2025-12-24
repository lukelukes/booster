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

type DefaultsEntry struct {
	Value  any    `yaml:"value"`
	Domain string `yaml:"domain"`
	Key    string `yaml:"key"`
	Type   string `yaml:"type"`
}

type DefaultsFile struct {
	Defaults []DefaultsEntry `yaml:"defaults"`
}

type DarwinDefaults struct {
	Runner  cmdexec.Runner
	OS      string
	Entries []DefaultsEntry
}

func (t *DarwinDefaults) NeedsSudo() bool {
	return false
}

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

func (t *DarwinDefaults) Run(ctx context.Context) Result {
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
		current, err := t.readDefault(ctx, entry.Domain, entry.Key)
		if err != nil {
			current = ""
		}

		desired := t.normalizeValue(entry.Type, entry.Value)
		normalized := t.normalizeValue(entry.Type, current)

		if normalized == desired && current != "" {
			alreadySet++
			continue
		}

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

func (t *DarwinDefaults) readDefault(ctx context.Context, domain, key string) (string, error) {
	output, err := t.Runner.Run(ctx, "defaults", "read", domain, key)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func (t *DarwinDefaults) writeDefault(ctx context.Context, domain, key, typ string, value any) (string, error) {
	var typeFlag string
	var valueStr string

	switch typ {
	case "bool":
		typeFlag = "-bool"

		switch v := value.(type) {
		case bool:
			valueStr = strconv.FormatBool(v)
		case int:
			valueStr = strconv.FormatBool(v != 0)
		case string:

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

type DarwinDefaultsConfig struct {
	Runner    cmdexec.Runner
	OS        string
	ConfigDir string
}

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

		resolvedPath := filePath
		if !strings.HasPrefix(filePath, "/") && !strings.HasPrefix(filePath, "~") {
			if cfg.ConfigDir != "" {
				resolvedPath = pathutil.Expand(cfg.ConfigDir) + "/" + filePath
			}
		}

		entries, err := loadDefaultsFile(resolvedPath)
		if err != nil {
			return nil, fmt.Errorf("load defaults file: %w", err)
		}

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
