package document

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"

	"github.com/giovanysievert/ask-anything/internal/database/db"
)

type ChunkInput struct {
	Content   string
	Embedding []float32
}

type Repository interface {
	CreateWithChunks(ctx context.Context, title string, chunks []ChunkInput) (Document, error)
	List(ctx context.Context, limit, offset int32) ([]Document, error)
}

type pgRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

func (r *pgRepository) CreateWithChunks(ctx context.Context, title string, chunks []ChunkInput) (Document, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Document{}, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := db.New(tx)

	row, err := qtx.CreateDocument(ctx, title)
	if err != nil {
		return Document{}, fmt.Errorf("creating document: %w", err)
	}

	for i, chunk := range chunks {
		_, err := qtx.CreateChunk(ctx, db.CreateChunkParams{
			DocumentID: row.ID,
			Content:    chunk.Content,
			Embedding:  pgvector.NewVector(chunk.Embedding),
		})
		if err != nil {
			return Document{}, fmt.Errorf("creating chunk %d: %w", i, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return Document{}, fmt.Errorf("committing transaction: %w", err)
	}

	return toDomain(row), nil
}

func (r *pgRepository) List(ctx context.Context, limit, offset int32) ([]Document, error) {
	rows, err := db.New(r.pool).ListDocuments(ctx, db.ListDocumentsParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, err
	}
	docs := make([]Document, len(rows))
	for i, row := range rows {
		docs[i] = toDomain(row)
	}
	return docs, nil
}

func toDomain(row db.Document) Document {
	return Document{
		ID:        row.ID,
		Title:     row.Title,
		CreatedAt: row.CreatedAt,
	}
}
