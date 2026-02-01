package repository

import (
	"context"

	"snmp-mqtt-bridge/internal/domain"
)

// DeviceRepository defines the interface for device persistence
type DeviceRepository interface {
	Create(ctx context.Context, device *domain.Device) error
	GetByID(ctx context.Context, id string) (*domain.Device, error)
	GetAll(ctx context.Context) ([]domain.Device, error)
	GetEnabled(ctx context.Context) ([]domain.Device, error)
	Update(ctx context.Context, device *domain.Device) error
	Delete(ctx context.Context, id string) error
	UpdateLastSeen(ctx context.Context, id string) error
}

// ProfileRepository defines the interface for profile persistence
type ProfileRepository interface {
	Create(ctx context.Context, profile *domain.Profile) error
	GetByID(ctx context.Context, id string) (*domain.Profile, error)
	GetAll(ctx context.Context) ([]domain.Profile, error)
	GetBuiltin(ctx context.Context) ([]domain.Profile, error)
	GetBySysObjectID(ctx context.Context, sysOID string) (*domain.Profile, error)
	Update(ctx context.Context, profile *domain.Profile) error
	Delete(ctx context.Context, id string) error
	Upsert(ctx context.Context, profile *domain.Profile) error
}

// TrapLogRepository defines the interface for trap log persistence
type TrapLogRepository interface {
	Create(ctx context.Context, trap *domain.TrapLog) error
	GetByID(ctx context.Context, id string) (*domain.TrapLog, error)
	GetByDeviceID(ctx context.Context, deviceID string, limit, offset int) ([]domain.TrapLog, error)
	GetAll(ctx context.Context, filter domain.TrapFilter) ([]domain.TrapLog, int64, error)
	DeleteOlderThan(ctx context.Context, days int) (int64, error)
}

// SettingRepository defines the interface for settings persistence
type SettingRepository interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	GetAll(ctx context.Context) ([]domain.Setting, error)
	Delete(ctx context.Context, key string) error
}
