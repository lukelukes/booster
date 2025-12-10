package tui

import (
	"booster/internal/variable"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromptCollector_CollectWithInput(t *testing.T) {
	// Simulate user typing "Alice\n" for the name field
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
	// Test with defaults - just press enter for each field
	// This avoids complexity of huh's multi-field input handling
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
	// Just press enter (empty input uses default)
	input := strings.NewReader("\n")

	collector := NewPromptCollector().WithInput(input)

	defs := []variable.Definition{
		{Name: "Name", Prompt: "Your name", Default: "DefaultName"},
	}

	result, err := collector.Collect(defs)

	require.NoError(t, err)
	// Default should be preserved when user just presses enter
	assert.Equal(t, "DefaultName", result["Name"])
}
