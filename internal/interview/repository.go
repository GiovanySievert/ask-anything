package interview

import (
	"context"

	"github.com/pgvector/pgvector-go"

	"github.com/giovanysievert/ask-anything/internal/database/db"
)

type Repository interface {
	SearchSimilarChunks(ctx context.Context, embedding []float32, topK int32) ([]string, error)
}

type pgRepository struct {
	q db.Querier
}

func NewRepository(q db.Querier) Repository {
	return &pgRepository{q: q}
}

func (r *pgRepository) SearchSimilarChunks(ctx context.Context, embedding []float32, topK int32) ([]string, error) {
	rows, err := r.q.SearchSimilarChunks(ctx, db.SearchSimilarChunksParams{
		Embedding: pgvector.NewVector(embedding),
		Limit:     topK,
	})
	if err != nil {
		return nil, err
	}
	contents := make([]string, len(rows))
	for i, row := range rows {
		contents[i] = row.Content
	}
	return contents, nil
}
