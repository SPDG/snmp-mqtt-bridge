package service

import (
	"context"

	"snmp-mqtt-bridge/internal/domain"
	"snmp-mqtt-bridge/internal/repository"
)

// SettingService handles setting business logic
type SettingService struct {
	repo repository.SettingRepository
}

// NewSettingService creates a new setting service
func NewSettingService(repo repository.SettingRepository) *SettingService {
	return &SettingService{repo: repo}
}

// Get retrieves a setting by key
func (s *SettingService) Get(ctx context.Context, key string) (string, error) {
	return s.repo.Get(ctx, key)
}

// Set creates or updates a setting
func (s *SettingService) Set(ctx context.Context, key, value string) error {
	return s.repo.Set(ctx, key, value)
}

// GetAll retrieves all settings
func (s *SettingService) GetAll(ctx context.Context) ([]domain.Setting, error) {
	return s.repo.GetAll(ctx)
}

// Delete deletes a setting
func (s *SettingService) Delete(ctx context.Context, key string) error {
	return s.repo.Delete(ctx, key)
}
