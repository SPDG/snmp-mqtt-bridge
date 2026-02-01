package sqlite

import (
	"context"
	"time"

	"snmp-mqtt-bridge/internal/domain"
	"snmp-mqtt-bridge/internal/repository"

	"gorm.io/gorm"
)

type trapLogRepository struct {
	db *gorm.DB
}

// NewTrapLogRepository creates a new trap log repository
func NewTrapLogRepository(db *gorm.DB) repository.TrapLogRepository {
	return &trapLogRepository{db: db}
}

func (r *trapLogRepository) Create(ctx context.Context, trap *domain.TrapLog) error {
	return r.db.WithContext(ctx).Create(trap).Error
}

func (r *trapLogRepository) GetByID(ctx context.Context, id string) (*domain.TrapLog, error) {
	var trap domain.TrapLog
	if err := r.db.WithContext(ctx).First(&trap, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &trap, nil
}

func (r *trapLogRepository) GetByDeviceID(ctx context.Context, deviceID string, limit, offset int) ([]domain.TrapLog, error) {
	var traps []domain.TrapLog
	query := r.db.WithContext(ctx).Where("device_id = ?", deviceID).Order("received_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&traps).Error; err != nil {
		return nil, err
	}
	return traps, nil
}

func (r *trapLogRepository) GetAll(ctx context.Context, filter domain.TrapFilter) ([]domain.TrapLog, int64, error) {
	var traps []domain.TrapLog
	var total int64

	query := r.db.WithContext(ctx).Model(&domain.TrapLog{})

	if filter.DeviceID != "" {
		query = query.Where("device_id = ?", filter.DeviceID)
	}
	if filter.Severity != "" {
		query = query.Where("severity = ?", filter.Severity)
	}
	if filter.StartTime != nil {
		query = query.Where("received_at >= ?", filter.StartTime)
	}
	if filter.EndTime != nil {
		query = query.Where("received_at <= ?", filter.EndTime)
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination and get results
	query = query.Order("received_at DESC")
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	if err := query.Find(&traps).Error; err != nil {
		return nil, 0, err
	}

	return traps, total, nil
}

func (r *trapLogRepository) DeleteOlderThan(ctx context.Context, days int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -days)
	result := r.db.WithContext(ctx).Where("received_at < ?", cutoff).Delete(&domain.TrapLog{})
	return result.RowsAffected, result.Error
}
