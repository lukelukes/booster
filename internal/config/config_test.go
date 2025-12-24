package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		checkValid func(*testing.T, *Config)
		name       string
		content    string
		wantErr    string
	}{
		{
			name: "valid config",
			content: `version: "1"
tasks:
  - action: dir.create
    args:
      - ~/.config/test
`,
			checkValid: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "1", cfg.Version)
				assert.Len(t, cfg.Tasks, 1)
				assert.Equal(t, "dir.create", cfg.Tasks[0].Action)
			},
		},
		{
			name: "missing version",
			content: `tasks:
  - action: dir.create
    args: []
`,
			wantErr: "missing version",
		},
		{
			name: "unsupported version",
			content: `version: "2"
tasks: []
`,
			wantErr: "unsupported config version",
		},
		{
			name: "invalid YAML",
			content: `version: "1"
tasks:
  - action: [invalid yaml structure
`,
			wantErr: "parse config",
		},
		{
			name: "empty tasks",
			content: `version: "1"
tasks: []
`,
			checkValid: func(t *testing.T, cfg *Config) {
				assert.Empty(t, cfg.Tasks)
			},
		},
		{
			name: "multiple tasks",
			content: `version: "1"
tasks:
  - action: dir.create
    args:
      - ~/.config/app1
  - action: dir.create
    args:
      - ~/.config/app2
`,
			checkValid: func(t *testing.T, cfg *Config) {
				assert.Len(t, cfg.Tasks, 2)
			},
		},
		{
			name: "empty action string",
			content: `version: "1"
tasks:
  - action: ""
    args: []
`,
			wantErr: "task 1: action cannot be empty",
		},
		{
			name: "empty action in second task",
			content: `version: "1"
tasks:
  - action: dir.create
    args:
      - ~/.config/test
  - action: ""
    args: []
  - action: dir.create
    args:
      - ~/.config/test2
`,
			wantErr: "task 2: action cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			configPath := filepath.Join(dir, "config.yaml")
			require.NoError(t, os.WriteFile(configPath, []byte(tt.content), 0o644))

			cfg, err := Load(configPath)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			if tt.checkValid != nil {
				tt.checkValid(t, cfg)
			}
		})
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	nonexistentPath := filepath.Join(dir, "definitely-does-not-exist.yaml")

	_, err := Load(nonexistentPath)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "read config")
}

func TestLoad_WhenCondition(t *testing.T) {
	tests := []struct {
		check   func(*testing.T, *Config)
		name    string
		content string
	}{
		{
			name: "single OS string",
			content: `version: "1"
tasks:
  - action: dir.create
    when:
      os: "arch"
    args:
      - ~/test
`,
			check: func(t *testing.T, cfg *Config) {
				require.Len(t, cfg.Tasks, 1)
				require.NotNil(t, cfg.Tasks[0].When)
				assert.Equal(t, StringOrSlice{"arch"}, cfg.Tasks[0].When.OS)
			},
		},
		{
			name: "multiple OS array",
			content: `version: "1"
tasks:
  - action: dir.create
    when:
      os: ["arch", "darwin"]
    args:
      - ~/test
`,
			check: func(t *testing.T, cfg *Config) {
				require.Len(t, cfg.Tasks, 1)
				require.NotNil(t, cfg.Tasks[0].When)
				assert.Equal(t, StringOrSlice{"arch", "darwin"}, cfg.Tasks[0].When.OS)
			},
		},
		{
			name: "no when condition",
			content: `version: "1"
tasks:
  - action: dir.create
    args:
      - ~/test
`,
			check: func(t *testing.T, cfg *Config) {
				require.Len(t, cfg.Tasks, 1)
				assert.Nil(t, cfg.Tasks[0].When)
			},
		},
		{
			name: "empty when",
			content: `version: "1"
tasks:
  - action: dir.create
    when: {}
    args:
      - ~/test
`,
			check: func(t *testing.T, cfg *Config) {
				require.Len(t, cfg.Tasks, 1)
				require.NotNil(t, cfg.Tasks[0].When)
				assert.Empty(t, cfg.Tasks[0].When.OS)
			},
		},
		{
			name: "mixed tasks with and without when",
			content: `version: "1"
tasks:
  - action: dir.create
    when:
      os: "arch"
    args:
      - ~/arch-only
  - action: dir.create
    args:
      - ~/everywhere
`,
			check: func(t *testing.T, cfg *Config) {
				require.Len(t, cfg.Tasks, 2)
				require.NotNil(t, cfg.Tasks[0].When)
				assert.Equal(t, StringOrSlice{"arch"}, cfg.Tasks[0].When.OS)
				assert.Nil(t, cfg.Tasks[1].When)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			configPath := filepath.Join(dir, "config.yaml")
			require.NoError(t, os.WriteFile(configPath, []byte(tt.content), 0o644))

			cfg, err := Load(configPath)
			require.NoError(t, err)
			tt.check(t, cfg)
		})
	}
}

func TestStringOrSlice_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		want    StringOrSlice
		wantErr bool
	}{
		{
			name: "single string",
			yaml: `os: arch`,
			want: StringOrSlice{"arch"},
		},
		{
			name: "array",
			yaml: `os: [arch, darwin]`,
			want: StringOrSlice{"arch", "darwin"},
		},
		{
			name: "multiline array",
			yaml: `os:
  - arch
  - ubuntu
  - fedora`,
			want: StringOrSlice{"arch", "ubuntu", "fedora"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w When
			err := yaml.Unmarshal([]byte(tt.yaml), &w)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, w.OS)
		})
	}
}

