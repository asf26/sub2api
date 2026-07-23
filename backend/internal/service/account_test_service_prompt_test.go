package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClaudeTestPayloadUsesConfiguredPrompt(t *testing.T) {
	payload, err := createTestPayload("claude-test", "reply only READY")
	require.NoError(t, err)

	messages, ok := payload["messages"].([]map[string]any)
	require.True(t, ok)
	content, ok := messages[0]["content"].([]map[string]any)
	require.True(t, ok)
	require.Equal(t, "reply only READY", content[0]["text"])
}

func TestOpenAITestPayloadUsesConfiguredPrompt(t *testing.T) {
	payload := createOpenAITestPayload("gpt-test", false, "reply only READY")
	input, ok := payload["input"].([]map[string]any)
	require.True(t, ok)
	content, ok := input[0]["content"].([]map[string]any)
	require.True(t, ok)
	require.Equal(t, "reply only READY", content[0]["text"])
}
