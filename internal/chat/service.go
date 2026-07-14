package chat

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/giovanysievert/ask-anything/internal/llm"
)

const topK = 5

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

type LLM interface {
	StreamChat(ctx context.Context, history []llm.ChatMessage, contextChunks []string, onDelta func(string) error) (string, error)
}

type Service struct {
	repo     Repository
	embedder Embedder
	llm      LLM
}

func NewService(repo Repository, embedder Embedder, model LLM) *Service {
	return &Service{repo: repo, embedder: embedder, llm: model}
}

func (s *Service) CreateConversation(ctx context.Context, title string) (Conversation, error) {
	if title == "" {
		title = "New chat"
	}
	return s.repo.CreateConversation(ctx, title)
}

func (s *Service) ListConversations(ctx context.Context, limit, offset int32) ([]Conversation, error) {
	return s.repo.ListConversations(ctx, limit, offset)
}

func (s *Service) ListMessages(ctx context.Context, conversationID uuid.UUID) ([]Message, error) {
	if _, err := s.repo.GetConversation(ctx, conversationID); err != nil {
		return nil, err
	}
	return s.repo.ListMessages(ctx, conversationID)
}

func (s *Service) StreamTurn(ctx context.Context, conversationID uuid.UUID, userContent string, onDelta func(string) error) (Message, error) {
	if _, err := s.repo.GetConversation(ctx, conversationID); err != nil {
		return Message{}, err
	}

	if _, err := s.repo.AppendMessage(ctx, conversationID, "user", userContent); err != nil {
		return Message{}, fmt.Errorf("persisting user message: %w", err)
	}

	embedding, err := s.embedder.Embed(ctx, userContent)
	if err != nil {
		return Message{}, fmt.Errorf("embedding message: %w", err)
	}

	chunks, err := s.repo.SearchSimilarChunks(ctx, embedding, topK)
	if err != nil {
		return Message{}, fmt.Errorf("searching chunks: %w", err)
	}

	history, err := s.repo.ListMessages(ctx, conversationID)
	if err != nil {
		return Message{}, fmt.Errorf("loading history: %w", err)
	}

	full, err := s.llm.StreamChat(ctx, toLLMHistory(history), chunks, onDelta)
	if err != nil {
		return Message{}, err
	}

	assistant, err := s.repo.AppendMessage(ctx, conversationID, "assistant", full)
	if err != nil {
		return Message{}, fmt.Errorf("persisting assistant message: %w", err)
	}

	return assistant, nil
}

func toLLMHistory(messages []Message) []llm.ChatMessage {
	out := make([]llm.ChatMessage, len(messages))
	for i, m := range messages {
		out[i] = llm.ChatMessage{Role: m.Role, Content: m.Content}
	}
	return out
}
