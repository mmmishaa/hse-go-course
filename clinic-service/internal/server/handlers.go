package server

import (
	"clinic-service/internal/db/postgres"
	"clinic-service/internal/middleware"
	"clinic-service/internal/models"
	"clinic-service/internal/service"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

type Handler struct {
	authSvc *service.AuthService
	appSvc  *service.AppointmentService
	repo    *postgres.Repository
	log     *slog.Logger
}

func (h *Handler) ServeIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, "./static/index.html")
}

func (h *Handler) Test(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.Ping(r.Context()); err != nil {
		http.Error(w, "db unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Write([]byte("Hello!"))
}

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
	if err := h.repo.SaveTestData(r.Context(), req.Data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

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
	user, err := h.authSvc.Register(r.Context(), req.FirstName, req.LastName, req.Phone, req.Email, req.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

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
	token, err := h.authSvc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

func (h *Handler) CreateAppointment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID := middleware.GetUserID(r)
	var req struct {
		Doctor string `json:"doctor"`
		Date   string `json:"date"`
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
		UserID: userID,
		Doctor: req.Doctor,
		Date:   date,
		Status: "pending",
	}
	if err := h.appSvc.Create(r.Context(), app); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(app)
}

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

func (h *Handler) GetDoctors(w http.ResponseWriter, r *http.Request) {
	doctors, err := h.appSvc.GetDoctors(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(doctors)
}

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