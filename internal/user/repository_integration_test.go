package user_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/giovanysievert/ask-anything/internal/database/db"
	"github.com/giovanysievert/ask-anything/internal/user"
)


func setupRepo(t *testing.T) user.Repository {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("5432/tcp").WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pgContainer.Terminate(ctx) })

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	applyMigrations(t, ctx, pool)

	return user.NewRepository(db.New(pool))
}

func applyMigrations(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	pattern := filepath.Join("..", "..", "db", "migrations", "*.up.sql")
	files, err := filepath.Glob(pattern)
	require.NoError(t, err)
	require.NotEmpty(t, files, "no migration files found")

	for _, f := range files {
		sqlBytes, err := os.ReadFile(f)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sqlBytes))
		require.NoError(t, err, "applying migration %s", f)
	}
}

func TestRepository_CreateAndGet(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, "ada@example.com", "Ada Lovelace")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, created.ID)
	require.Equal(t, "ada@example.com", created.Email)
	require.False(t, created.CreatedAt.IsZero())

	got, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)
	require.Equal(t, "Ada Lovelace", got.Name)
}

func TestRepository_Get_NotFound(t *testing.T) {
	repo := setupRepo(t)

	_, err := repo.GetByID(context.Background(), uuid.New())
	require.ErrorIs(t, err, user.ErrNotFound)
}

func TestRepository_Create_DuplicateEmail(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	_, err := repo.Create(ctx, "dup@example.com", "First")
	require.NoError(t, err)

	_, err = repo.Create(ctx, "dup@example.com", "Second")
	require.ErrorIs(t, err, user.ErrEmailTaken)
}

func TestRepository_UpdateAndDelete(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, "old@example.com", "Old Name")
	require.NoError(t, err)

	updated, err := repo.Update(ctx, created.ID, "New Name", "new@example.com")
	require.NoError(t, err)
	require.Equal(t, "New Name", updated.Name)
	require.Equal(t, "new@example.com", updated.Email)

	err = repo.Delete(ctx, created.ID)
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, created.ID)
	require.ErrorIs(t, err, user.ErrNotFound)
}

func TestRepository_List(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	for _, email := range []string{"a@x.com", "b@x.com", "c@x.com"} {
		_, err := repo.Create(ctx, email, "User")
		require.NoError(t, err)
	}

	users, err := repo.List(ctx, 2, 0)
	require.NoError(t, err)
	require.Len(t, users, 2)
}
