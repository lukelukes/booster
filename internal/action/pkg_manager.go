package action

import (
	"context"
	"os"
	"strings"
)

type PackageManager interface {
	Name() string

	Available(ctx context.Context) bool

	ListInstalled(ctx context.Context) ([]string, error)

	Install(ctx context.Context, pkgs []string) (output string, err error)
}

type CaskInstaller interface {
	ListInstalledCasks(ctx context.Context) ([]string, error)

	InstallCasks(ctx context.Context, casks []string) (output string, err error)
}

type BrewPathFinder func() (path string, found bool)

var knownBrewPaths = []string{
	"/opt/homebrew/bin/brew",
	"/usr/local/bin/brew",
	"/home/linuxbrew/.linuxbrew/bin/brew",
}

func defaultBrewPathFinder() (string, bool) {
	for _, path := range knownBrewPaths {
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
	}
	return "", false
}

func parseLines(output string) []string {
	lines := strings.Split(strings.TrimSpace(output), "\n")

	var result []string
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			result = append(result, line)
		}
	}

	return result
}

func missingFrom(installed, wanted []string) []string {
	present := make(map[string]bool, len(installed))
	for _, item := range installed {
		present[item] = true
	}

	var missing []string
	for _, item := range wanted {
		if !present[item] {
			missing = append(missing, item)
		}
	}

	return missing
}
