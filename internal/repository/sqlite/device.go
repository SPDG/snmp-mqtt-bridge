package sqlite

import (
	"context"
	"time"

	"snmp-mqtt-bridge/internal/domain"
	"snmp-mqtt-bridge/internal/repository"

	"gorm.io/gorm"
)

type deviceRepository struct {
	db *gorm.DB
}

// NewDeviceRepository creates a new device repository
func NewDeviceRepository(db *gorm.DB) repository.DeviceRepository {
	return &deviceRepository{db: db}
}

func (r *deviceRepository) Create(ctx context.Context, device *domain.Device) error {
	return r.db.WithContext(ctx).Create(device).Error
}

func (r *deviceRepository) GetByID(ctx context.Context, id string) (*domain.Device, error) {
	var device domain.Device
	if err := r.db.WithContext(ctx).First(&device, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &device, nil
}

func (r *deviceRepository) GetAll(ctx context.Context) ([]domain.Device, error) {
	var devices []domain.Device
	if err := r.db.WithContext(ctx).Find(&devices).Error; err != nil {
		return nil, err
	}
	return devices, nil
}

func (r *deviceRepository) GetEnabled(ctx context.Context) ([]domain.Device, error) {
	var devices []domain.Device
	if err := r.db.WithContext(ctx).Where("enabled = ?", true).Find(&devices).Error; err != nil {
		return nil, err
	}
	return devices, nil
}

func (r *deviceRepository) Update(ctx context.Context, device *domain.Device) error {
	return r.db.WithContext(ctx).Save(device).Error
}

func (r *deviceRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&domain.Device{}, "id = ?", id).Error
}

func (r *deviceRepository) UpdateLastSeen(ctx context.Context, id string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&domain.Device{}).Where("id = ?", id).Update("last_seen", &now).Error
}
