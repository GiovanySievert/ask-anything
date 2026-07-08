package user

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type fakeRepo struct {
	listLimit  int32
	listOffset int32
	users      []User
	createErr  error
}

func (f *fakeRepo) Create(_ context.Context, email, name string) (User, error) {
	if f.createErr != nil {
		return User{}, f.createErr
	}
	return User{ID: uuid.New(), Email: email, Name: name}, nil
}

func (f *fakeRepo) GetByID(_ context.Context, id uuid.UUID) (User, error) {
	for _, u := range f.users {
		if u.ID == id {
			return u, nil
		}
	}
	return User{}, ErrNotFound
}

func (f *fakeRepo) List(_ context.Context, limit, offset int32) ([]User, error) {
	f.listLimit = limit
	f.listOffset = offset
	return f.users, nil
}

func (f *fakeRepo) Update(_ context.Context, id uuid.UUID, name, email string) (User, error) {
	return User{ID: id, Name: name, Email: email}, nil
}

func (f *fakeRepo) Delete(_ context.Context, _ uuid.UUID) error { return nil }

func TestService_List_ClampsLimit(t *testing.T) {
	tests := []struct {
		name      string
		limit     int32
		wantLimit int32
	}{
		{"zero uses default", 0, defaultLimit},
		{"negative uses default", -5, defaultLimit},
		{"above max is capped", 500, maxLimit},
		{"within range is kept", 50, 50},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepo{}
			svc := NewService(repo)

			_, err := svc.List(context.Background(), tc.limit, 0)

			require.NoError(t, err)
			require.Equal(t, tc.wantLimit, repo.listLimit)
		})
	}
}

func TestService_List_NegativeOffsetBecomesZero(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	_, err := svc.List(context.Background(), 10, -3)

	require.NoError(t, err)
	require.Equal(t, int32(0), repo.listOffset)
}

func TestService_Create_PassesThrough(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	u, err := svc.Create(context.Background(), CreateInput{Email: "a@b.com", Name: "Ada"})

	require.NoError(t, err)
	require.Equal(t, "a@b.com", u.Email)
	require.Equal(t, "Ada", u.Name)
	require.NotEqual(t, uuid.Nil, u.ID)
}
