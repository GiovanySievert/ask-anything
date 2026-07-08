package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/giovanysievert/ask-anything/internal/database"
	"github.com/giovanysievert/ask-anything/internal/httputil"
	"github.com/giovanysievert/ask-anything/internal/user"
)

func newRouter(logger *slog.Logger, db *database.DB) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(requestLogger(logger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/healthz", healthHandler(db))

	userHandler := user.NewHandler(user.NewService(user.NewRepository(db.Queries)))

	r.Route("/api/v1", func(r chi.Router) {
		userHandler.RegisterRoutes(r)
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
