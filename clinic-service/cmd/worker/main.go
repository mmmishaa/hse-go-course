package main

import (
	"clinic-service/internal/config"
	"clinic-service/internal/db/postgres"
	"clinic-service/internal/rabbitmq"
	"context"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	if err := godotenv.Load(); err != nil {
		log.Warn("no .env file")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Error("config error", slog.Any("error", err))
		os.Exit(1)
	}

	db, err := postgres.NewPostgresDB(&cfg.Postgres)
	if err != nil {
		log.Error("db error", slog.Any("error", err))
		os.Exit(1)
	}
	defer db.Close()

	repo := postgres.NewRepository(db)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Warn("worker shutting down")
		cancel()
	}()

	rabbit, err := rabbitmq.NewRabbitMQ(&cfg.RabbitMQ, ctx, log)
	if err != nil {
		log.Error("rabbitmq error", slog.Any("error", err))
		os.Exit(1)
	}
	defer rabbit.Close()

	msgs, err := rabbit.Consume("appointments.new")
	if err != nil {
		log.Error("consume error", slog.Any("error", err))
		os.Exit(1)
	}

	log.Info("worker started, waiting for messages...")

	for {
		select {
		case <-ctx.Done():
			log.Info("worker stopped gracefully")
			return
		case msg, ok := <-msgs:
			if !ok {
				log.Warn("message channel closed")
				return
			}
			
			id := strings.TrimSpace(string(msg.Body))
			id = strings.Trim(id, "\"")
			
			log.Info("received message", slog.String("clean_id", id))

			if err := repo.UpdateAppointmentStatus(ctx, id, "processing"); err != nil {
				log.Error("update to processing failed", slog.String("id", id), slog.Any("error", err))
				msg.Nack(false, true)
				continue
			}
			log.Info("status changed", slog.String("id", id), slog.String("status", "processing"))
			rabbit.Publish("appointments.status", []byte(`{"appointment_id":"`+id+`","status":"processing"}`))

			time.Sleep(time.Duration(2+rand.Intn(3)) * time.Second)
			
			if err := repo.UpdateAppointmentStatus(ctx, id, "completed"); err != nil {
				log.Error("update to completed failed", slog.String("id", id), slog.Any("error", err))
				msg.Nack(false, true)
				continue
			}
			log.Info("status changed", slog.String("id", id), slog.String("status", "completed"))
			rabbit.Publish("appointments.status", []byte(`{"appointment_id":"`+id+`","status":"completed"}`))
			
			msg.Ack(false)
			log.Info("appointment fully processed", slog.String("id", id))
		}
	}
}