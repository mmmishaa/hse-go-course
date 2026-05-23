package domain

import (
	"context"
	"net/http"

	"clinic-auth/internal/models"
)

type Repository interface {
	CreateUser(ctx context.Context, user *models.User, password string) error
	GetUserByLogin(ctx context.Context, login string) (*models.User, error)
	GetUserByID(ctx context.Context, id string) (*models.User, error)
	UpdateUser(ctx context.Context, user *models.User) error

	CreateSession(ctx context.Context, session *models.Session) error
	GetSessionByUserID(ctx context.Context, userID string) (*models.Session, error)
	UpdateSessionExpiry(ctx context.Context, sessionID string) error
	DeleteSession(ctx context.Context, sessionID string) error
}

type UseCase interface {
	Register(ctx context.Context, req *models.RegisterRequest) (*models.User, error)
	Login(ctx context.Context, req *models.LoginRequest) (*models.LoginResponse, error)
	ValidateToken(token string) (userID string, role string, err error)
	Logout(ctx context.Context, userID string) error
}

type Handler interface {
	Register() http.HandlerFunc
	Login() http.HandlerFunc
	Logout() http.HandlerFunc
	Validate() http.HandlerFunc
}
