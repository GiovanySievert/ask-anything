package embedding_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/giovanysievert/ask-anything/internal/embedding"
)

func ollamaURL() string {
	if v := os.Getenv("OLLAMA_URL"); v != "" {
		return v
	}
	return "http://localhost:11434"
}

func TestEmbed_ReturnsExpectedDimensions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := embedding.New(ollamaURL(), "nomic-embed-text")

	vec, err := client.Embed(context.Background(), "react native flatlist performance")
	require.NoError(t, err)
	require.Len(t, vec, 768)
}

func TestEmbed_SimilarTextsAreCloser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := embedding.New(ollamaURL(), "nomic-embed-text")
	ctx := context.Background()

	base, err := client.Embed(ctx, "how to optimize a slow list in react native")
	require.NoError(t, err)
	similar, err := client.Embed(ctx, "improving FlatList scroll performance")
	require.NoError(t, err)
	unrelated, err := client.Embed(ctx, "recipe for chocolate cake")
	require.NoError(t, err)

	require.Greater(t, cosine(base, unrelated), 0.0)
	require.Greater(t, cosine(base, similar), cosine(base, unrelated))
}

func cosine(a, b []float32) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (sqrt(na) * sqrt(nb))
}

func sqrt(x float64) float64 {
	if x == 0 {
		return 0
	}
	z := x
	for i := 0; i < 20; i++ {
		z = (z + x/z) / 2
	}
	return z
}
