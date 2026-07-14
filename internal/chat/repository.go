package chat

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"

	"github.com/giovanysievert/ask-anything/internal/database/db"
)

var ErrConversationNotFound = errors.New("conversation not found")

type Repository interface {
	CreateConversation(ctx context.Context, title string) (Conversation, error)
	GetConversation(ctx context.Context, id uuid.UUID) (Conversation, error)
	ListConversations(ctx context.Context, limit, offset int32) ([]Conversation, error)
	ListMessages(ctx context.Context, conversationID uuid.UUID) ([]Message, error)
	AppendMessage(ctx context.Context, conversationID uuid.UUID, role, content string) (Message, error)
	SearchSimilarChunks(ctx context.Context, embedding []float32, topK int32) ([]string, error)
}

type pgRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

func (r *pgRepository) CreateConversation(ctx context.Context, title string) (Conversation, error) {
	row, err := db.New(r.pool).CreateConversation(ctx, title)
	if err != nil {
		return Conversation{}, err
	}
	return conversationToDomain(row), nil
}

func (r *pgRepository) GetConversation(ctx context.Context, id uuid.UUID) (Conversation, error) {
	row, err := db.New(r.pool).GetConversation(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Conversation{}, ErrConversationNotFound
		}
		return Conversation{}, err
	}
	return conversationToDomain(row), nil
}

func (r *pgRepository) ListConversations(ctx context.Context, limit, offset int32) ([]Conversation, error) {
	rows, err := db.New(r.pool).ListConversations(ctx, db.ListConversationsParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, err
	}
	out := make([]Conversation, len(rows))
	for i, row := range rows {
		out[i] = conversationToDomain(row)
	}
	return out, nil
}

func (r *pgRepository) ListMessages(ctx context.Context, conversationID uuid.UUID) ([]Message, error) {
	rows, err := db.New(r.pool).ListMessages(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	out := make([]Message, len(rows))
	for i, row := range rows {
		out[i] = messageToDomain(row)
	}
	return out, nil
}

func (r *pgRepository) AppendMessage(ctx context.Context, conversationID uuid.UUID, role, content string) (Message, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Message{}, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := db.New(tx)

	row, err := qtx.CreateMessage(ctx, db.CreateMessageParams{
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
	})
	if err != nil {
		return Message{}, fmt.Errorf("creating message: %w", err)
	}

	if err := qtx.TouchConversation(ctx, conversationID); err != nil {
		return Message{}, fmt.Errorf("touching conversation: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Message{}, fmt.Errorf("committing transaction: %w", err)
	}

	return messageToDomain(row), nil
}

func (r *pgRepository) SearchSimilarChunks(ctx context.Context, embedding []float32, topK int32) ([]string, error) {
	rows, err := db.New(r.pool).SearchSimilarChunks(ctx, db.SearchSimilarChunksParams{
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

func conversationToDomain(row db.Conversation) Conversation {
	return Conversation{
		ID:        row.ID,
		Title:     row.Title,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

func messageToDomain(row db.Message) Message {
	return Message{
		ID:             row.ID,
		ConversationID: row.ConversationID,
		Role:           row.Role,
		Content:        row.Content,
		CreatedAt:      row.CreatedAt,
	}
}
