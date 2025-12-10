package cmdexec

import "context"

// MockRunner is a test double for Runner.
// Configure it with expected responses before use.
type MockRunner struct {
	// RunFunc is called when Run is invoked.
	// Set this to control test behavior.
	RunFunc func(ctx context.Context, name string, args ...string) ([]byte, error)

	// LookPathFunc is called when LookPath is invoked.
	LookPathFunc func(name string) (string, error)

	// Calls records all Run invocations for assertions.
	Calls []RunCall
}

// RunCall records a single Run invocation.
type RunCall struct {
	Name string
	Args []string
}

// Run delegates to RunFunc and records the call.
func (m *MockRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	m.Calls = append(m.Calls, RunCall{Name: name, Args: args})
	if m.RunFunc != nil {
		return m.RunFunc(ctx, name, args...)
	}
	return nil, nil
}

// LookPath delegates to LookPathFunc.
func (m *MockRunner) LookPath(name string) (string, error) {
	if m.LookPathFunc != nil {
		return m.LookPathFunc(name)
	}
	return "", nil
}
