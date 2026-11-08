package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyCmd_CmdActionConvergesOnItsCheck(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "installed")
	configPath := filepath.Join(dir, "bootstrap.yaml")
	require.NoError(t, os.WriteFile(configPath, fmt.Appendf(nil, `version: "1"
tasks:
  - name: marker
    action: cmd
    args: { check: [test, -f, %q], run: [touch, %q] }
`, marker, marker), 0o644))
	cli := &CLI{Config: configPath}
	var first fakeTerminal

	require.NoError(t, (&ApplyCmd{Yes: true}).run(t.Context(), cli, first.terminal(t, "", false)))

	assert.Contains(t, first.out.String(), "+ marker")
	assert.FileExists(t, marker)

	var second fakeTerminal
	require.NoError(t, (&ApplyCmd{}).run(t.Context(), cli, second.terminal(t, "", false)))
	assert.Contains(t, second.out.String(), "0 to change · 1 up to date")
}
