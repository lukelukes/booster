package condition

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
		{
			name:    "empty content",
			content: "",
			want:    "",
		},
		{
			name:    "arch linux",
			content: "NAME=\"Arch Linux\"\nID=arch\nPRETTY_NAME=\"Arch Linux\"\n",
			want:    "arch",
		},
		{
			name:    "ubuntu",
			content: "NAME=\"Ubuntu\"\nVERSION=\"22.04.3 LTS\"\nID=ubuntu\nID_LIKE=debian\n",
			want:    "ubuntu",
		},
		{
			name:    "fedora",
			content: "NAME=\"Fedora Linux\"\nID=fedora\nVERSION_ID=39\n",
			want:    "fedora",
		},
		{
			name:    "debian",
			content: "PRETTY_NAME=\"Debian GNU/Linux 12\"\nNAME=\"Debian GNU/Linux\"\nID=debian\n",
			want:    "debian",
		},
		{
			name:    "quoted ID",
			content: "ID=\"fedora\"\n",
			want:    "fedora",
		},
		{
			name:    "single quoted ID",
			content: "ID='manjaro'\n",
			want:    "manjaro",
		},
		{
			name:    "ID not present",
			content: "NAME=Foo\nVERSION=1.0\n",
			want:    "",
		},
		{
			name:    "ID with trailing whitespace",
			content: "ID=arch  \n",
			want:    "arch",
		},
		{
			name:    "ID first in file",
			content: "ID=arch\nNAME=Arch\n",
			want:    "arch",
		},
		{
			name:    "ID_LIKE does not match",
			content: "ID_LIKE=debian\n",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseOSReleaseContent(tt.content)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSystemDetector_Detect_WithMockedFileReader(t *testing.T) {
	tests := []struct {
		name        string
		fileContent string
		fileError   error
		wantOS      string
	}{
		{
			name:        "arch linux",
			fileContent: "NAME=\"Arch Linux\"\nID=arch\nPRETTY_NAME=\"Arch Linux\"\n",
			fileError:   nil,
			wantOS:      "arch",
		},
		{
			name:        "ubuntu",
			fileContent: "NAME=\"Ubuntu\"\nVERSION=\"22.04.3 LTS\"\nID=ubuntu\nID_LIKE=debian\n",
			fileError:   nil,
			wantOS:      "ubuntu",
		},
		{
			name:        "fedora",
			fileContent: "NAME=\"Fedora Linux\"\nID=fedora\nVERSION_ID=39\n",
			fileError:   nil,
			wantOS:      "fedora",
		},
		{
			name:        "debian",
			fileContent: "PRETTY_NAME=\"Debian GNU/Linux 12\"\nNAME=\"Debian GNU/Linux\"\nID=debian\n",
			fileError:   nil,
			wantOS:      "debian",
		},
		{
			name:        "file doesn't exist - fallback to GOOS",
			fileContent: "",
			fileError:   errors.New("file not found"),
			wantOS:      runtime.GOOS,
		},
		{
			name:        "empty file - fallback to GOOS",
			fileContent: "",
			fileError:   nil,
			wantOS:      runtime.GOOS,
		},
		{
			name:        "file with no ID field - fallback to GOOS",
			fileContent: "NAME=\"Unknown\"\nVERSION=\"1.0\"\n",
			fileError:   nil,
			wantOS:      runtime.GOOS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &SystemDetector{
				ReadFile: func(path string) ([]byte, error) {
					if tt.fileError != nil {
						return nil, tt.fileError
					}
					return []byte(tt.fileContent), nil
				},
			}

			ctx := d.Detect()
			assert.Equal(t, tt.wantOS, ctx.OS)
		})
	}
}

func TestSystemDetector_Detect_OnDarwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("This test validates darwin-specific behavior and requires macOS")
	}

	readFileCalled := false
	d := &SystemDetector{
		ReadFile: func(path string) ([]byte, error) {
			readFileCalled = true

			return []byte("ID=ubuntu\n"), nil
		},
	}

	ctx := d.Detect()
	assert.Equal(t, "darwin", ctx.OS, "darwin systems should return 'darwin' as OS identifier")
	assert.False(t, readFileCalled, "ReadFile should not be called on darwin - OS detection short-circuits")
}

func TestSystemDetector_Detect_DefaultBehavior(t *testing.T) {
	d := &SystemDetector{}
	ctx := d.Detect()

	assert.NotEmpty(t, ctx.OS)

	if runtime.GOOS == "darwin" {
		assert.Equal(t, "darwin", ctx.OS)
	}
}
