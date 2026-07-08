package interview

import (
	"context"
	"fmt"

	"github.com/giovanysievert/ask-anything/internal/llm"
)

const topK = 5

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

type LLM interface {
	GenerateQuestion(ctx context.Context, topic, level string, contextChunks []string) (string, error)
	EvaluateAnswer(ctx context.Context, question, answer string) (llm.Evaluation, error)
}

type Service struct {
	repo     Repository
	embedder Embedder
	llm      LLM
}

func NewService(repo Repository, embedder Embedder, model LLM) *Service {
	return &Service{repo: repo, embedder: embedder, llm: model}
}

func (s *Service) GenerateQuestion(ctx context.Context, topic, level string) (Question, error) {
	embedding, err := s.embedder.Embed(ctx, topic+" "+level)
	if err != nil {
		return Question{}, fmt.Errorf("embedding topic: %w", err)
	}

	chunks, err := s.repo.SearchSimilarChunks(ctx, embedding, topK)
	if err != nil {
		return Question{}, fmt.Errorf("searching chunks: %w", err)
	}

	question, err := s.llm.GenerateQuestion(ctx, topic, level, chunks)
	if err != nil {
		return Question{}, fmt.Errorf("generating question: %w", err)
	}

	return Question{Question: question}, nil
}

func (s *Service) EvaluateAnswer(ctx context.Context, question, answer string) (Evaluation, error) {
	eval, err := s.llm.EvaluateAnswer(ctx, question, answer)
	if err != nil {
		return Evaluation{}, fmt.Errorf("evaluating answer: %w", err)
	}

	return Evaluation{
		Score:         eval.Score,
		Feedback:      eval.Feedback,
		MissingPoints: eval.MissingPoints,
		WeakTopics:    eval.WeakTopics,
		NextQuestion:  eval.NextQuestion,
	}, nil
}
