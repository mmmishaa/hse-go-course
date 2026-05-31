package utils

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP метрики
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	// Бизнес-метрики
	AppointmentsCreated = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "appointments_created_total",
			Help: "Total number of appointments created",
		},
	)

	AppointmentsProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "appointments_processed_total",
			Help: "Total number of appointments processed by status",
		},
		[]string{"status"},
	)

	UsersRegistered = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "users_registered_total",
			Help: "Total number of users registered",
		},
	)

	FailedLogins = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "failed_logins_total",
			Help: "Total number of failed login attempts",
		},
	)

	SlotValidationErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "slot_validation_errors_total",
			Help: "Total number of slot validation errors by type",
		},
		[]string{"error_type"},
	)

	// Метрики RabbitMQ
	RabbitMQMessagesPublished = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rabbitmq_messages_published_total",
			Help: "Total number of messages published to RabbitMQ",
		},
		[]string{"queue"},
	)

	RabbitMQMessagesConsumed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rabbitmq_messages_consumed_total",
			Help: "Total number of messages consumed from RabbitMQ",
		},
		[]string{"queue", "status"},
	)

	// Метрики базы данных
	DBQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
		[]string{"operation"},
	)

	ActiveConnections = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "active_connections",
			Help: "Number of active connections",
		},
		[]string{"type"},
	)
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func PrometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{w, http.StatusOK}
		
		ActiveConnections.WithLabelValues("http").Inc()
		defer ActiveConnections.WithLabelValues("http").Dec()
		
		next.ServeHTTP(rw, r)
		
		duration := time.Since(start).Seconds()
		httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, strconv.Itoa(rw.statusCode)).Inc()
		httpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
	})
}

// RecordAppointmentCreated записывает метрику создания записи
func RecordAppointmentCreated() {
	AppointmentsCreated.Inc()
}

// RecordUserRegistered записывает метрику регистрации
func RecordUserRegistered() {
	UsersRegistered.Inc()
}

// RecordFailedLogin записывает метрику неудачного входа
func RecordFailedLogin() {
	FailedLogins.Inc()
}

// RecordSlotValidationError записывает метрику ошибки валидации слота
func RecordSlotValidationError(errorType string) {
	SlotValidationErrors.WithLabelValues(errorType).Inc()
}

// RecordRabbitMQPublish записывает метрику публикации в RabbitMQ
func RecordRabbitMQPublish(queue string) {
	RabbitMQMessagesPublished.WithLabelValues(queue).Inc()
}

// RecordRabbitMQConsume записывает метрику потребления из RabbitMQ
func RecordRabbitMQConsume(queue, status string) {
	RabbitMQMessagesConsumed.WithLabelValues(queue, status).Inc()
}

// RecordDBQuery записывает метрику запроса к БД
func RecordDBQuery(operation string, duration time.Duration) {
	DBQueryDuration.WithLabelValues(operation).Observe(duration.Seconds())
}