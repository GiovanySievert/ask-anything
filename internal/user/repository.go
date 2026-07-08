package user

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/giovanysievert/ask-anything/internal/database/db"
)

const uniqueViolationCode = "23505"

type Repository interface {
	Create(ctx context.Context, email, name string) (User, error)
	GetByID(ctx context.Context, id uuid.UUID) (User, error)
	List(ctx context.Context, limit, offset int32) ([]User, error)
	Update(ctx context.Context, id uuid.UUID, name, email string) (User, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type pgRepository struct {
	q db.Querier
}

func NewRepository(q db.Querier) Repository {
	return &pgRepository{q: q}
}

func (r *pgRepository) Create(ctx context.Context, email, name string) (User, error) {
	row, err := r.q.CreateUser(ctx, db.CreateUserParams{Email: email, Name: name})
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrEmailTaken
		}
		return User{}, err
	}
	return toDomain(row), nil
}

func (r *pgRepository) GetByID(ctx context.Context, id uuid.UUID) (User, error) {
	row, err := r.q.GetUser(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	return toDomain(row), nil
}

func (r *pgRepository) List(ctx context.Context, limit, offset int32) ([]User, error) {
	rows, err := r.q.ListUsers(ctx, db.ListUsersParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, err
	}
	users := make([]User, len(rows))
	for i, row := range rows {
		users[i] = toDomain(row)
	}
	return users, nil
}

func (r *pgRepository) Update(ctx context.Context, id uuid.UUID, name, email string) (User, error) {
	row, err := r.q.UpdateUser(ctx, db.UpdateUserParams{ID: id, Name: name, Email: email})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		if isUniqueViolation(err) {
			return User{}, ErrEmailTaken
		}
		return User{}, err
	}
	return toDomain(row), nil
}

func (r *pgRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.q.DeleteUser(ctx, id)
}

func toDomain(row db.User) User {
	return User{
		ID:        row.ID,
		Email:     row.Email,
		Name:      row.Name,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode
}
