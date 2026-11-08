package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

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
