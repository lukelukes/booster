// Package pathutil provides path manipulation utilities.
package pathutil

import (
	"os"
	"path/filepath"
	"strings"
)

// Expand expands ~ to the user's home directory.
func Expand(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
