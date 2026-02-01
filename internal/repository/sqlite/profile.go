package sqlite

import (
	"context"

	"snmp-mqtt-bridge/internal/domain"
	"snmp-mqtt-bridge/internal/repository"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type profileRepository struct {
	db *gorm.DB
}

// NewProfileRepository creates a new profile repository
func NewProfileRepository(db *gorm.DB) repository.ProfileRepository {
	return &profileRepository{db: db}
}

func (r *profileRepository) Create(ctx context.Context, profile *domain.Profile) error {
	return r.db.WithContext(ctx).Create(profile).Error
}

func (r *profileRepository) GetByID(ctx context.Context, id string) (*domain.Profile, error) {
	var profile domain.Profile
	if err := r.db.WithContext(ctx).First(&profile, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &profile, nil
}

func (r *profileRepository) GetAll(ctx context.Context) ([]domain.Profile, error) {
	var profiles []domain.Profile
	if err := r.db.WithContext(ctx).Find(&profiles).Error; err != nil {
		return nil, err
	}
	return profiles, nil
}

func (r *profileRepository) GetBuiltin(ctx context.Context) ([]domain.Profile, error) {
	var profiles []domain.Profile
	if err := r.db.WithContext(ctx).Where("is_builtin = ?", true).Find(&profiles).Error; err != nil {
		return nil, err
	}
	return profiles, nil
}

func (r *profileRepository) GetBySysObjectID(ctx context.Context, sysOID string) (*domain.Profile, error) {
	var profile domain.Profile
	if err := r.db.WithContext(ctx).Where("sys_object_id = ?", sysOID).First(&profile).Error; err != nil {
		return nil, err
	}
	return &profile, nil
}

func (r *profileRepository) Update(ctx context.Context, profile *domain.Profile) error {
	return r.db.WithContext(ctx).Save(profile).Error
}

func (r *profileRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&domain.Profile{}, "id = ?", id).Error
}

func (r *profileRepository) Upsert(ctx context.Context, profile *domain.Profile) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).Create(profile).Error
}
