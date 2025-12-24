package task

import "context"

type mockPackageManager struct {
	installErr     error
	installCaskErr error
	listErr        error
	listCaskErr    error
	installed      map[string]bool
	casksInstalled map[string]bool
	name           string
	installCalls   [][]string
	caskCalls      [][]string
	supportsCasks  bool
}

func newMockManager(name string, supportsCasks bool) *mockPackageManager {
	return &mockPackageManager{
		name:           name,
		installed:      make(map[string]bool),
		casksInstalled: make(map[string]bool),
		supportsCasks:  supportsCasks,
	}
}

func (m *mockPackageManager) Name() string { return m.name }

func (m *mockPackageManager) ListInstalled(ctx context.Context) ([]string, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var result []string
	for pkg := range m.installed {
		result = append(result, pkg)
	}
	return result, nil
}

func (m *mockPackageManager) Install(ctx context.Context, pkgs []string) (string, error) {
	m.installCalls = append(m.installCalls, pkgs)
	if m.installErr != nil {
		return "mock install output", m.installErr
	}
	for _, pkg := range pkgs {
		m.installed[pkg] = true
	}
	return "mock install output", nil
}

func (m *mockPackageManager) ListInstalledCasks(ctx context.Context) ([]string, error) {
	if m.listCaskErr != nil {
		return nil, m.listCaskErr
	}
	var result []string
	for cask := range m.casksInstalled {
		result = append(result, cask)
	}
	return result, nil
}

func (m *mockPackageManager) InstallCasks(ctx context.Context, casks []string) (string, error) {
	m.caskCalls = append(m.caskCalls, casks)
	if m.installCaskErr != nil {
		return "mock cask output", m.installCaskErr
	}
	for _, cask := range casks {
		m.casksInstalled[cask] = true
	}
	return "mock cask output", nil
}

func (m *mockPackageManager) SupportsCasks() bool { return m.supportsCasks }
