package postgres

import (
	"clinic-service/internal/config"
	"clinic-service/internal/models"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

type Repository struct {
	db *sqlx.DB
}

func NewPostgresDB(cfg *config.PostgresConfig) (*sqlx.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName)

	db, err := sqlx.ConnectContext(context.Background(), "postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("db connect: %w", err)
	}

	schema := `
	CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
	
	CREATE TABLE IF NOT EXISTS users(
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		first_name VARCHAR(100),
		last_name VARCHAR(100),
		phone VARCHAR(20),
		email VARCHAR(255) UNIQUE NOT NULL,
		password VARCHAR(255) NOT NULL,
		created_at TIMESTAMP DEFAULT NOW()
	);
	
	CREATE TABLE IF NOT EXISTS doctors(
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		name VARCHAR(255) UNIQUE NOT NULL,
		speciality VARCHAR(100) NOT NULL,
		office VARCHAR(10) NOT NULL,
		lunch_start INT DEFAULT 13,
		lunch_end INT DEFAULT 14
	);
	
	INSERT INTO doctors (name, speciality, office, lunch_start, lunch_end) VALUES
	('Иванов Иван Иванович', 'Терапевт', '101', 13, 14),
	('Петров Петр Петрович', 'Хирург', '205', 12, 13),
	('Сидорова Анна Михайловна', 'Кардиолог', '302', 13, 14),
	('Козлов Дмитрий Сергеевич', 'Невролог', '115', 14, 15),
	('Новикова Елена Викторовна', 'Офтальмолог', '401', 12, 13)
	ON CONFLICT (name) DO NOTHING;

	CREATE TABLE IF NOT EXISTS doctor_absences(
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		doctor_id UUID REFERENCES doctors(id) ON DELETE CASCADE,
		start_date DATE NOT NULL,
		end_date DATE NOT NULL,
		type VARCHAR(20) DEFAULT 'vacation'
	);

	CREATE TABLE IF NOT EXISTS appointments(
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		user_id UUID REFERENCES users(id) ON DELETE CASCADE,
		doctor VARCHAR(255) NOT NULL,
		date TIMESTAMP NOT NULL,
		status VARCHAR(20) DEFAULT 'pending',
		created_at TIMESTAMP DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS db_test(
		id SERIAL PRIMARY KEY,
		data TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT NOW()
	);`

	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("schema init: %w", err)
	}
	return db, nil
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	user := &models.User{}
	query := `SELECT id, first_name, last_name, phone, email, password, created_at FROM users WHERE email = $1`
	err := r.db.GetContext(ctx, user, query, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return user, nil
}

func (r *Repository) RegisterUser(ctx context.Context, firstName, lastName, phone, email, password string) (*models.User, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash failed: %w", err)
	}
	user := &models.User{}
	query := `INSERT INTO users(first_name, last_name, phone, email, password) 
			  VALUES($1, $2, $3, $4, $5) RETURNING id, created_at`
	err = r.db.QueryRowxContext(ctx, query, firstName, lastName, phone, email, string(hashed)).
		Scan(&user.ID, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	user.FirstName, user.LastName, user.Phone, user.Email = firstName, lastName, phone, email
	return user, nil
}

func (r *Repository) CreateAppointment(ctx context.Context, app *models.Appointment) error {
	query := `INSERT INTO appointments(user_id, doctor, date) VALUES($1, $2, $3) RETURNING id, created_at`
	return r.db.QueryRowxContext(ctx, query, app.UserID, app.Doctor, app.Date).
		Scan(&app.ID, &app.CreatedAt)
}

func (r *Repository) GetAppointmentsByUser(ctx context.Context, userID string) ([]models.Appointment, error) {
	var apps []models.Appointment
	query := `SELECT id, user_id, doctor, date, status, created_at FROM appointments WHERE user_id = $1 ORDER BY date DESC`
	err := r.db.SelectContext(ctx, &apps, query, userID)
	return apps, err
}

func (r *Repository) UpdateAppointmentStatus(ctx context.Context, id, status string) error {
	query := `UPDATE appointments SET status = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, status, id)
	return err
}

func (r *Repository) GetAppointmentByID(ctx context.Context, id string) (*models.Appointment, error) {
	app := &models.Appointment{}
	query := `SELECT id, user_id, doctor, date, status, created_at FROM appointments WHERE id = $1`
	err := r.db.GetContext(ctx, app, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return app, nil
}

func (r *Repository) SaveTestData(ctx context.Context, data string) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO db_test(data) VALUES($1)`, data)
	return err
}

func (r *Repository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

func (r *Repository) IsSlotAvailable(ctx context.Context, doctor string, date time.Time) (bool, error) {
	query := `SELECT COUNT(*) FROM appointments WHERE doctor = $1 AND date = $2 AND status != 'cancelled'`
	var count int
	err := r.db.QueryRowContext(ctx, query, doctor, date).Scan(&count)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

func (r *Repository) GetAllDoctors(ctx context.Context) ([]models.Doctor, error) {
	var doctors []models.Doctor
	query := `SELECT id, name, speciality, office, lunch_start, lunch_end FROM doctors ORDER BY name`
	err := r.db.SelectContext(ctx, &doctors, query)
	return doctors, err
}

func (r *Repository) GetAvailableSlots(ctx context.Context, doctorName string, date time.Time) ([]time.Time, error) {
	var doc models.Doctor
	if err := r.db.GetContext(ctx, &doc, `SELECT id, name, speciality, office, lunch_start, lunch_end FROM doctors WHERE name = $1`, doctorName); err != nil {
		return nil, fmt.Errorf("doctor not found: %w", err)
	}

	var absenceCount int
	queryAbs := `SELECT COUNT(*) FROM doctor_absences WHERE doctor_id = $1 AND start_date <= $2 AND end_date >= $2`
	r.db.GetContext(ctx, &absenceCount, queryAbs, doc.ID, date)
	if absenceCount > 0 {
		return []time.Time{}, nil
	}

	var available []time.Time
	for h := 9; h < 18; h++ {
		if h >= doc.LunchStart && h < doc.LunchEnd {
			continue
		}
		slot := time.Date(date.Year(), date.Month(), date.Day(), h, 0, 0, 0, date.Location())
		available = append(available, slot)
	}

	var bookedTimes []time.Time
	r.db.SelectContext(ctx, &bookedTimes, `SELECT date FROM appointments WHERE doctor = $1 AND DATE(date) = DATE($2) AND status != 'cancelled'`, doctorName, date)
	
	bookedMap := make(map[int]bool)
	for _, t := range bookedTimes {
		bookedMap[t.Hour()] = true
	}

	var finalSlots []time.Time
	for _, slot := range available {
		if !bookedMap[slot.Hour()] {
			finalSlots = append(finalSlots, slot)
		}
	}
	return finalSlots, nil
}