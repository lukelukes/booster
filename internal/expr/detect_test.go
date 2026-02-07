package expr

import (
	"errors"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseOSReleaseContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "empty content", content: "", want: ""},
		{name: "arch linux", content: "NAME=\"Arch Linux\"\nID=arch\n", want: "arch"},
		{name: "ubuntu", content: "ID=ubuntu\nID_LIKE=debian\n", want: "ubuntu"},
		{name: "quoted ID", content: "ID=\"fedora\"\n", want: "fedora"},
		{name: "single quoted ID", content: "ID='manjaro'\n", want: "manjaro"},
		{name: "ID not present", content: "NAME=Foo\n", want: ""},
		{name: "ID with trailing whitespace", content: "ID=arch  \n", want: "arch"},
		{name: "ID_LIKE does not match", content: "ID_LIKE=debian\n", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseOSReleaseContent(tt.content)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseOSRelease(t *testing.T) {
	tests := []struct {
		name        string
		fileContent string
		fileError   error
		wantOS      string
	}{
		{name: "arch linux", fileContent: "ID=arch\n", wantOS: "arch"},
		{name: "ubuntu", fileContent: "ID=ubuntu\n", wantOS: "ubuntu"},
		{name: "missing file", fileError: errors.New("file not found"), wantOS: ""},
		{name: "empty file", fileContent: "", wantOS: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseOSRelease("/etc/os-release", func(path string) ([]byte, error) {
				if tt.fileError != nil {
					return nil, tt.fileError
				}
				return []byte(tt.fileContent), nil
			})
			assert.Equal(t, tt.wantOS, got)
		})
	}
}

func TestDetectOS(t *testing.T) {
	got := detectOS()
	assert.NotEmpty(t, got)

	if runtime.GOOS == "darwin" {
		assert.Equal(t, "darwin", got)
	}
}
