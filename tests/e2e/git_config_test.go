package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlan_GitConfigShowsOldAndNewValues(t *testing.T) {
	dir := t.TempDir()
	gitconfig := filepath.Join(dir, ".gitconfig")
	t.Setenv("GIT_CONFIG_GLOBAL", gitconfig)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	seed := exec.Command("git", "config", "--global", "user.email", "old@example.com")
	output, err := seed.CombinedOutput()
	require.NoError(t, err, string(output))
	config := writeConfig(t, dir, `version: "1"
tasks:
  - name: git
    action: git.config
    args:
      - {key: user.email, value: new@example.com}
`)

	res := booster(t, "--config", config, "plan")

	assert.Equal(t, 2, res.code, res.stderr)
	assert.Contains(t, res.stdout, "user.email  old@example.com → new@example.com")
	assert.Empty(t, res.stderr)
	content, err := os.ReadFile(gitconfig)
	require.NoError(t, err)
	assert.Contains(t, string(content), "old@example.com")
}
