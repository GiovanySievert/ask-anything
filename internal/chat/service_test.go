package chat

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/giovanysievert/ask-anything/internal/llm"
)

type fakeRepo struct {
	conversations map[uuid.UUID]Conversation
	messages      map[uuid.UUID][]Message
	chunks        []string
	appended      []Message
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		conversations: make(map[uuid.UUID]Conversation),
		messages:      make(map[uuid.UUID][]Message),
	}
}

func (f *fakeRepo) CreateConversation(_ context.Context, title string) (Conversation, error) {
	c := Conversation{ID: uuid.New(), Title: title}
	f.conversations[c.ID] = c
	return c, nil
}

func (f *fakeRepo) GetConversation(_ context.Context, id uuid.UUID) (Conversation, error) {
	c, ok := f.conversations[id]
	if !ok {
		return Conversation{}, ErrConversationNotFound
	}
	return c, nil
}

func (f *fakeRepo) ListConversations(_ context.Context, _, _ int32) ([]Conversation, error) {
	out := make([]Conversation, 0, len(f.conversations))
	for _, c := range f.conversations {
		out = append(out, c)
	}
	return out, nil
}

func (f *fakeRepo) ListMessages(_ context.Context, conversationID uuid.UUID) ([]Message, error) {
	return f.messages[conversationID], nil
}

func (f *fakeRepo) AppendMessage(_ context.Context, conversationID uuid.UUID, role, content string) (Message, error) {
	m := Message{ID: uuid.New(), ConversationID: conversationID, Role: role, Content: content}
	f.messages[conversationID] = append(f.messages[conversationID], m)
	f.appended = append(f.appended, m)
	return m, nil
}

func (f *fakeRepo) SearchSimilarChunks(_ context.Context, _ []float32, _ int32) ([]string, error) {
	return f.chunks, nil
}

type fakeEmbedder struct {
	vec []float32
	err error
}

func (f fakeEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return f.vec, f.err
}

type fakeLLM struct {
	deltas     []string
	gotHistory []llm.ChatMessage
	gotChunks  []string
	err        error
}

func (f *fakeLLM) StreamChat(_ context.Context, history []llm.ChatMessage, chunks []string, onDelta func(string) error) (string, error) {
	f.gotHistory = history
	f.gotChunks = chunks
	if f.err != nil {
		return "", f.err
	}
	full := ""
	for _, d := range f.deltas {
		if err := onDelta(d); err != nil {
			return full, err
		}
		full += d
	}
	return full, nil
}

func TestStreamTurn_PersistsBothMessagesAndStreamsDeltas(t *testing.T) {
	repo := newFakeRepo()
	repo.chunks = []string{"reference chunk"}
	conv, _ := repo.CreateConversation(context.Background(), "t")

	llmFake := &fakeLLM{deltas: []string{"po", "ng"}}
	svc := NewService(repo, fakeEmbedder{vec: []float32{0.1, 0.2}}, llmFake)

	var streamed string
	assistant, err := svc.StreamTurn(context.Background(), conv.ID, "ping", func(d string) error {
		streamed += d
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, "pong", streamed)
	require.Equal(t, "pong", assistant.Content)
	require.Equal(t, "assistant", assistant.Role)

	require.Len(t, repo.appended, 2)
	require.Equal(t, "user", repo.appended[0].Role)
	require.Equal(t, "ping", repo.appended[0].Content)
	require.Equal(t, "assistant", repo.appended[1].Role)

	require.Equal(t, []string{"reference chunk"}, llmFake.gotChunks)
	require.Equal(t, "ping", llmFake.gotHistory[len(llmFake.gotHistory)-1].Content)
}

func TestStreamTurn_UnknownConversation(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, fakeEmbedder{}, &fakeLLM{})

	_, err := svc.StreamTurn(context.Background(), uuid.New(), "hi", func(string) error { return nil })
	require.ErrorIs(t, err, ErrConversationNotFound)
	require.Empty(t, repo.appended)
}

func TestStreamTurn_EmbedError(t *testing.T) {
	repo := newFakeRepo()
	conv, _ := repo.CreateConversation(context.Background(), "t")
	svc := NewService(repo, fakeEmbedder{err: errors.New("boom")}, &fakeLLM{})

	_, err := svc.StreamTurn(context.Background(), conv.ID, "hi", func(string) error { return nil })
	require.Error(t, err)
}
