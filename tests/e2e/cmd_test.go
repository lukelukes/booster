package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApply_CmdPassesArgvLiterally(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	require.NoError(t, os.Mkdir(target, 0o755))
	name := `it's a "file"; $HOME * ?.txt`
	marker := filepath.Join(target, name)
	config := writeConfig(t, dir, fmt.Sprintf(`version: "1"
tasks:
  - name: marker
    action: cmd
    args: { check: [test, -f, %q], run: [touch, %q] }
`, marker, marker))

	t.Run("creates exactly the named file", func(t *testing.T) {
		res := booster(t, "--config", config, "apply", "--yes")

		assert.Equal(t, 0, res.code, res.stderr)
		assert.Contains(t, res.stdout, "+ marker  changed")
		entries, err := os.ReadDir(target)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, name, entries[0].Name())
	})

	t.Run("the check makes the next run a no-op", func(t *testing.T) {
		res := booster(t, "--config", config, "apply")

		assert.Equal(t, 0, res.code, res.stderr)
		assert.Contains(t, res.stdout, "0 to change · 1 up to date")
		entries, err := os.ReadDir(target)
		require.NoError(t, err)
		assert.Len(t, entries, 1)
	})
}
