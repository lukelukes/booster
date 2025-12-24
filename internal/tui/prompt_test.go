package tui

import (
	"booster/internal/variable"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromptCollector_CollectWithInput(t *testing.T) {
	input := strings.NewReader("Alice\n")

	collector := NewPromptCollector().WithInput(input)

	defs := []variable.Definition{
		{Name: "Name", Prompt: "Your name"},
	}

	result, err := collector.Collect(defs)

	require.NoError(t, err)
	assert.Equal(t, "Alice", result["Name"])
}

func TestPromptCollector_CollectMultipleFields(t *testing.T) {
	input := strings.NewReader("\n\n")

	collector := NewPromptCollector().WithInput(input)

	defs := []variable.Definition{
		{Name: "Name", Prompt: "Your name", Default: "Alice"},
		{Name: "Email", Prompt: "Your email", Default: "alice@example.com"},
	}

	result, err := collector.Collect(defs)

	require.NoError(t, err)
	assert.Equal(t, "Alice", result["Name"])
	assert.Equal(t, "alice@example.com", result["Email"])
}

func TestPromptCollector_CollectEmptyDefinitions(t *testing.T) {
	collector := NewPromptCollector()

	result, err := collector.Collect(nil)

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestPromptCollector_UsesDefaultWhenEmpty(t *testing.T) {
	input := strings.NewReader("\n")

	collector := NewPromptCollector().WithInput(input)

	defs := []variable.Definition{
		{Name: "Name", Prompt: "Your name", Default: "DefaultName"},
	}

	result, err := collector.Collect(defs)

	require.NoError(t, err)

	assert.Equal(t, "DefaultName", result["Name"])
}
