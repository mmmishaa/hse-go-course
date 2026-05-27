package service

import (
	"clinic-service/internal/db/postgres"
	"clinic-service/internal/models"
	"clinic-service/internal/rabbitmq"
	"context"
	"errors"
	"log/slog"
	"time"
)

type AppointmentService struct {
	repo   *postgres.Repository
	rabbit *rabbitmq.Client
	log    *slog.Logger
}

func NewAppointmentService(repo *postgres.Repository, rabbit *rabbitmq.Client, log *slog.Logger) *AppointmentService {
	return &AppointmentService{repo: repo, rabbit: rabbit, log: log}
}

func (s *AppointmentService) Create(ctx context.Context, app *models.Appointment) error {
	if app.Date.Weekday() == time.Saturday || app.Date.Weekday() == time.Sunday {
		return errors.New("запись на выходные невозможна, работаем Пн-Пт")
	}
	if app.Date.Before(time.Now()) {
		return errors.New("нельзя записаться на прошедшую дату")
	}
	if app.Date.Hour() < 9 || app.Date.Hour() >= 18 {
		return errors.New("запись возможна только в рабочие часы (9:00-18:00)")
	}

	available, err := s.repo.IsSlotAvailable(ctx, app.Doctor, app.Date)
	if err != nil {
		return err
	}
	if !available {
		return errors.New("выбранное время уже занято, выберите другое")
	}

	if err := s.repo.CreateAppointment(ctx, app); err != nil {
		return err
	}

	if s.rabbit != nil {
		if err := s.rabbit.Publish("appointments.new", []byte(app.ID)); err != nil {
			s.log.Error("publish failed", slog.Any("error", err))
		}
	}
	return nil
}

func (s *AppointmentService) ListByUser(ctx context.Context, userID string) ([]models.Appointment, error) {
	return s.repo.GetAppointmentsByUser(ctx, userID)
}

func (s *AppointmentService) GetDoctors(ctx context.Context) ([]models.Doctor, error) {
	return s.repo.GetAllDoctors(ctx)
}

func (s *AppointmentService) GetAvailableSlots(ctx context.Context, doctor string, date time.Time) ([]models.SlotInfo, error) {
	if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
		return nil, errors.New("в выходные не работаем")
	}
	if date.Before(time.Now().AddDate(0, 0, -1)) {
		return nil, errors.New("нельзя выбрать прошедшую дату")
	}

	slots, err := s.repo.GetAvailableSlots(ctx, doctor, date)
	if err != nil {
		return nil, err
	}

	var result []models.SlotInfo
	for _, slot := range slots {
		if slot.After(time.Now()) {
			result = append(result, models.SlotInfo{
				Doctor:    doctor,
				DateTime:  slot.Format(time.RFC3339),
				Available: true,
			})
		}
	}
	return result, nil
}