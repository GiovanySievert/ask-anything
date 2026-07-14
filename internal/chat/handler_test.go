package chat

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func newTestHandler(repo Repository, llmFake LLM) *Handler {
	svc := NewService(repo, fakeEmbedder{vec: []float32{0.1}}, llmFake)
	return NewHandler(svc, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func newRouter(h *Handler) chi.Router {
	r := chi.NewRouter()
	h.RegisterJSONRoutes(r)
	h.RegisterStreamRoutes(r)
	return r
}

func TestCreateConversation(t *testing.T) {
	h := newTestHandler(newFakeRepo(), &fakeLLM{})
	r := newRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/conversations", strings.NewReader(`{"title":"study"}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	var conv Conversation
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &conv))
	require.Equal(t, "study", conv.Title)
}

func TestListMessages_NotFound(t *testing.T) {
	h := newTestHandler(newFakeRepo(), &fakeLLM{})
	r := newRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/conversations/"+uuid.New().String()+"/messages", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestPostMessage_StreamsSSE(t *testing.T) {
	repo := newFakeRepo()
	conv, _ := repo.CreateConversation(context.Background(), "t")
	h := newTestHandler(repo, &fakeLLM{deltas: []string{"po", "ng"}})
	r := newRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/conversations/"+conv.ID.String()+"/messages", strings.NewReader(`{"content":"ping"}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))

	body := rec.Body.String()
	require.Contains(t, body, `event: delta`)
	require.Contains(t, body, `"text":"po"`)
	require.Contains(t, body, `"text":"ng"`)
	require.Contains(t, body, `event: done`)
}

func TestPostMessage_InvalidBody(t *testing.T) {
	repo := newFakeRepo()
	conv, _ := repo.CreateConversation(context.Background(), "t")
	h := newTestHandler(repo, &fakeLLM{})
	r := newRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/conversations/"+conv.ID.String()+"/messages", strings.NewReader(`{"content":""}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}
