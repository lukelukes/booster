package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func loadConfigFromContent(t *testing.T, content string) (*Config, error) {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		return nil, err
	}
	return Load(configPath)
}

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
			cfg, err := loadConfigFromContent(t, tt.content)

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
		wantErr string
	}{
		{
			name: "expression when string",
			content: `version: "1"
tasks:
  - action: dir.create
    when: ${ os == "arch" }
    args:
      - ~/test
`,
			check: func(t *testing.T, cfg *Config) {
				require.Len(t, cfg.Tasks, 1)
				require.NotNil(t, cfg.Tasks[0].When)
				assert.Equal(t, `${ os == "arch" }`, string(*cfg.Tasks[0].When))
			},
		},
		{
			name: "mapping when is rejected",
			content: `version: "1"
tasks:
  - action: dir.create
    when:
      os: "arch"
    args:
      - ~/test
`,
			wantErr: "when must be an expression string in `${ ... }` form",
		},
		{
			name: "no when",
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
			name: "non-expression when string is rejected",
			content: `version: "1"
tasks:
  - action: dir.create
    when: arch
    args:
      - ~/test
`,
			wantErr: "when must be an expression string in `${ ... }` form",
		},
		{
			name: "empty when string is rejected",
			content: `version: "1"
tasks:
  - action: dir.create
    when: "   "
    args:
      - ~/test
`,
			wantErr: "when cannot be empty",
		},
		{
			name: "malformed when missing closing brace is rejected",
			content: `version: "1"
tasks:
  - action: dir.create
    when: '${ os == "arch" '
    args:
      - ~/test
`,
			wantErr: "when must be an expression string in `${ ... }` form",
		},
		{
			name: "mixed tasks with and without when",
			content: `version: "1"
tasks:
  - action: dir.create
    when: ${ os == "arch" }
    args:
      - ~/arch-only
  - action: dir.create
    args:
      - ~/everywhere
`,
			check: func(t *testing.T, cfg *Config) {
				require.Len(t, cfg.Tasks, 2)
				require.NotNil(t, cfg.Tasks[0].When)
				assert.Equal(t, `${ os == "arch" }`, string(*cfg.Tasks[0].When))
				assert.Nil(t, cfg.Tasks[1].When)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := loadConfigFromContent(t, tt.content)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

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
		{name: "single string", yaml: `value: arch`, want: StringOrSlice{"arch"}},
		{name: "array", yaml: `value: [arch, darwin]`, want: StringOrSlice{"arch", "darwin"}},
		{name: "multiline array", yaml: "value:\n  - arch\n  - ubuntu\n  - fedora", want: StringOrSlice{"arch", "ubuntu", "fedora"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v struct {
				Value StringOrSlice `yaml:"value"`
			}
			err := yaml.Unmarshal([]byte(tt.yaml), &v)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, v.Value)
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
			cfg, err := loadConfigFromContent(t, tt.content)
			require.NoError(t, err)
			tt.check(t, cfg)
		})
	}
}

func TestLoad_WhenExpressionsForProfileAndOS(t *testing.T) {
	tests := []struct {
		check   func(*testing.T, *Config)
		name    string
		content string
		wantErr string
	}{
		{
			name: "profile expression",
			content: `version: "1"
tasks:
  - action: dir.create
    when: ${ profile == "personal" }
    args:
      - ~/test
`,
			check: func(t *testing.T, cfg *Config) {
				require.Len(t, cfg.Tasks, 1)
				require.NotNil(t, cfg.Tasks[0].When)
				assert.Equal(t, `${ profile == "personal" }`, string(*cfg.Tasks[0].When))
			},
		},
		{
			name: "combined expression",
			content: `version: "1"
tasks:
  - action: dir.create
    when: ${ os == "arch" and profile in ["personal", "work"] }
    args:
      - ~/test
`,
			check: func(t *testing.T, cfg *Config) {
				require.Len(t, cfg.Tasks, 1)
				require.NotNil(t, cfg.Tasks[0].When)
				assert.Equal(t, `${ os == "arch" and profile in ["personal", "work"] }`, string(*cfg.Tasks[0].When))
			},
		},
		{
			name: "combined mapping when is rejected",
			content: `version: "1"
tasks:
  - action: dir.create
    when:
      os: ["arch", "darwin"]
      profile: "work"
    args:
      - ~/test
`,
			wantErr: "when must be an expression string in `${ ... }` form",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := loadConfigFromContent(t, tt.content)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
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
			cfg, err := loadConfigFromContent(t, tt.content)
			require.NoError(t, err)
			tt.check(t, cfg)
		})
	}
}
