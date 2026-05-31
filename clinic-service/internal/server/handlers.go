package server

import (
	"clinic-service/internal/db/postgres"
	"clinic-service/internal/middleware"
	"clinic-service/internal/models"
	"clinic-service/internal/utils"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

type Handler struct {
	authSvc   AuthServiceInterface       // ← интерфейс!
	appSvc    AppointmentServiceInterface // ← интерфейс!
	healthSvc HealthServiceInterface     // ← интерфейс!
	repo      *postgres.Repository       // для /dbtest (можно тоже через интерфейс)
	log       *slog.Logger
}

// ServeIndex отдаёт главную страницу
func (h *Handler) ServeIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, "./static/index.html")
}

// Test - эндпоинт ЛР1, задействует ВСЕ 3 слоя через интерфейсы:
// Handler → HealthService (интерфейс) → Repository (интерфейс) → PostgreSQL
func (h *Handler) Test(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if err := h.healthSvc.HealthCheck(r.Context()); err != nil {
		utils.RecordDBQuery("ping", time.Since(start))
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	utils.RecordDBQuery("ping", time.Since(start))
	w.Write([]byte("Hello!"))
}

// HealthCheck для Docker healthcheck
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.healthSvc.HealthCheck(ctx); err != nil {
		h.log.Error("health check failed", slog.Any("error", err))
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// DBTest - эндпоинт ЛР2
func (h *Handler) DBTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Data string `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	
	start := time.Now()
	if err := h.repo.SaveTestData(r.Context(), req.Data); err != nil {
		utils.RecordDBQuery("save_test_data", time.Since(start))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	utils.RecordDBQuery("save_test_data", time.Since(start))
	w.WriteHeader(http.StatusCreated)
}

// Register - регистрация пользователя (ЛР3)
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Phone     string `json:"phone"`
		Email     string `json:"email"`
		Password  string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request format", http.StatusBadRequest)
		return
	}
	
	start := time.Now()
	user, err := h.authSvc.Register(r.Context(), req.FirstName, req.LastName, req.Phone, req.Email, req.Password)
	utils.RecordDBQuery("register_user", time.Since(start))
	
	if err != nil {
		h.log.Warn("registration failed", slog.String("email", req.Email), slog.Any("error", err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	utils.RecordUserRegistered()
	h.log.Info("user registered", 
		slog.String("user_id", user.ID),
		slog.String("email", user.Email))
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

// Login - авторизация (ЛР3)
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	
	start := time.Now()
	token, err := h.authSvc.Login(r.Context(), req.Email, req.Password)
	utils.RecordDBQuery("login", time.Since(start))
	
	if err != nil {
		utils.RecordFailedLogin()
		h.log.Warn("login failed", slog.String("email", req.Email), slog.Any("error", err))
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	
	h.log.Info("user logged in", slog.String("email", req.Email))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

// CreateAppointment - создание записи (ЛР4)
func (h *Handler) CreateAppointment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID := middleware.GetUserID(r)
	var req struct {
		Doctor           string `json:"doctor"`
		DoctorSpeciality string `json:"doctor_speciality"`
		DoctorOffice     string `json:"doctor_office"`
		Date             string `json:"date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request format", http.StatusBadRequest)
		return
	}
	
	var date time.Time
	var err error
	date, err = time.Parse(time.RFC3339, req.Date)
	if err != nil {
		date, err = time.Parse("2006-01-02T15:04", req.Date)
		if err != nil {
			http.Error(w, "invalid date format", http.StatusBadRequest)
			return
		}
	}
	
	app := &models.Appointment{
		UserID:           userID,
		Doctor:           req.Doctor,
		DoctorSpeciality: req.DoctorSpeciality,
		DoctorOffice:     req.DoctorOffice,
		Date:             date,
		Status:           "pending",
	}
	
	if err := h.appSvc.Create(r.Context(), app); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	h.log.Info("appointment created",
		slog.String("appointment_id", app.ID),
		slog.String("user_id", userID),
		slog.String("doctor", app.Doctor))
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(app)
}

// ListAppointments - просмотр записей (ЛР4)
func (h *Handler) ListAppointments(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	apps, err := h.appSvc.ListByUser(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if apps == nil {
		apps = []models.Appointment{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apps)
}

// GetDoctors - список врачей
func (h *Handler) GetDoctors(w http.ResponseWriter, r *http.Request) {
	doctors, err := h.appSvc.GetDoctors(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(doctors)
}

// GetAvailableSlots - свободные слоты
func (h *Handler) GetAvailableSlots(w http.ResponseWriter, r *http.Request) {
	doctor := r.URL.Query().Get("doctor")
	dateStr := r.URL.Query().Get("date")
	
	if doctor == "" || dateStr == "" {
		http.Error(w, "doctor and date are required", http.StatusBadRequest)
		return
	}
	
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		http.Error(w, "invalid date format, use YYYY-MM-DD", http.StatusBadRequest)
		return
	}
	
	slots, err := h.appSvc.GetAvailableSlots(r.Context(), doctor, date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(slots)
}