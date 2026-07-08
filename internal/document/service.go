package document

import (
	"context"
	"fmt"

	"github.com/giovanysievert/ask-anything/internal/chunking"
)

const (
	chunkSize    = 500
	chunkOverlap = 50
)

type Embedder interface {
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

type Service struct {
	repo     Repository
	embedder Embedder
}

func NewService(repo Repository, embedder Embedder) *Service {
	return &Service{repo: repo, embedder: embedder}
}

type CreateInput struct {
	Title   string
	Content string
}

func (s *Service) Create(ctx context.Context, in CreateInput) (Document, error) {
	texts := chunking.Split(in.Content, chunkSize, chunkOverlap)
	if len(texts) == 0 {
		return Document{}, fmt.Errorf("content produced no chunks")
	}

	embeddings, err := s.embedder.EmbedBatch(ctx, texts)
	if err != nil {
		return Document{}, fmt.Errorf("embedding chunks: %w", err)
	}

	chunks := make([]ChunkInput, len(texts))
	for i := range texts {
		chunks[i] = ChunkInput{Content: texts[i], Embedding: embeddings[i]}
	}

	return s.repo.CreateWithChunks(ctx, in.Title, chunks)
}

func (s *Service) List(ctx context.Context, limit, offset int32) ([]Document, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.List(ctx, limit, offset)
}
