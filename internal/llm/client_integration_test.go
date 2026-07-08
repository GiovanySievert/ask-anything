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

func TestGenerateQuestion(t *testing.T) {
	client := newClient(t)

	question, err := client.GenerateQuestion(context.Background(), "react native", "senior", nil)
	require.NoError(t, err)
	require.NotEmpty(t, question)
}

func TestEvaluateAnswer(t *testing.T) {
	client := newClient(t)

	eval, err := client.EvaluateAnswer(
		context.Background(),
		"How would you optimize a slow FlatList in React Native?",
		"I would use getItemLayout, keyExtractor, and windowSize tuning, plus memoized row components.",
	)
	require.NoError(t, err)
	require.GreaterOrEqual(t, eval.Score, 0)
	require.LessOrEqual(t, eval.Score, 10)
	require.NotEmpty(t, eval.Feedback)
	require.NotEmpty(t, eval.NextQuestion)
}
