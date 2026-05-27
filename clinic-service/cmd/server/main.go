package main

import (
	"clinic-service/internal/config"
	"clinic-service/internal/db/postgres"
	"clinic-service/internal/rabbitmq"
	"clinic-service/internal/server"
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := godotenv.Load(); err != nil {
		log.Warn("no .env file, using env vars")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Error("failed to load config", slog.Any("error", err))
		os.Exit(1)
	}

	db, err := postgres.NewPostgresDB(&cfg.Postgres)
	if err != nil {
		log.Error("failed to connect to DB", slog.Any("error", err))
		os.Exit(1)
	}
	defer db.Close()

	termCtx, termCancel := context.WithCancel(context.Background())
	go waitSigterm(termCancel, log)

	rabbit, err := rabbitmq.NewRabbitMQ(&cfg.RabbitMQ, termCtx, log)
	if err != nil {
		log.Error("failed to connect to RabbitMQ", slog.Any("error", err))
		os.Exit(1)
	}
	defer rabbit.Close()

	errCh := make(chan error, 1)
	srv := server.NewServer(cfg, db, rabbit, log)

	if err := srv.Run(errCh); err != nil {
		log.Error("failed to start server", slog.Any("error", err))
		os.Exit(1)
	}

	select {
	case err := <-errCh:
		log.Error("server error", slog.Any("error", err))
	case <-termCtx.Done():
		if err := srv.Stop(); err != nil {
			log.Error("graceful shutdown failed", slog.Any("error", err))
		}
	}

	log.Info("service terminated")
}

func waitSigterm(terminate context.CancelFunc, log *slog.Logger) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Warn("received termination signal")
	signal.Stop(sigCh)
	close(sigCh)
	terminate()
}