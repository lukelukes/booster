package action

import (
	"booster/internal/model"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTemplateRender(t *testing.T, source, target string) *TemplateRender {
	t.Helper()

	return &TemplateRender{
		Files: []SourceTarget{{Source: source, Target: target}},
		Context: TemplateContext{
			Vars:   map[string]any{"email": "me@example.com"},
			System: TemplateSystem{OS: "arch", Profile: "personal"},
		},
	}
}

func TestTemplateRender_Check(t *testing.T) {
	t.Run("reports a missing target as a create", func(t *testing.T) {
		dir := t.TempDir()
		source := writeFile(t, filepath.Join(dir, "gitconfig.tmpl"), "email = {{ .Vars.email }}")
		target := filepath.Join(dir, "out", "gitconfig")

		change, err := newTemplateRender(t, source, target).Check(context.Background())

		require.NoError(t, err)
		assert.Equal(t, model.ChangeCreate, change.Kind)
		assert.Equal(t, 1, change.Total)
		assert.Equal(t, []model.Item{{Name: target}}, change.Items)
	})

	t.Run("reports no change when the target is up to date", func(t *testing.T) {
		dir := t.TempDir()
		source := writeFile(t, filepath.Join(dir, "gitconfig.tmpl"), "email = {{ .Vars.email }}")
		target := writeFile(t, filepath.Join(dir, "gitconfig"), "email = me@example.com")

		change, err := newTemplateRender(t, source, target).Check(context.Background())

		require.NoError(t, err)
		assert.Equal(t, model.ChangeNone, change.Kind)
		assert.Empty(t, change.Items)
	})

	t.Run("reports differing content as an update", func(t *testing.T) {
		dir := t.TempDir()
		source := writeFile(t, filepath.Join(dir, "gitconfig.tmpl"), "email = {{ .Vars.email }}")
		target := writeFile(t, filepath.Join(dir, "gitconfig"), "email = old@example.com")

		change, err := newTemplateRender(t, source, target).Check(context.Background())

		require.NoError(t, err)
		assert.Equal(t, model.ChangeUpdate, change.Kind)
		assert.Equal(t, []model.Item{{Name: target}}, change.Items)
	})

	t.Run("fails when the template is missing", func(t *testing.T) {
		dir := t.TempDir()

		_, err := newTemplateRender(t, filepath.Join(dir, "missing.tmpl"), filepath.Join(dir, "out")).
			Check(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "read template")
	})

	t.Run("fails when the template is malformed", func(t *testing.T) {
		dir := t.TempDir()
		source := writeFile(t, filepath.Join(dir, "bad.tmpl"), "{{ .Vars.email")

		_, err := newTemplateRender(t, source, filepath.Join(dir, "out")).Check(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse template")
	})
}

func TestTemplateRender_Apply(t *testing.T) {
	dir := t.TempDir()
	source := writeFile(t, filepath.Join(dir, "gitconfig.tmpl"),
		"email = {{ .Vars.email }}\nos = {{ .System.OS }}\nprofile = {{ .System.Profile }}")
	target := filepath.Join(dir, "nested", "gitconfig")

	action := newTemplateRender(t, source, target)

	require.NoError(t, action.Apply(context.Background()))

	content, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "email = me@example.com\nos = arch\nprofile = personal", string(content))

	change, err := action.Check(context.Background())
	require.NoError(t, err)
	assert.Equal(t, model.ChangeNone, change.Kind)
}

func TestNewTemplateRender(t *testing.T) {
	args := argsNode(t, `
- source: templates/gitconfig.tmpl
  target: ${ vars.config_dir }/git/config
`)

	created, err := NewTemplateRender(args, testContext())

	require.NoError(t, err)

	render := created.(*TemplateRender)
	assert.Equal(t, []SourceTarget{{
		Source: "templates/gitconfig.tmpl",
		Target: "/home/tester/.config/git/config",
	}}, render.Files)
	assert.Equal(t, TemplateContext{
		Vars:   map[string]any{"config_dir": "/home/tester/.config"},
		System: TemplateSystem{OS: "arch", Profile: "personal"},
	}, render.Context)
}
