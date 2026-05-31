package service

import (
	"clinic-service/internal/db/postgres"
	"clinic-service/internal/models"
	"clinic-service/internal/rabbitmq"
	"clinic-service/internal/utils"
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
	s.log.Info("creating appointment", 
		slog.String("user_id", app.UserID),
		slog.String("doctor", app.Doctor),
		slog.Time("date", app.Date))

	// Валидация выходных
	if app.Date.Weekday() == time.Saturday || app.Date.Weekday() == time.Sunday {
		utils.RecordSlotValidationError("weekend")
		return errors.New("запись на выходные невозможна, работаем Пн-Пт")
	}

	// Валидация прошедшей даты
	now := time.Now()
	if app.Date.Before(now) {
		utils.RecordSlotValidationError("past_date")
		return errors.New("нельзя записаться на прошедшую дату")
	}

	// Валидация: нельзя записаться менее чем за 1 час
	if app.Date.Sub(now) < time.Hour {
		utils.RecordSlotValidationError("too_soon")
		return errors.New("запись возможна не менее чем за 1 час до приема")
	}

	// Валидация рабочих часов
	if app.Date.Hour() < 9 || app.Date.Hour() >= 18 {
		utils.RecordSlotValidationError("outside_working_hours")
		return errors.New("запись возможна только в рабочие часы (9:00-18:00)")
	}

	// Валидация: минуты должны быть 00 (только целые часы)
	if app.Date.Minute() != 0 || app.Date.Second() != 0 {
		utils.RecordSlotValidationError("invalid_time_format")
		return errors.New("запись возможна только на начало часа (например, 10:00, 11:00)")
	}

	// Проверка занятости слота
	start := time.Now()
	available, err := s.repo.IsSlotAvailable(ctx, app.Doctor, app.Date)
	utils.RecordDBQuery("check_slot_availability", time.Since(start))
	
	if err != nil {
		s.log.Error("failed to check slot availability", slog.Any("error", err))
		return err
	}
	if !available {
		utils.RecordSlotValidationError("slot_occupied")
		return errors.New("выбранное время уже занято, выберите другое")
	}

	// Создание записи в БД
	start = time.Now()
	if err := s.repo.CreateAppointment(ctx, app); err != nil {
		s.log.Error("failed to create appointment in DB", slog.Any("error", err))
		return err
	}
	utils.RecordDBQuery("create_appointment", time.Since(start))

	s.log.Info("appointment created in DB", 
		slog.String("id", app.ID),
		slog.String("user_id", app.UserID))

	// Записываем бизнес-метрику
	utils.RecordAppointmentCreated()

	// Публикация в RabbitMQ
	if s.rabbit != nil {
		if err := s.rabbit.Publish("appointments.new", []byte(app.ID)); err != nil {
			s.log.Error("failed to publish to RabbitMQ", slog.Any("error", err))
			// Не возвращаем ошибку, так как запись уже создана в БД
		} else {
			utils.RecordRabbitMQPublish("appointments.new")
		}
	}

	return nil
}

func (s *AppointmentService) ListByUser(ctx context.Context, userID string) ([]models.Appointment, error) {
	start := time.Now()
	apps, err := s.repo.GetAppointmentsByUser(ctx, userID)
	utils.RecordDBQuery("list_appointments", time.Since(start))
	
	if err != nil {
		s.log.Error("failed to list appointments", 
			slog.String("user_id", userID),
			slog.Any("error", err))
		return nil, err
	}
	
	return apps, nil
}

func (s *AppointmentService) GetDoctors(ctx context.Context) ([]models.Doctor, error) {
	start := time.Now()
	doctors, err := s.repo.GetAllDoctors(ctx)
	utils.RecordDBQuery("get_doctors", time.Since(start))
	
	if err != nil {
		s.log.Error("failed to get doctors", slog.Any("error", err))
		return nil, err
	}
	
	return doctors, nil
}

func (s *AppointmentService) GetAvailableSlots(ctx context.Context, doctor string, date time.Time) ([]models.SlotInfo, error) {
	// Валидация выходных
	if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
		utils.RecordSlotValidationError("weekend")
		return nil, errors.New("в выходные не работаем")
	}

	// Валидация прошедшей даты
	if date.Before(time.Now().AddDate(0, 0, -1)) {
		utils.RecordSlotValidationError("past_date")
		return nil, errors.New("нельзя выбрать прошедшую дату")
	}

	start := time.Now()
	slots, err := s.repo.GetAvailableSlots(ctx, doctor, date)
	utils.RecordDBQuery("get_available_slots", time.Since(start))
	
	if err != nil {
		s.log.Error("failed to get available slots",
			slog.String("doctor", doctor),
			slog.Time("date", date),
			slog.Any("error", err))
		return nil, err
	}

	var result []models.SlotInfo
	now := time.Now()
	for _, slot := range slots {
		// Показываем только будущие слоты (с запасом в 1 час)
		if slot.After(now.Add(time.Hour)) {
			result = append(result, models.SlotInfo{
				Doctor:    doctor,
				DateTime:  slot.Format(time.RFC3339),
				Available: true,
			})
		}
	}
	
	return result, nil
}

func (s *AppointmentService) ProcessAppointment(ctx context.Context, appointmentID string) error {
	s.log.Info("processing appointment", slog.String("id", appointmentID))

	// Этап 1: pending -> processing
	start := time.Now()
	if err := s.repo.UpdateAppointmentStatus(ctx, appointmentID, "processing"); err != nil {
		s.log.Error("failed to update to processing",
			slog.String("id", appointmentID),
			slog.Any("error", err))
		utils.RecordRabbitMQConsume("appointments.new", "failed")
		return err
	}
	utils.RecordDBQuery("update_status_processing", time.Since(start))
	utils.AppointmentsProcessed.WithLabelValues("processing").Inc()

	s.log.Info("status updated to processing", slog.String("id", appointmentID))

	// Публикуем уведомление
	if s.rabbit != nil {
		if err := s.rabbit.Publish("appointments.status", []byte(`{"id":"`+appointmentID+`","status":"processing"}`)); err != nil {
			s.log.Warn("failed to publish status update", slog.Any("error", err))
		}
	}

	// Этап 2: processing -> completed
	start = time.Now()
	if err := s.repo.UpdateAppointmentStatus(ctx, appointmentID, "completed"); err != nil {
		s.log.Error("failed to update to completed",
			slog.String("id", appointmentID),
			slog.Any("error", err))
		return err
	}
	utils.RecordDBQuery("update_status_completed", time.Since(start))
	utils.AppointmentsProcessed.WithLabelValues("completed").Inc()

	s.log.Info("status updated to completed", slog.String("id", appointmentID))

	// Публикуем уведомление
	if s.rabbit != nil {
		if err := s.rabbit.Publish("appointments.status", []byte(`{"id":"`+appointmentID+`","status":"completed"}`)); err != nil {
			s.log.Warn("failed to publish status update", slog.Any("error", err))
		}
	}

	utils.RecordRabbitMQConsume("appointments.new", "success")
	return nil
}