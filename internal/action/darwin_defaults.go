package action

import (
	"booster/internal/cmdexec"
	"booster/internal/expr"
	"booster/internal/model"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type DefaultsEntry struct {
	Domain string
	Key    string
	Type   string
	Value  string
}

func (e DefaultsEntry) name() string {
	return e.Domain + " " + e.Key
}

type DarwinDefaults struct {
	Runner  cmdexec.Runner
	Entries []DefaultsEntry
}

func NewDarwinDefaults(args yaml.Node, ctx *expr.Context, runner cmdexec.Runner) (Action, error) {
	if ctx.OS != "darwin" {
		return nil, ErrNotApplicable
	}

	if args.Kind != yaml.MappingNode {
		return nil, errors.New("args must be a mapping with 'file'")
	}

	var raw struct {
		File expr.Str `yaml:"file"`
	}
	if err := args.Decode(&raw); err != nil {
		return nil, err
	}
	if raw.File == "" {
		return nil, errors.New("missing 'file'")
	}

	file, err := raw.File.Resolve(ctx)
	if err != nil {
		return nil, fmt.Errorf("file: %w", err)
	}

	entries, err := loadDefaultsFile(resolveConfigPath(file, ctx.ConfigDir))
	if err != nil {
		return nil, err
	}

	return &DarwinDefaults{Runner: runner, Entries: entries}, nil
}

func resolveConfigPath(path, configDir string) string {
	expanded := expr.ExpandPath(path)
	if filepath.IsAbs(expanded) || configDir == "" {
		return expanded
	}
	return filepath.Join(expr.ExpandPath(configDir), expanded)
}

func loadDefaultsFile(path string) ([]DefaultsEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read defaults file: %w", err)
	}

	var file struct {
		Defaults []struct {
			Domain string `yaml:"domain"`
			Key    string `yaml:"key"`
			Type   string `yaml:"type"`
			Value  any    `yaml:"value"`
		} `yaml:"defaults"`
	}
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse defaults file %s: %w", path, err)
	}

	entries := make([]DefaultsEntry, 0, len(file.Defaults))
	for i, raw := range file.Defaults {
		switch {
		case raw.Domain == "":
			return nil, fmt.Errorf("entry %d: missing 'domain'", i+1)
		case raw.Key == "":
			return nil, fmt.Errorf("entry %d: missing 'key'", i+1)
		case raw.Value == nil:
			return nil, fmt.Errorf("entry %d: missing 'value'", i+1)
		}

		value, err := normalizeDefault(raw.Type, raw.Value)
		if err != nil {
			return nil, fmt.Errorf("entry %d: %w", i+1, err)
		}

		entries = append(entries, DefaultsEntry{Domain: raw.Domain, Key: raw.Key, Type: raw.Type, Value: value})
	}

	return entries, nil
}

func normalizeDefault(typ string, value any) (string, error) {
	text := strings.TrimSpace(fmt.Sprint(value))

	switch typ {
	case "bool":
		switch strings.ToLower(text) {
		case "true", "1", "yes":
			return "true", nil
		case "false", "0", "no":
			return "false", nil
		}
		return "", fmt.Errorf("invalid bool value %q", text)
	case "int":
		n, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return "", fmt.Errorf("invalid int value %q", text)
		}
		return strconv.FormatInt(n, 10), nil
	case "float":
		f, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return "", fmt.Errorf("invalid float value %q", text)
		}
		return strconv.FormatFloat(f, 'g', -1, 64), nil
	case "string":
		return fmt.Sprint(value), nil
	}

	return "", fmt.Errorf("invalid type %q (must be bool, int, float, or string)", typ)
}

func (a *DarwinDefaults) Name() string {
	return "set.darwin.defaults"
}

func (a *DarwinDefaults) Check(ctx context.Context) (model.Change, error) {
	change := model.Change{Verb: "set", Total: len(a.Entries)}

	for _, entry := range a.Entries {
		current, exists, err := a.read(ctx, entry)
		if err != nil {
			return model.Change{}, err
		}
		if exists && current == entry.Value {
			continue
		}

		change.Items = append(change.Items, model.Item{Name: entry.name(), From: current, To: entry.Value})
		if exists {
			change.Kind = model.ChangeUpdate
		}
	}

	if change.Kind == model.ChangeNone && len(change.Items) > 0 {
		change.Kind = model.ChangeCreate
	}

	return change, nil
}

func (a *DarwinDefaults) Apply(ctx context.Context) error {
	for _, entry := range a.Entries {
		current, exists, err := a.read(ctx, entry)
		if err != nil {
			return err
		}
		if exists && current == entry.Value {
			continue
		}

		output, err := a.Runner.Run(ctx, "defaults", "write", entry.Domain, entry.Key, "-"+entry.Type, entry.Value)
		if err != nil {
			return fmt.Errorf("write %s: %w: %s", entry.name(), err, strings.TrimSpace(string(output)))
		}
	}

	return nil
}

func (a *DarwinDefaults) read(ctx context.Context, entry DefaultsEntry) (string, bool, error) {
	output, err := a.Runner.Run(ctx, "defaults", "read", entry.Domain, entry.Key)
	if err != nil {
		if missingDefault(err, output) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("read %s: %w: %s", entry.name(), err, strings.TrimSpace(string(output)))
	}

	current := strings.TrimSuffix(string(output), "\n")
	if entry.Type == "string" {
		return current, true, nil
	}

	normalized, err := normalizeDefault(entry.Type, current)
	if err != nil {
		return current, true, nil
	}
	return normalized, true, nil
}

func missingDefault(err error, output []byte) bool {
	var exit interface{ ExitCode() int }
	return errors.As(err, &exit) && exit.ExitCode() == 1 && strings.Contains(string(output), "does not exist")
}
