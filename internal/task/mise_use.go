package task

import (
	"booster/internal/cmdexec"
	"context"
	"errors"
	"fmt"
	"strings"
)

// ToolSpec represents a tool with its required version.
type ToolSpec struct {
	Name    string // e.g., "go", "node", "rust"
	Version string // e.g., "1.22.0", "20.10.0"
}

// String returns the tool@version format.
func (t ToolSpec) String() string {
	return t.Name + "@" + t.Version
}

// parseToolSpec parses a "tool@version" string into a ToolSpec.
func parseToolSpec(s string) (ToolSpec, error) {
	idx := strings.Index(s, "@")
	if idx == -1 || idx == 0 || idx == len(s)-1 {
		return ToolSpec{}, fmt.Errorf("invalid tool spec %q: expected format tool@version", s)
	}
	return ToolSpec{
		Name:    s[:idx],
		Version: s[idx+1:],
	}, nil
}

// MiseUse ensures toolchains are installed and set as global defaults via mise.
type MiseUse struct {
	Runner cmdexec.Runner
	Tools  []ToolSpec
}

// Name returns a human-readable description for display.
func (t *MiseUse) Name() string {
	if len(t.Tools) == 0 {
		return "mise use: (none)"
	}
	if len(t.Tools) <= 3 {
		specs := make([]string, len(t.Tools))
		for i, tool := range t.Tools {
			specs[i] = tool.String()
		}
		return "mise use: " + strings.Join(specs, ", ")
	}
	return fmt.Sprintf("mise use: %d tools", len(t.Tools))
}

// NeedsSudo returns false - mise operates in user space.
func (t *MiseUse) NeedsSudo() bool {
	return false
}

// Run executes the mise configuration. It is idempotent.
func (t *MiseUse) Run(ctx context.Context) Result {
	runner := t.Runner
	if runner == nil {
		runner = cmdexec.DefaultRunner()
	}

	// Check mise is available
	if err := t.checkMiseAvailable(runner); err != nil {
		return Result{Status: StatusFailed, Error: err, Message: "mise not installed"}
	}

	// Find which tools need to be installed/updated
	missing := t.findMissingTools(ctx, runner)

	// If all tools are already at correct versions, skip
	if len(missing) == 0 {
		return Result{Status: StatusSkipped, Message: "all tools at correct versions"}
	}

	// Install each missing tool
	var allOutput strings.Builder
	for _, tool := range missing {
		output, err := t.installTool(ctx, runner, tool)
		if output != "" {
			if allOutput.Len() > 0 {
				allOutput.WriteString("\n")
			}
			allOutput.WriteString(output)
		}
		if err != nil {
			return Result{Status: StatusFailed, Error: err, Output: allOutput.String()}
		}
	}

	msg := fmt.Sprintf("configured %d tool(s)", len(missing))
	return Result{Status: StatusDone, Message: msg, Output: allOutput.String()}
}

// checkMiseAvailable returns an error if mise is not in PATH.
func (t *MiseUse) checkMiseAvailable(runner cmdexec.Runner) error {
	_, err := runner.LookPath("mise")
	if err != nil {
		return errors.New("mise not found in PATH; install mise first (e.g., via pkg.install)")
	}
	return nil
}

// getCurrentVersion returns the current version of a tool, or empty if not set.
func (t *MiseUse) getCurrentVersion(ctx context.Context, runner cmdexec.Runner, toolName string) string {
	output, err := runner.Run(ctx, "mise", "current", toolName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// findMissingTools returns the tools that need to be installed or updated.
func (t *MiseUse) findMissingTools(ctx context.Context, runner cmdexec.Runner) []ToolSpec {
	var missing []ToolSpec
	for _, tool := range t.Tools {
		current := t.getCurrentVersion(ctx, runner, tool.Name)
		if current != tool.Version {
			missing = append(missing, tool)
		}
	}
	return missing
}

// installTool installs and sets a tool as global default.
func (t *MiseUse) installTool(ctx context.Context, runner cmdexec.Runner, tool ToolSpec) (string, error) {
	spec := tool.String()
	output, err := runner.Run(ctx, "mise", "use", "--global", spec)
	if err != nil {
		return string(output), fmt.Errorf("mise use %s: %w", spec, err)
	}
	return string(output), nil
}

// MiseUseConfig holds the factory configuration.
type MiseUseConfig struct {
	Runner cmdexec.Runner
}

// NewMiseUseFactory creates a factory for MiseUse tasks.
func NewMiseUseFactory(cfg MiseUseConfig) Factory {
	return func(args any) ([]Task, error) {
		list, ok := args.([]any)
		if !ok {
			return nil, errors.New("args must be a list of tool@version specs")
		}

		var tools []ToolSpec
		for i, item := range list {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("arg %d: must be a string", i+1)
			}
			spec, err := parseToolSpec(s)
			if err != nil {
				return nil, fmt.Errorf("arg %d: %w", i+1, err)
			}
			tools = append(tools, spec)
		}

		if len(tools) == 0 {
			return nil, nil
		}

		runner := cfg.Runner
		if runner == nil {
			runner = cmdexec.DefaultRunner()
		}

		return []Task{&MiseUse{
			Runner: runner,
			Tools:  tools,
		}}, nil
	}
}
