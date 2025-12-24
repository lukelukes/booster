package pathutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpand(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "tilde prefix expands to home",
			input:    "~/Documents",
			expected: filepath.Join(home, "Documents"),
		},
		{
			name:     "nested path after tilde",
			input:    "~/.config/app/settings",
			expected: filepath.Join(home, ".config/app/settings"),
		},
		{
			name:     "absolute path unchanged",
			input:    "/etc/config",
			expected: "/etc/config",
		},
		{
			name:     "relative path unchanged",
			input:    "relative/path",
			expected: "relative/path",
		},
		{
			name:     "tilde in middle unchanged",
			input:    "/path/~/file",
			expected: "/path/~/file",
		},
		{
			name:     "just tilde slash expands to home",
			input:    "~/",
			expected: home,
		},
		{
			name:     "bare tilde unchanged (no slash)",
			input:    "~",
			expected: "~",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Expand(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExpand_FallsBackWhenHomeUnset(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("USER", "")

	require.NoError(t, os.Unsetenv("HOME"))
	require.NoError(t, os.Unsetenv("USER"))

	result := Expand("~/test")
	assert.Equal(t, "~/test", result)
}
