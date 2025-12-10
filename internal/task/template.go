package task

import (
	"booster/internal/pathutil"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

// TemplateSystem holds system information available in templates.
type TemplateSystem struct {
	// OS is the detected operating system (e.g., "darwin", "arch", "ubuntu").
	OS string
	// Profile is the user-selected profile.
	Profile string
}

// TemplateContext is the data passed to templates during rendering.
type TemplateContext struct {
	// Vars contains user-defined variables from the config.
	Vars map[string]string
	// System contains detected system information.
	System TemplateSystem
}

// TemplateRender renders a template file to a target location.
type TemplateRender struct {
	Context TemplateContext
	Source  string
	Target  string
}

// Name returns a human-readable description.
func (t *TemplateRender) Name() string {
	return fmt.Sprintf("render %s → %s", filepath.Base(t.Source), filepath.Base(t.Target))
}

// Run executes the template rendering. It's idempotent - skips if output matches.
func (t *TemplateRender) Run(ctx context.Context) Result {
	source := pathutil.Expand(t.Source)
	target := pathutil.Expand(t.Target)

	// 1. Read template
	tmplContent, err := os.ReadFile(source)
	if err != nil {
		return Result{Status: StatusFailed, Error: fmt.Errorf("read template: %w", err)}
	}

	// 2. Parse template
	tmpl, err := template.New(filepath.Base(source)).Parse(string(tmplContent))
	if err != nil {
		return Result{Status: StatusFailed, Error: fmt.Errorf("parse template: %w", err)}
	}

	// 3. Execute template with context (Vars + System)
	var buf bytes.Buffer
	if execErr := tmpl.Execute(&buf, t.Context); execErr != nil {
		return Result{Status: StatusFailed, Error: fmt.Errorf("execute template: %w", execErr)}
	}
	rendered := buf.Bytes()

	// 4. Check idempotency - compare with existing target
	existing, err := os.ReadFile(target)
	if err == nil && bytes.Equal(existing, rendered) {
		return Result{Status: StatusSkipped, Message: "already up to date"}
	}

	// 5. Create parent directories
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return Result{Status: StatusFailed, Error: fmt.Errorf("create directories: %w", err)}
	}

	// 6. Write rendered content
	if err := os.WriteFile(target, rendered, 0o644); err != nil {
		return Result{Status: StatusFailed, Error: fmt.Errorf("write output: %w", err)}
	}

	return Result{Status: StatusDone, Message: "rendered"}
}

// TemplateRenderConfig holds configuration for creating template render tasks.
type TemplateRenderConfig struct {
	Vars    map[string]string
	OS      string
	Profile string
}

// NewTemplateRenderFactory returns a Factory that creates TemplateRender tasks.
// The config is captured in the closure and shared by all created tasks.
func NewTemplateRenderFactory(cfg TemplateRenderConfig) Factory {
	return func(args any) ([]Task, error) {
		pairs, err := parseSourceTargetArgs(args)
		if err != nil {
			return nil, err
		}

		ctx := TemplateContext{
			Vars: cfg.Vars,
			System: TemplateSystem{
				OS:      cfg.OS,
				Profile: cfg.Profile,
			},
		}

		tasks := make([]Task, 0, len(pairs))
		for _, pair := range pairs {
			tasks = append(tasks, &TemplateRender{
				Source:  pair.Source,
				Target:  pair.Target,
				Context: ctx,
			})
		}

		return tasks, nil
	}
}
