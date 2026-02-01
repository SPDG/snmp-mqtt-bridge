package sqlite

import (
	"context"
	"errors"

	"snmp-mqtt-bridge/internal/domain"
	"snmp-mqtt-bridge/internal/repository"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type settingRepository struct {
	db *gorm.DB
}

// NewSettingRepository creates a new setting repository
func NewSettingRepository(db *gorm.DB) repository.SettingRepository {
	return &settingRepository{db: db}
}

func (r *settingRepository) Get(ctx context.Context, key string) (string, error) {
	var setting domain.Setting
	if err := r.db.WithContext(ctx).First(&setting, "key = ?", key).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return setting.Value, nil
}

func (r *settingRepository) Set(ctx context.Context, key, value string) error {
	setting := domain.Setting{Key: key, Value: value}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&setting).Error
}

func (r *settingRepository) GetAll(ctx context.Context) ([]domain.Setting, error) {
	var settings []domain.Setting
	if err := r.db.WithContext(ctx).Find(&settings).Error; err != nil {
		return nil, err
	}
	return settings, nil
}

func (r *settingRepository) Delete(ctx context.Context, key string) error {
	return r.db.WithContext(ctx).Delete(&domain.Setting{}, "key = ?", key).Error
}
