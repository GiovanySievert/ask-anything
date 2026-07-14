package llm_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/giovanysievert/ask-anything/internal/llm"
)

func newClient(t *testing.T) *llm.Client {
	t.Helper()

	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" || testing.Short() {
		t.Skip("skipping: ANTHROPIC_API_KEY not set or short mode")
	}
	return llm.New(key, "claude-haiku-4-5")
}

func TestStreamChat(t *testing.T) {
	client := newClient(t)

	var deltas int
	history := []llm.ChatMessage{
		{Role: "user", Content: "Reply with exactly the word: pong"},
	}
	full, err := client.StreamChat(context.Background(), history, nil, func(string) error {
		deltas++
		return nil
	})
	require.NoError(t, err)
	require.Positive(t, deltas)
	require.NotEmpty(t, full)
}
