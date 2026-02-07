package expr

import (
	"maps"
	"os"
	"runtime"
	"strings"
)

type Context struct {
	OS   string `expr:"os"`
	Arch string `expr:"arch"`
	Home string `expr:"home"`

	Profile string `expr:"profile"`

	Env map[string]string `expr:"env"`

	Vars map[string]any `expr:"vars"`

	Tasks map[string]TaskResult `expr:"tasks"`
}

type TaskResult struct {
	Output any    `expr:"output"`
	Status string `expr:"status"` // "done", "failed", "skipped"
}

func NewContext() *Context {
	return &Context{
		OS:    normalizeOS(detectOS()),
		Arch:  runtime.GOARCH,
		Home:  os.Getenv("HOME"),
		Env:   envToMap(),
		Vars:  make(map[string]any),
		Tasks: make(map[string]TaskResult),
	}
}

func (c *Context) WithProfile(profile string) *Context {
	cp := c.clone()
	cp.Profile = profile
	return cp
}

func (c *Context) WithVars(vars map[string]any) *Context {
	cp := c.clone()
	cp.Vars = vars
	return cp
}

func (c *Context) SetTaskResult(name string, output any, status string) {
	c.Tasks[name] = TaskResult{Output: output, Status: status}
}

func (c *Context) clone() *Context {
	cp := *c

	cp.Env = make(map[string]string, len(c.Env))
	maps.Copy(cp.Env, c.Env)

	cp.Vars = make(map[string]any, len(c.Vars))
	maps.Copy(cp.Vars, c.Vars)

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
	return goos
}
