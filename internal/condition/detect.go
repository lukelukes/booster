package condition

import (
	"os"
	"runtime"
	"strings"
)

type Detector interface {
	Detect() Context
}

type SystemDetector struct {
	ReadFile func(string) ([]byte, error)
}

func (d *SystemDetector) Detect() Context {
	readFile := d.ReadFile
	if readFile == nil {
		readFile = os.ReadFile
	}
	return Context{
		OS: detectOS(readFile),
	}
}

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

func parseOSRelease(path string, readFile func(string) ([]byte, error)) string {
	data, err := readFile(path)
	if err != nil {
		return ""
	}
	return parseOSReleaseContent(string(data))
}

func parseOSReleaseContent(content string) string {
	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "ID="); ok {
			value := after

			value = strings.Trim(value, `"'`)
			return value
		}
	}
	return ""
}
