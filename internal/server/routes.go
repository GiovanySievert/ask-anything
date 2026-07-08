package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/giovanysievert/ask-anything/internal/database"
	"github.com/giovanysievert/ask-anything/internal/document"
	"github.com/giovanysievert/ask-anything/internal/embedding"
	"github.com/giovanysievert/ask-anything/internal/httputil"
	"github.com/giovanysievert/ask-anything/internal/interview"
	"github.com/giovanysievert/ask-anything/internal/llm"
)

func newRouter(logger *slog.Logger, db *database.DB, llmClient *llm.Client, embedClient *embedding.Client) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(requestLogger(logger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/healthz", healthHandler(db))

	documentHandler := document.NewHandler(document.NewService(document.NewRepository(db.Pool), embedClient))
	interviewHandler := interview.NewHandler(interview.NewService(interview.NewRepository(db.Queries), embedClient, llmClient))

	r.Route("/api/v1", func(r chi.Router) {
		documentHandler.RegisterRoutes(r)
		interviewHandler.RegisterRoutes(r)
	})

	return r
}

func healthHandler(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := db.Health(r.Context()); err != nil {
			httputil.WriteError(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
