package expr

import (
	"os"
	"runtime"
	"strings"
)

func detectOS() string {
	if runtime.GOOS == "darwin" {
		return "darwin"
	}

	if runtime.GOOS == "linux" {
		if distro := parseOSRelease("/etc/os-release", os.ReadFile); distro != "" {
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
			value := strings.TrimSpace(after)
			value = strings.Trim(value, `"'`)
			return value
		}
	}
	return ""
}
