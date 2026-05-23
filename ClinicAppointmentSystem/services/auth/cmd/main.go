package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"clinic-auth/internal/config"
	"clinic-auth/internal/delivery/http"
	"clinic-auth/internal/middleware"
	"clinic-auth/internal/repository"
	"clinic-auth/internal/usecase"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := godotenv.Load(); err != nil {
		log.Warn("No .env file found, using environment variables")
	}

	cfg, err := config.New()
	if err != nil {
		log.Error("Failed to load config", slog.Any("error", err))
		os.Exit(1)
	}

	dsn := cfg.DB.DSN()
	log.Info("Connecting to database", slog.String("dsn", dsn))

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		log.Error("Failed to connect to database", slog.Any("error", err))
		os.Exit(1)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Error("Database ping failed", slog.Any("error", err))
		os.Exit(1)
	}
	log.Info("Connected to database successfully")

	repo := repository.NewAuthRepository(db, log)
	uc := usecase.NewAuthUseCase(repo, log)
	handler := http.NewAuthHandler(uc, log)
	middlewareManager := middleware.NewMiddlewareManager(log, cfg.JWTSecret)

	mux := http.NewServeMux()

	mux.HandleFunc("POST /auth/register", handler.Register())
	mux.HandleFunc("POST /auth/login", handler.Login())

	protectedMux := http.NewServeMux()
	protectedMux.HandleFunc("POST /auth/logout", handler.Logout())
	protectedMux.HandleFunc("POST /auth/validate", handler.Validate())

	mux.Handle("/auth/logout", middlewareManager.JWTMiddleware(protectedMux))
	mux.Handle("/auth/validate", middlewareManager.JWTMiddleware(protectedMux))

	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("Server starting", slog.String("port", cfg.Server.Port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Server failed", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	<-quit
	log.Info("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("Server shutdown error", slog.Any("error", err))
	}

	log.Info("Server stopped gracefully")
}
