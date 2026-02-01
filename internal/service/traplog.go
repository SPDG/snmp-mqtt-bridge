package service

import (
	"context"

	"snmp-mqtt-bridge/internal/domain"
	"snmp-mqtt-bridge/internal/repository"
)

// TrapLogService handles trap log business logic
type TrapLogService struct {
	repo repository.TrapLogRepository
}

// NewTrapLogService creates a new trap log service
func NewTrapLogService(repo repository.TrapLogRepository) *TrapLogService {
	return &TrapLogService{repo: repo}
}

// Create creates a new trap log entry
func (s *TrapLogService) Create(ctx context.Context, trap *domain.TrapLog) error {
	return s.repo.Create(ctx, trap)
}

// GetByID retrieves a trap log by ID
func (s *TrapLogService) GetByID(ctx context.Context, id string) (*domain.TrapLog, error) {
	return s.repo.GetByID(ctx, id)
}

// GetByDeviceID retrieves trap logs for a specific device
func (s *TrapLogService) GetByDeviceID(ctx context.Context, deviceID string, limit, offset int) ([]domain.TrapLog, error) {
	return s.repo.GetByDeviceID(ctx, deviceID, limit, offset)
}

// GetAll retrieves all trap logs with filtering
func (s *TrapLogService) GetAll(ctx context.Context, filter domain.TrapFilter) ([]domain.TrapLog, int64, error) {
	return s.repo.GetAll(ctx, filter)
}

// DeleteOlderThan deletes trap logs older than specified days
func (s *TrapLogService) DeleteOlderThan(ctx context.Context, days int) (int64, error) {
	return s.repo.DeleteOlderThan(ctx, days)
}
