package variable

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCollector struct {
	values map[string]string
	err    error
	called bool
}

func (m *mockCollector) Collect(defs []Definition) (map[string]string, error) {
	m.called = true
	if m.err != nil {
		return nil, m.err
	}
	return m.values, nil
}

func TestResolver_Resolve(t *testing.T) {
	tests := []struct {
		name          string
		storedValues  map[string]string
		envLookup     func(string) string
		collectorVals map[string]string
		collectorErr  error
		defs          []Definition
		wantResolved  map[string]string
		wantPrompted  bool
		wantSaved     map[string]string
		wantErr       string
	}{
		{
			name:         "env takes precedence over stored and prompted values",
			storedValues: map[string]string{"Name": "stored-value"},
			envLookup: func(key string) string {
				if key == "Name" {
					return "env-value"
				}
				return ""
			},
			collectorVals: map[string]string{"Name": "prompted-value"},
			defs:          []Definition{{Name: "Name", Prompt: "Your name"}},
			wantResolved:  map[string]string{"Name": "env-value"},
			wantPrompted:  false,
			wantSaved:     map[string]string{"Name": "stored-value"},
		},
		{
			name:          "uses stored value when no env var is set",
			storedValues:  map[string]string{"Name": "stored-value"},
			envLookup:     func(key string) string { return "" },
			collectorVals: map[string]string{"Name": "prompted-value"},
			defs:          []Definition{{Name: "Name", Prompt: "Your name"}},
			wantResolved:  map[string]string{"Name": "stored-value"},
			wantPrompted:  false,
			wantSaved:     map[string]string{"Name": "stored-value"},
		},
		{
			name:          "prompts when value is missing from env and store",
			storedValues:  nil,
			envLookup:     func(key string) string { return "" },
			collectorVals: map[string]string{"Name": "prompted-value"},
			defs:          []Definition{{Name: "Name", Prompt: "Your name"}},
			wantResolved:  map[string]string{"Name": "prompted-value"},
			wantPrompted:  true,
			wantSaved:     map[string]string{"Name": "prompted-value"},
		},
		{
			name:          "applies default when collector returns empty string",
			storedValues:  nil,
			envLookup:     func(key string) string { return "" },
			collectorVals: map[string]string{"Email": ""},
			defs:          []Definition{{Name: "Email", Prompt: "Your email", Default: "default@example.com"}},
			wantResolved:  map[string]string{"Email": "default@example.com"},
			wantPrompted:  true,
			wantSaved:     map[string]string{"Email": "default@example.com"},
		},
		{
			name:          "saves prompted values to store",
			storedValues:  nil,
			envLookup:     func(key string) string { return "" },
			collectorVals: map[string]string{"Name": "Alice"},
			defs:          []Definition{{Name: "Name", Prompt: "Your name"}},
			wantResolved:  map[string]string{"Name": "Alice"},
			wantPrompted:  true,
			wantSaved:     map[string]string{"Name": "Alice"},
		},
		{
			name:         "does not save env values to store",
			storedValues: nil,
			envLookup: func(key string) string {
				if key == "Name" {
					return "from-env"
				}
				return ""
			},
			collectorVals: map[string]string{},
			defs:          []Definition{{Name: "Name", Prompt: "Your name"}},
			wantResolved:  map[string]string{"Name": "from-env"},
			wantPrompted:  false,
			wantSaved:     map[string]string{},
		},
		{
			name:          "returns error when collector fails",
			storedValues:  nil,
			envLookup:     func(key string) string { return "" },
			collectorVals: nil,
			collectorErr:  errors.New("user cancelled"),
			defs:          []Definition{{Name: "Name", Prompt: "Your name"}},
			wantErr:       "user cancelled",
		},
		{
			name:          "handles empty definitions without error",
			storedValues:  nil,
			envLookup:     func(key string) string { return "" },
			collectorVals: map[string]string{},
			defs:          nil,
			wantResolved:  map[string]string{},
			wantPrompted:  false,
			wantSaved:     map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			storePath := filepath.Join(dir, "values.yaml")
			store := NewFileStore(storePath)

			if tt.storedValues != nil {
				require.NoError(t, store.Save(tt.storedValues))
			}

			collector := &mockCollector{values: tt.collectorVals, err: tt.collectorErr}
			resolver := NewResolver(store,
				WithEnvLookup(tt.envLookup),
				WithCollector(collector),
			)

			resolved, err := resolver.Resolve(tt.defs)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantResolved, resolved)
			assert.Equal(t, tt.wantPrompted, collector.called)

			savedValues, err := store.Load()
			require.NoError(t, err)
			assert.Equal(t, tt.wantSaved, savedValues)
		})
	}
}

func TestResolver_MultipleVariables(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(filepath.Join(dir, "values.yaml"))

	require.NoError(t, store.Save(map[string]string{"Email": "stored@example.com"}))

	collector := &mockCollector{values: map[string]string{"Editor": "vim"}}

	resolver := NewResolver(store,
		WithEnvLookup(func(key string) string {
			if key == "Name" {
				return "env-name"
			}
			return ""
		}),
		WithCollector(collector),
	)

	defs := []Definition{
		{Name: "Name", Prompt: "Your name"},
		{Name: "Email", Prompt: "Your email"},
		{Name: "Editor", Prompt: "Your editor"},
	}
	resolved, err := resolver.Resolve(defs)

	require.NoError(t, err)
	assert.Equal(t, "env-name", resolved["Name"])
	assert.Equal(t, "stored@example.com", resolved["Email"])
	assert.Equal(t, "vim", resolved["Editor"])
}
