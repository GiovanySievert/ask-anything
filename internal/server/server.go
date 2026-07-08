package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/giovanysievert/ask-anything/internal/config"
	"github.com/giovanysievert/ask-anything/internal/database"
	"github.com/giovanysievert/ask-anything/internal/embedding"
	"github.com/giovanysievert/ask-anything/internal/llm"
)

type Server struct {
	httpServer *http.Server
	logger     *slog.Logger
}

func New(cfg *config.Config, db *database.DB, logger *slog.Logger, llmClient *llm.Client, embedClient *embedding.Client) *Server {
	handler := newRouter(logger, db, llmClient, embedClient)

	return &Server{
		logger: logger,
		httpServer: &http.Server{
			Addr:    fmt.Sprintf(":%d", cfg.Port),
			Handler: handler,

			ReadTimeout:       10 * time.Second,
			ReadHeaderTimeout: 5 * time.Second,
			WriteTimeout:      90 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
	}
}

func (s *Server) Start(ctx context.Context) error {

	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("server listening", slog.String("addr", s.httpServer.Addr))
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		s.logger.Info("shutdown signal received, draining connections")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(shutdownCtx)
	}
}