func TestLoad_Profiles(t *testing.T) {
	tests := []struct {
		check   func(*testing.T, *Config)
		name    string
		content string
	}{
		{
			name: "config with profiles list",
			content: `version: "1"
profiles:
  - personal
  - work
tasks: []
`,
			check: func(t *testing.T, cfg *Config) {
				require.Len(t, cfg.Profiles, 2)
				assert.Equal(t, []string{"personal", "work"}, cfg.Profiles)
			},
		},
		{
			name: "config without profiles",
			content: `version: "1"
tasks: []
`,
			check: func(t *testing.T, cfg *Config) {
				assert.Nil(t, cfg.Profiles)
			},
		},
		{
			name: "empty profiles section",
			content: `version: "1"
profiles: []
tasks: []
`,
			check: func(t *testing.T, cfg *Config) {
				assert.Empty(t, cfg.Profiles)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			configPath := filepath.Join(dir, "config.yaml")
			require.NoError(t, os.WriteFile(configPath, []byte(tt.content), 0o644))

			cfg, err := Load(configPath)
			require.NoError(t, err)
			tt.check(t, cfg)
		})
	}
}

func TestLoad_WhenProfileCondition(t *testing.T) {
	tests := []struct {
		check   func(*testing.T, *Config)
		name    string
		content string
	}{
		{
			name: "single profile string",
			content: `version: "1"
tasks:
  - action: dir.create
    when:
      profile: "personal"
    args:
      - ~/test
`,
			check: func(t *testing.T, cfg *Config) {
				require.Len(t, cfg.Tasks, 1)
				require.NotNil(t, cfg.Tasks[0].When)
				assert.Equal(t, StringOrSlice{"personal"}, cfg.Tasks[0].When.Profile)
			},
		},
		{
			name: "multiple profiles array",
			content: `version: "1"
tasks:
  - action: dir.create
    when:
      profile: ["personal", "work"]
    args:
      - ~/test
`,
			check: func(t *testing.T, cfg *Config) {
				require.Len(t, cfg.Tasks, 1)
				require.NotNil(t, cfg.Tasks[0].When)
				assert.Equal(t, StringOrSlice{"personal", "work"}, cfg.Tasks[0].When.Profile)
			},
		},
		{
			name: "combined OS and profile conditions",
			content: `version: "1"
tasks:
  - action: dir.create
    when:
      os: "arch"
      profile: "work"
    args:
      - ~/test
`,
			check: func(t *testing.T, cfg *Config) {
				require.Len(t, cfg.Tasks, 1)
				require.NotNil(t, cfg.Tasks[0].When)
				assert.Equal(t, StringOrSlice{"arch"}, cfg.Tasks[0].When.OS)
				assert.Equal(t, StringOrSlice{"work"}, cfg.Tasks[0].When.Profile)
			},
		},
		{
			name: "profile without OS",
			content: `version: "1"
tasks:
  - action: dir.create
    when:
      profile: "personal"
    args:
      - ~/test
`,
			check: func(t *testing.T, cfg *Config) {
				require.Len(t, cfg.Tasks, 1)
				require.NotNil(t, cfg.Tasks[0].When)
				assert.Empty(t, cfg.Tasks[0].When.OS)
				assert.Equal(t, StringOrSlice{"personal"}, cfg.Tasks[0].When.Profile)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			configPath := filepath.Join(dir, "config.yaml")
			require.NoError(t, os.WriteFile(configPath, []byte(tt.content), 0o644))

			cfg, err := Load(configPath)
			require.NoError(t, err)
			tt.check(t, cfg)
		})
	}
}

func TestLoad_Variables(t *testing.T) {
	tests := []struct {
		check   func(*testing.T, *Config)
		name    string
		content string
	}{
		{
			name: "config with variables",
			content: `version: "1"
variables:
  Name:
    prompt: "Your full name"
  Email:
    prompt: "Your email"
    default: "user@example.com"
tasks: []
`,
			check: func(t *testing.T, cfg *Config) {
				require.Len(t, cfg.Variables, 2)

				name, ok := cfg.Variables["Name"]
				require.True(t, ok)
				assert.Equal(t, "Your full name", name.Prompt)
				assert.Empty(t, name.Default)

				email, ok := cfg.Variables["Email"]
				require.True(t, ok)
				assert.Equal(t, "Your email", email.Prompt)
				assert.Equal(t, "user@example.com", email.Default)
			},
		},
		{
			name: "config without variables",
			content: `version: "1"
tasks: []
`,
			check: func(t *testing.T, cfg *Config) {
				assert.Nil(t, cfg.Variables)
			},
		},
		{
			name: "empty variables section",
			content: `version: "1"
variables: {}
tasks: []
`,
			check: func(t *testing.T, cfg *Config) {
				assert.Empty(t, cfg.Variables)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			configPath := filepath.Join(dir, "config.yaml")
			require.NoError(t, os.WriteFile(configPath, []byte(tt.content), 0o644))

			cfg, err := Load(configPath)
			require.NoError(t, err)
			tt.check(t, cfg)
		})
	}
}
