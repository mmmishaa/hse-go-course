package service

import (
	"clinic-service/internal/models"
	"context"
	"time"
)

// UserRepository - интерфейс для работы с пользователями
// Repository реализует этот интерфейс автоматически (Go duck typing)
type UserRepository interface {
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	RegisterUser(ctx context.Context, firstName, lastName, phone, email, password string) (*models.User, error)
}

// AppointmentRepository - интерфейс для работы с записями
type AppointmentRepository interface {
	CreateAppointment(ctx context.Context, app *models.Appointment) error
	GetAppointmentsByUser(ctx context.Context, userID string) ([]models.Appointment, error)
	UpdateAppointmentStatus(ctx context.Context, id, status string) error
	GetAppointmentByID(ctx context.Context, id string) (*models.Appointment, error)
	IsSlotAvailable(ctx context.Context, doctor string, date time.Time) (bool, error)
	GetAllDoctors(ctx context.Context) ([]models.Doctor, error)
	GetAvailableSlots(ctx context.Context, doctor string, date time.Time) ([]time.Time, error)
}

// HealthRepository - интерфейс для проверки здоровья БД
type HealthRepository interface {
	Ping(ctx context.Context) error
}

// TestRepository - интерфейс для тестовых данных (ЛР2)
type TestRepository interface {
	SaveTestData(ctx context.Context, data string) error
}