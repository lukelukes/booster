package expr

import (
	"maps"
	"os"
	"runtime"
	"strings"
)

// Context holds all values available during expression evaluation.
// This is passed to expr-lang as the evaluation environment.
type Context struct {
	// System context (read-only, derived from runtime)
	OS   string `expr:"os"`
	Arch string `expr:"arch"`
	Home string `expr:"home"`

	// User-selected profile
	Profile string `expr:"profile"`

	// Environment variables (accessed as env.VAR_NAME)
	Env map[string]string `expr:"env"`

	// User-defined variables from config
	Vars map[string]any `expr:"vars"`

	// Task outputs (populated during execution)
	// Accessed as tasks.task_name.output, tasks.task_name.status
	Tasks map[string]TaskResult `expr:"tasks"`
}

// TaskResult holds the output of a completed task.
type TaskResult struct {
	Output any    `expr:"output"`
	Status string `expr:"status"` // "done", "failed", "skipped"
}

// NewContext creates a Context populated with system defaults.
func NewContext() *Context {
	return &Context{
		OS:    normalizeOS(runtime.GOOS),
		Arch:  runtime.GOARCH,
		Home:  os.Getenv("HOME"),
		Env:   envToMap(),
		Vars:  make(map[string]any),
		Tasks: make(map[string]TaskResult),
	}
}

// WithProfile returns a copy of the context with the profile set.
// The returned context has its own copies of all maps to prevent mutation issues.
func (c *Context) WithProfile(profile string) *Context {
	cp := c.clone()
	cp.Profile = profile
	return cp
}

// WithVars returns a copy of the context with variables set.
// The returned context has its own copies of all maps to prevent mutation issues.
func (c *Context) WithVars(vars map[string]any) *Context {
	cp := c.clone()
	cp.Vars = vars
	return cp
}

// SetTaskResult records the result of a completed task.
// Note: This mutates the context in place. Use clone() first if you need isolation.
func (c *Context) SetTaskResult(name string, output any, status string) {
	c.Tasks[name] = TaskResult{Output: output, Status: status}
}

// clone creates a deep copy of the context with independent maps.
func (c *Context) clone() *Context {
	cp := *c

	// Deep copy Env map
	cp.Env = make(map[string]string, len(c.Env))
	maps.Copy(cp.Env, c.Env)

	// Deep copy Vars map
	cp.Vars = make(map[string]any, len(c.Vars))
	maps.Copy(cp.Vars, c.Vars)

	// Deep copy Tasks map
	cp.Tasks = make(map[string]TaskResult, len(c.Tasks))
	maps.Copy(cp.Tasks, c.Tasks)

	return &cp
}

func envToMap() map[string]string {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		k, v, _ := strings.Cut(e, "=")
		env[k] = v
	}
	return env
}

func normalizeOS(goos string) string {
	// Map GOOS to user-friendly names if desired
	// For now, pass through directly
	return goos
}
