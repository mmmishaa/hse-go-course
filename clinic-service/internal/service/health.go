package service

import (
	"context"
)

// HealthService реализует проверку здоровья системы
// Это Service-слой, который вызывает Repository-слой через интерфейс
type HealthService struct {
	repo HealthRepository
}

func NewHealthService(repo HealthRepository) *HealthService {
	return &HealthService{repo: repo}
}

// HealthCheck проверяет доступность БД
// Цепочка: Handler → HealthService → Repository → PostgreSQL
func (s *HealthService) HealthCheck(ctx context.Context) error {
	return s.repo.Ping(ctx)
}