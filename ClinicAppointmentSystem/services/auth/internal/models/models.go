package models

import "time"

type User struct {
	ID           string    `db:"id" json:"id"`
	Login        string    `db:"login" json:"login"`
	PasswordHash string    `db:"password_hash" json:"-"`
	FullName     string    `db:"full_name" json:"fullName"`
	Phone        string    `db:"phone" json:"phone,omitempty"`
	Email        string    `db:"email" json:"email,omitempty"`
	Role         string    `db:"role" json:"role"`
	IsActive     bool      `db:"is_active" json:"isActive"`
	CreatedAt    time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt    time.Time `db:"updated_at" json:"updatedAt"`
}

type Session struct {
	SessionID string    `db:"session_id" json:"sessionId"`
	UserID    string    `db:"user_id" json:"userId"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
	ExpiresAt time.Time `db:"expires_at" json:"expiresAt"`
}

type RegisterRequest struct {
	Login    string `json:"login" validate:"required,min=3,max=50"`
	Password string `json:"password" validate:"required,min=6"`
	FullName string `json:"fullName" validate:"required"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
}

type LoginRequest struct {
	Login    string `json:"login" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  *User  `json:"user"`
}

type ValidateRequest struct {
	Token string `json:"token" validate:"required"`
}

type ValidateResponse struct {
	Valid  bool   `json:"valid"`
	UserID string `json:"userId,omitempty"`
	Role   string `json:"role,omitempty"`
}
