package service

import (
	"clinic-service/internal/db/postgres"
	"clinic-service/internal/models"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	repo      *postgres.Repository
	jwtSecret []byte
}

func NewAuthService(repo *postgres.Repository, secret string) *AuthService {
	return &AuthService{repo: repo, jwtSecret: []byte(secret)}
}

// Register теперь принимает 6 параметров: имя, фамилия, телефон, email, пароль
func (s *AuthService) Register(ctx context.Context, firstName, lastName, phone, email, password string) (*models.User, error) {
	if email == "" || password == "" {
		return nil, errors.New("email and password required")
	}
	existing, _ := s.repo.GetUserByEmail(ctx, email)
	if existing != nil {
		return nil, errors.New("user already exists")
	}
	return s.repo.RegisterUser(ctx, firstName, lastName, phone, email, password)
}

func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return "", err
	}
	if user == nil {
		return "", errors.New("invalid credentials")
	}
	// Исправлено: в models.User поле называется Password, а не PasswordHash
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return "", errors.New("invalid credentials")
	}
	return s.generateToken(user.ID)
}

func (s *AuthService) generateToken(userID string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	})
	t, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("token sign: %w", err)
	}
	return t, nil
}