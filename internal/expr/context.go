package expr

import (
	"os"
	"runtime"
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
		OS:    normalizeOS(runtime.GOOS),
		Arch:  runtime.GOARCH,
		Home:  os.Getenv("HOME"),
		Env:   envToMap(),
		Vars:  make(map[string]any),
		Tasks: make(map[string]TaskResult),
	}
}

func (c *Context) WithProfile(profile string) *Context {
	cp := *c
	cp.Profile = profile
	return &cp
}

func (c *Context) WithVars(vars map[string]any) *Context {
	cp := *c
	cp.Vars = vars
	return &cp
}

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
	return goos
}
