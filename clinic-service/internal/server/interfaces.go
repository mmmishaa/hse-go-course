package server

import (
	"clinic-service/internal/models"
	"context"
	"time"
)

// AuthServiceInterface - интерфейс для аутентификации
// Handler зависит от интерфейса, а не от конкретной реализации
type AuthServiceInterface interface {
	Register(ctx context.Context, firstName, lastName, phone, email, password string) (*models.User, error)
	Login(ctx context.Context, email, password string) (string, error)
}

// AppointmentServiceInterface - интерфейс для работы с записями
type AppointmentServiceInterface interface {
	Create(ctx context.Context, app *models.Appointment) error
	ListByUser(ctx context.Context, userID string) ([]models.Appointment, error)
	GetDoctors(ctx context.Context) ([]models.Doctor, error)
	GetAvailableSlots(ctx context.Context, doctor string, date time.Time) ([]models.SlotInfo, error)
}

// HealthServiceInterface - интерфейс для проверки здоровья
type HealthServiceInterface interface {
	HealthCheck(ctx context.Context) error
}