package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"snmp-mqtt-bridge/internal/domain"
	"snmp-mqtt-bridge/internal/repository"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// ProfileService handles profile business logic
type ProfileService struct {
	repo repository.ProfileRepository
}

// NewProfileService creates a new profile service
func NewProfileService(repo repository.ProfileRepository) *ProfileService {
	return &ProfileService{repo: repo}
}

// Create creates a new profile
func (s *ProfileService) Create(ctx context.Context, profile *domain.Profile) error {
	if profile.ID == "" {
		profile.ID = uuid.New().String()
	}
	return s.repo.Create(ctx, profile)
}

// GetByID retrieves a profile by ID
func (s *ProfileService) GetByID(ctx context.Context, id string) (*domain.Profile, error) {
	return s.repo.GetByID(ctx, id)
}

// GetAll retrieves all profiles
func (s *ProfileService) GetAll(ctx context.Context) ([]domain.Profile, error) {
	return s.repo.GetAll(ctx)
}

// GetBySysObjectID finds a profile by SNMP sysObjectID
func (s *ProfileService) GetBySysObjectID(ctx context.Context, sysOID string) (*domain.Profile, error) {
	return s.repo.GetBySysObjectID(ctx, sysOID)
}

// Update updates an existing profile
func (s *ProfileService) Update(ctx context.Context, profile *domain.Profile) error {
	return s.repo.Update(ctx, profile)
}

// Delete deletes a profile
func (s *ProfileService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// LoadBuiltinProfiles loads profiles from YAML files in the profiles directory
func (s *ProfileService) LoadBuiltinProfiles(ctx context.Context, profilesDir string) error {
	files, err := filepath.Glob(filepath.Join(profilesDir, "*.yaml"))
	if err != nil {
		return fmt.Errorf("failed to list profile files: %w", err)
	}

	for _, file := range files {
		if err := s.loadProfileFile(ctx, file); err != nil {
			return fmt.Errorf("failed to load profile %s: %w", file, err)
		}
	}

	return nil
}

func (s *ProfileService) loadProfileFile(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var profileYAML domain.ProfileYAML
	if err := yaml.Unmarshal(data, &profileYAML); err != nil {
		return err
	}

	// Expand indexed OIDs
	oidMappings := make([]domain.OIDMapping, 0, len(profileYAML.OIDMappings))
	oidMappings = append(oidMappings, profileYAML.OIDMappings...)

	for _, indexed := range profileYAML.IndexedOIDs {
		for i := indexed.IndexStart; i <= indexed.IndexEnd; i++ {
			mapping := indexed.OIDMapping
			mapping.OID = fmt.Sprintf("%s.%d", indexed.BaseOID, i)
			mapping.Name = fmt.Sprintf(indexed.NameFormat, i)
			oidMappings = append(oidMappings, mapping)
		}
	}

	profile := &domain.Profile{
		ID:           profileYAML.ID,
		Name:         profileYAML.Name,
		Manufacturer: profileYAML.Manufacturer,
		Model:        profileYAML.Model,
		Category:     profileYAML.Category,
		SysObjectID:  profileYAML.SysObjectID,
		SNMPVersions: profileYAML.SNMPVersions,
		OIDMappings:  oidMappings,
		IsBuiltin:    true,
	}

	return s.repo.Upsert(ctx, profile)
}
