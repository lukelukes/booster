package condition

import (
	"os"
	"runtime"
	"strings"
)

// Detector provides runtime environment detection.
type Detector interface {
	Detect() Context
}

// SystemDetector detects the current system environment.
type SystemDetector struct {
	// ReadFile is used to read /etc/os-release. If nil, uses os.ReadFile.
	ReadFile func(string) ([]byte, error)
}

// Detect returns the current system context.
func (d *SystemDetector) Detect() Context {
	readFile := d.ReadFile
	if readFile == nil {
		readFile = os.ReadFile
	}
	return Context{
		OS: detectOS(readFile),
	}
}

// detectOS returns the OS identifier.
// On macOS, returns "darwin".
// On Linux, returns the distro ID from /etc/os-release (e.g., "arch", "ubuntu").
func detectOS(readFile func(string) ([]byte, error)) string {
	if runtime.GOOS == "darwin" {
		return "darwin"
	}

	if runtime.GOOS == "linux" {
		if distro := parseOSRelease("/etc/os-release", readFile); distro != "" {
			return distro
		}
	}

	return runtime.GOOS
}

// parseOSRelease reads /etc/os-release and returns the ID field value.
// Returns empty string if file doesn't exist or ID is not found.
func parseOSRelease(path string, readFile func(string) ([]byte, error)) string {
	data, err := readFile(path)
	if err != nil {
		return ""
	}
	return parseOSReleaseContent(string(data))
}

// parseOSReleaseContent parses the content of /etc/os-release and returns ID.
func parseOSReleaseContent(content string) string {
	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "ID="); ok {
			value := after
			// Remove quotes if present
			value = strings.Trim(value, `"'`)
			return value
		}
	}
	return ""
}
