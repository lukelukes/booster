package expr

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/expr-lang/expr"
)

// CompileOptions returns expr options with built-in functions registered.
func CompileOptions() []expr.Option {
	return []expr.Option{
		expr.Env(&Context{}),
		expr.Function("exists", existsFunc,
			new(func(string) bool),
		),
		expr.Function("which", whichFunc,
			new(func(string) string),
		),
		expr.Function("installed", installedFunc,
			new(func(string) bool),
		),
		expr.Function("default", defaultFunc,
			new(func(any, any) any),
		),
		expr.Function("expand", expandFunc,
			new(func(string) string),
		),
		expr.Function("hasSubstr", containsStrFunc,
			new(func(string, string) bool),
		),
		expr.Function("join", joinFunc,
			new(func([]any, string) string),
		),
	}
}

// exists checks if a file or directory exists.
// Usage: exists("~/.config/nvim")
func existsFunc(params ...any) (any, error) {
	if len(params) != 1 {
		return nil, fmt.Errorf("exists: expected 1 argument, got %d", len(params))
	}
	path, ok := params[0].(string)
	if !ok {
		return nil, fmt.Errorf("exists: expected string, got %T", params[0])
	}
	expanded := expandPath(path)
	_, err := os.Stat(expanded)
	return err == nil, nil
}

// which returns the path to an executable, or empty string if not found.
// Usage: which("nvim")
func whichFunc(params ...any) (any, error) {
	if len(params) != 1 {
		return nil, fmt.Errorf("which: expected 1 argument, got %d", len(params))
	}
	name, ok := params[0].(string)
	if !ok {
		return nil, fmt.Errorf("which: expected string, got %T", params[0])
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", nil
	}
	return path, nil
}

// installed checks if a command is available in PATH.
// Usage: installed("git")
func installedFunc(params ...any) (any, error) {
	if len(params) != 1 {
		return nil, fmt.Errorf("installed: expected 1 argument, got %d", len(params))
	}
	name, ok := params[0].(string)
	if !ok {
		return nil, fmt.Errorf("installed: expected string, got %T", params[0])
	}
	_, err := exec.LookPath(name)
	return err == nil, nil
}

// default returns the first non-nil/non-empty value.
// Usage: default(vars.editor, "vim")
func defaultFunc(params ...any) (any, error) {
	if len(params) != 2 {
		return nil, fmt.Errorf("default: expected 2 arguments, got %d", len(params))
	}
	val, fallback := params[0], params[1]
	if val == nil {
		return fallback, nil
	}
	if s, ok := val.(string); ok && s == "" {
		return fallback, nil
	}
	return val, nil
}

// expand expands ~ and environment variables in a path.
// Usage: expand("~/.config")
func expandFunc(params ...any) (any, error) {
	if len(params) != 1 {
		return nil, fmt.Errorf("expand: expected 1 argument, got %d", len(params))
	}
	path, ok := params[0].(string)
	if !ok {
		return nil, fmt.Errorf("expand: expected string, got %T", params[0])
	}
	return expandPath(path), nil
}

// containsStrFunc checks if a string contains a substring.
// Usage: contains("hello world", "world")
func containsStrFunc(params ...any) (any, error) {
	if len(params) != 2 {
		return nil, fmt.Errorf("contains: expected 2 arguments, got %d", len(params))
	}
	haystack, ok := params[0].(string)
	if !ok {
		return nil, fmt.Errorf("contains: expected string, got %T", params[0])
	}
	needle, ok := params[1].(string)
	if !ok {
		return nil, fmt.Errorf("contains: expected string, got %T", params[1])
	}
	return strings.Contains(haystack, needle), nil
}

// join concatenates list elements with a separator.
// Usage: join(packages, ", ")
func joinFunc(params ...any) (any, error) {
	if len(params) != 2 {
		return nil, fmt.Errorf("join: expected 2 arguments, got %d", len(params))
	}
	items, ok := params[0].([]any)
	if !ok {
		return nil, fmt.Errorf("join: expected list, got %T", params[0])
	}
	sep, ok := params[1].(string)
	if !ok {
		return nil, fmt.Errorf("join: expected string separator, got %T", params[1])
	}
	strs := make([]string, len(items))
	for i, item := range items {
		strs[i] = fmt.Sprint(item)
	}
	return strings.Join(strs, sep), nil
}

// expandPath expands ~ to home directory and environment variables.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[2:])
	}
	return os.ExpandEnv(path)
}
