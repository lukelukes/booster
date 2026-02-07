package expr

import (
	"errors"
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
	tests := []struct {
		name          string
		goos          string
		fileContent   string
		fileError     error
		wantOS        string
		wantReadCalls int
	}{
		{
			name:          "linux with valid os-release returns distro ID",
			goos:          "linux",
			fileContent:   "NAME=Arch Linux\nID=arch\n",
			wantOS:        "arch",
			wantReadCalls: 1,
		},
		{
			name:          "linux with missing os-release falls back to linux",
			goos:          "linux",
			fileError:     errors.New("file not found"),
			wantOS:        "linux",
			wantReadCalls: 1,
		},
		{
			name:          "linux with invalid os-release falls back to linux",
			goos:          "linux",
			fileContent:   "NAME=Foo Linux\nID_LIKE=debian\n",
			wantOS:        "linux",
			wantReadCalls: 1,
		},
		{
			name:          "darwin short-circuits without reading os-release",
			goos:          "darwin",
			wantOS:        "darwin",
			wantReadCalls: 0,
		},
		{
			name:          "other GOOS falls back directly",
			goos:          "windows",
			wantOS:        "windows",
			wantReadCalls: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readCalls := 0
			got := detectOSWith(tt.goos, func(path string) ([]byte, error) {
				readCalls++
				if tt.fileError != nil {
					return nil, tt.fileError
				}
				return []byte(tt.fileContent), nil
			})

			assert.Equal(t, tt.wantOS, got)
			assert.Equal(t, tt.wantReadCalls, readCalls)
		})
	}
}
