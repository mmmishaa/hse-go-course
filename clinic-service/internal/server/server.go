package server

import (
	"clinic-service/internal/config"
	"clinic-service/internal/db/postgres"
	"clinic-service/internal/middleware"
	"clinic-service/internal/rabbitmq"
	"clinic-service/internal/service"
	"clinic-service/internal/utils"
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	cfg     *config.Config
	httpSrv *http.Server
	handler *Handler
	log     *slog.Logger
}

func NewServer(cfg *config.Config, db *sqlx.DB, rabbit *rabbitmq.Client, log *slog.Logger) *Server {
	repo := postgres.NewRepository(db)

	// Создаём конкретные реализации сервисов
	authService := service.NewAuthService(repo, cfg.JWTSecret)
	appointmentService := service.NewAppointmentService(repo, rabbit, log)
	healthService := service.NewHealthService(repo)

	// Передаём их в Handler как ИНТЕРФЕЙСЫ
	// Go автоматически приводит типы, так как реализации удовлетворяют интерфейсам
	h := &Handler{
		authSvc:   authService,       // AuthService → AuthServiceInterface
		appSvc:    appointmentService, // AppointmentService → AppointmentServiceInterface
		healthSvc: healthService,     // HealthService → HealthServiceInterface
		repo:      repo,
		log:       log,
	}

	mux := http.NewServeMux()

	// Статика и фронтенд
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	mux.HandleFunc("/", h.ServeIndex)

	// Публичные эндпоинты
	mux.HandleFunc("/test", h.Test)
	mux.HandleFunc("/health", h.HealthCheck)
	mux.HandleFunc("/dbtest", h.DBTest)
	mux.HandleFunc("/auth/register", h.Register)
	mux.HandleFunc("/auth/login", h.Login)
	mux.HandleFunc("/doctors", h.GetDoctors)
	mux.HandleFunc("/slots", h.GetAvailableSlots)

	// Защищенные эндпоинты с JWT middleware
	authMW := middleware.AuthMiddleware(cfg.JWTSecret)
	mux.Handle("/appointments/create", authMW(http.HandlerFunc(h.CreateAppointment)))
	mux.Handle("/appointments/list", authMW(http.HandlerFunc(h.ListAppointments)))

	// Метрики Prometheus
	mux.Handle("/metrics", promhttp.Handler())

	// Оборачиваем в middleware для сбора метрик
	wrapped := utils.PrometheusMiddleware(mux)

	return &Server{
		cfg:     cfg,
		handler: h,
		log:     log,
		httpSrv: &http.Server{
			Addr:         ":" + cfg.Server.Port,
			Handler:      wrapped,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}
}

func (s *Server) Run(errCh chan<- error) error {
	s.log.Info("starting HTTP server", slog.String("addr", s.httpSrv.Addr))
	go func() {
		if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	return nil
}

func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s.log.Info("shutting down HTTP server")
	return s.httpSrv.Shutdown(ctx)
}