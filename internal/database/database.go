package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/giovanysievert/ask-anything/internal/database/db"
)

type DB struct {
	Pool    *pgxpool.Pool
	Queries *db.Queries
}

func Connect(ctx context.Context, databaseURL string) (*DB, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("creating pgx pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return &DB{
		Pool:    pool,
		Queries: db.New(pool),
	}, nil
}

func (d *DB) Health(ctx context.Context) error {
	return d.Pool.Ping(ctx)
}

func (d *DB) Close() {
	d.Pool.Close()
}
