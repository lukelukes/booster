package expr

import (
	"os"
	"runtime"
	"strings"
)

func detectOS() string {
	return detectOSWith(runtime.GOOS, os.ReadFile)
}

func detectOSWith(goos string, readFile func(string) ([]byte, error)) string {
	if goos == "darwin" {
		return "darwin"
	}

	if goos == "linux" {
		if distro := parseOSRelease("/etc/os-release", readFile); distro != "" {
			return distro
		}
	}

	return goos
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
