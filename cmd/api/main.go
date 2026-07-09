package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"github.com/giovanysievert/ask-anything/internal/config"
	"github.com/giovanysievert/ask-anything/internal/database"
	"github.com/giovanysievert/ask-anything/internal/embedding"
	"github.com/giovanysievert/ask-anything/internal/llm"
	"github.com/giovanysievert/ask-anything/internal/server"

	_ "github.com/giovanysievert/ask-anything/docs"
)

// @title           ask-anything API
// @version         1.0
// @description     AI-powered technical-interview API. Ingest documents, then generate RAG-grounded interview questions and evaluate candidate answers.
// @BasePath        /api/v1
func main() {
	if err := run(); err != nil {
		slog.Error("fatal error", slog.Any("err", err))
		os.Exit(1)
	}
}

func run() error {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := newLogger(cfg.Env)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	logger.Info("connected to database")

	llmClient := llm.New(cfg.AnthropicAPIKey, cfg.LLMModel)
	embedClient := embedding.New(cfg.OllamaURL, cfg.EmbeddingModel)

	srv := server.New(cfg, db, logger, llmClient, embedClient)
	return srv.Start(ctx)
}

func newLogger(env string) *slog.Logger {
	if env == "production" {
		return slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
}
