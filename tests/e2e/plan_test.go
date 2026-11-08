package e2e

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var binary string

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	dir, err := os.MkdirTemp("", "booster-e2e-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(dir)

	binary = filepath.Join(dir, "booster")
	build := exec.Command("go", "build", "-o", binary, "booster/cmd/cli")
	build.Stdout = os.Stderr
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "build booster:", err)
		return 1
	}

	return m.Run()
}

type result struct {
	code   int
	stdout string
	stderr string
}

func booster(t *testing.T, args ...string) result {
	t.Helper()
	cmd, stdout, stderr := boosterCmd(t, args...)

	err := cmd.Run()

	code := 0
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		code = exitErr.ExitCode()
	} else {
		require.NoError(t, err)
	}
	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func boosterCmd(t *testing.T, args ...string) (cmd *exec.Cmd, stdout, stderr *bytes.Buffer) {
	t.Helper()
	home := t.TempDir()
	cmd = exec.Command(binary, args...)
	cmd.Env = append(os.Environ(), "HOME="+home, "XDG_DATA_HOME="+filepath.Join(home, "data"), "XDG_STATE_HOME="+filepath.Join(home, "state"))
	stdout, stderr = &bytes.Buffer{}, &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd, stdout, stderr
}

func writeConfig(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "bootstrap.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestPlan(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "dotfiles", "zshrc")
	require.NoError(t, os.MkdirAll(filepath.Dir(source), 0o755))
	require.NoError(t, os.WriteFile(source, []byte("export X=1"), 0o644))
	newDir := filepath.Join(dir, "home", ".config")
	link := filepath.Join(dir, "home", ".zshrc")
	excluded := filepath.Join(dir, "never")

	config := writeConfig(t, dir, fmt.Sprintf(`version: "1"
tasks:
  - name: dirs
    action: dir.create
    args: [%q]
  - name: elsewhere
    action: dir.create
    when: ${ os == "plan9" }
    args: [%q]
  - name: links
    action: symlink.create
    args:
      - {source: %q, target: %q}
`, newDir, excluded, source, link))

	t.Run("exits 2 and reports drift without touching the system", func(t *testing.T) {
		res := booster(t, "--config", config, "plan")

		assert.Equal(t, 2, res.code)
		assert.Contains(t, res.stdout, "+ dirs")
		assert.Contains(t, res.stdout, "+ links")
		assert.Contains(t, res.stdout, link+"  → "+source)
		assert.NotContains(t, res.stdout, "elsewhere")
		assert.Contains(t, res.stdout, "2 to change · 0 up to date · 1 excluded")
		assert.Empty(t, res.stderr)
		assert.NoDirExists(t, newDir)
		assert.NoFileExists(t, link)
		assert.NoDirExists(t, excluded)
	})

	t.Run("exits 0 once the system matches", func(t *testing.T) {
		require.NoError(t, os.MkdirAll(newDir, 0o755))
		require.NoError(t, os.Symlink(source, link))

		res := booster(t, "--config", config, "plan")

		assert.Equal(t, 0, res.code)
		assert.Contains(t, res.stdout, "✓ dirs")
		assert.Contains(t, res.stdout, "✓ links")
		assert.Contains(t, res.stdout, "0 to change · 2 up to date · 1 excluded")
		assert.Empty(t, res.stderr)
	})
}

func TestPlan_InvalidConfigExits1BeforeChecking(t *testing.T) {
	dir := t.TempDir()
	config := writeConfig(t, dir, `version: "1"
tasks:
  - name: links
    action: symlink.create
    args:
      - {source: /does/not/exist, target: /nowhere}
  - name: mystery
    action: nope.nothing
`)

	res := booster(t, "--config", config, "plan")

	assert.Equal(t, 1, res.code)
	assert.Contains(t, res.stderr, `unknown action "nope.nothing"`)
	assert.NotContains(t, res.stderr, "source does not exist")
	assert.Empty(t, res.stdout)
}
