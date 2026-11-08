package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApply(t *testing.T) {
	dir := t.TempDir()
	newDir := filepath.Join(dir, "home", ".config")
	config := writeConfig(t, dir, fmt.Sprintf(`version: "1"
tasks:
  - name: dirs
    action: dir.create
    args: [%q]
`, newDir))

	t.Run("applies the changes and logs the run", func(t *testing.T) {
		res := booster(t, "--config", config, "apply", "--yes")

		assert.Equal(t, 0, res.code, res.stderr)
		assert.DirExists(t, newDir)
		assert.Contains(t, res.stdout, "+ dirs  changed")
		log := runLog(t, res.stdout)
		assert.Contains(t, log, "1 to change")
		assert.Contains(t, log, "+ dirs  changed")
	})

	t.Run("an up-to-date run asks nothing and still logs the run", func(t *testing.T) {
		res := booster(t, "--config", config, "apply")

		assert.Equal(t, 0, res.code, res.stderr)
		assert.Contains(t, res.stdout, "0 to change · 1 up to date")
		assert.NotContains(t, res.stdout, "apply these changes")
		assert.NotContains(t, res.stdout, "applying")
		log := runLog(t, res.stdout)
		assert.Contains(t, log, "0 to change · 1 up to date")
	})
}

func runLog(t *testing.T, stdout string) string {
	t.Helper()
	match := regexp.MustCompile(`(?m)^log: (.+)$`).FindStringSubmatch(stdout)
	require.NotNil(t, match, "no log path in output:\n%s", stdout)
	content, err := os.ReadFile(match[1])
	require.NoError(t, err)
	return string(content)
}

func TestApply_Interrupt(t *testing.T) {
	tests := []struct {
		name    string
		blockOn string
		check   func(t *testing.T, cmd *exec.Cmd, err error, stdout, stderr string)
	}{
		{
			name:    "while planning, the process stops at once",
			blockOn: "current",
			check: func(t *testing.T, cmd *exec.Cmd, err error, stdout, _ string) {
				require.Error(t, err)
				status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
				require.True(t, ok)
				assert.True(t, status.Signaled(), "exited normally instead of by the signal")
				assert.Equal(t, syscall.SIGINT, status.Signal())
				assert.NotContains(t, stdout, "applying")
			},
		},
		{
			name:    "while applying, the remaining steps do not run",
			blockOn: "use",
			check: func(t *testing.T, cmd *exec.Cmd, _ error, stdout, stderr string) {
				assert.Equal(t, 1, cmd.ProcessState.ExitCode(), stderr)
				assert.Contains(t, stderr, "apply interrupted")
				assert.Contains(t, stdout, "✗ tools  failed")
				assert.Contains(t, stdout, "- dirs  not run")
				log := runLog(t, stdout)
				assert.Contains(t, log, "- dirs  not run")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			ready, release := filepath.Join(dir, "ready"), filepath.Join(dir, "release")
			t.Cleanup(func() { _ = os.WriteFile(release, nil, 0o644) })
			fakeMise(t, dir, tt.blockOn, ready, release)
			newDir := filepath.Join(dir, "home", ".config")
			config := writeConfig(t, dir, fmt.Sprintf(`version: "1"
tasks:
  - name: tools
    action: mise.use
    args: [jq@1.7]
  - name: dirs
    action: dir.create
    args: [%q]
`, newDir))
			cmd, stdout, stderr := boosterCmd(t, "--config", config, "apply", "--yes")
			require.NoError(t, cmd.Start())

			waitForFile(t, ready)
			require.NoError(t, cmd.Process.Signal(os.Interrupt))
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			var err error
			select {
			case err = <-done:
			case <-time.After(5 * time.Second):
				_ = cmd.Process.Kill()
				require.FailNow(t, "booster kept running after the interrupt")
			}

			tt.check(t, cmd, err, stdout.String(), stderr.String())
			assert.NoDirExists(t, newDir)
		})
	}
}

func fakeMise(t *testing.T, dir, blockOn, ready, release string) {
	t.Helper()
	bin := filepath.Join(dir, "bin")
	require.NoError(t, os.MkdirAll(bin, 0o755))
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = %q ]; then
  touch %q
  while [ ! -f %q ]; do sleep 0.05; done
fi
`, blockOn, ready, release)
	require.NoError(t, os.WriteFile(filepath.Join(bin, "mise"), []byte(script), 0o755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.FailNow(t, "booster never reached the blocking step", path)
}
