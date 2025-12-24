package cmdexec

import "context"

type MockRunner struct {
	RunFunc func(ctx context.Context, name string, args ...string) ([]byte, error)

	LookPathFunc func(name string) (string, error)

	Calls []RunCall
}

type RunCall struct {
	Name string
	Args []string
}

func (m *MockRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	m.Calls = append(m.Calls, RunCall{Name: name, Args: args})
	if m.RunFunc != nil {
		return m.RunFunc(ctx, name, args...)
	}
	return nil, nil
}

func (m *MockRunner) LookPath(name string) (string, error) {
	if m.LookPathFunc != nil {
		return m.LookPathFunc(name)
	}
	return "", nil
}
