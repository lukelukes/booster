package expr

import (
	"os"
	"runtime"
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
func (c *Context) WithProfile(profile string) *Context {
	cp := *c
	cp.Profile = profile
	return &cp
}

// WithVars returns a copy of the context with variables set.
func (c *Context) WithVars(vars map[string]any) *Context {
	cp := *c
	cp.Vars = vars
	return &cp
}

// SetTaskResult records the result of a completed task.
func (c *Context) SetTaskResult(name string, output any, status string) {
	c.Tasks[name] = TaskResult{Output: output, Status: status}
}

func envToMap() map[string]string {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		for i := 0; i < len(e); i++ {
			if e[i] == '=' {
				env[e[:i]] = e[i+1:]
				break
			}
		}
	}
	return env
}

func normalizeOS(goos string) string {
	// Map GOOS to user-friendly names if desired
	// For now, pass through directly
	return goos
}
