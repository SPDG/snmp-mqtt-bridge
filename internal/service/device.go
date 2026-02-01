package service

import (
	"context"
	"fmt"
	"time"

	"snmp-mqtt-bridge/internal/domain"
	"snmp-mqtt-bridge/internal/repository"

	"github.com/google/uuid"
	"github.com/gosnmp/gosnmp"
)

// DeviceService handles device business logic
type DeviceService struct {
	repo repository.DeviceRepository
}

// NewDeviceService creates a new device service
func NewDeviceService(repo repository.DeviceRepository) *DeviceService {
	return &DeviceService{repo: repo}
}

// Create creates a new device
func (s *DeviceService) Create(ctx context.Context, req *domain.DeviceCreateRequest) (*domain.Device, error) {
	device := &domain.Device{
		ID:           uuid.New().String(),
		Name:         req.Name,
		IPAddress:    req.IPAddress,
		Port:         req.Port,
		Community:    req.Community,
		SNMPVersion:  req.SNMPVersion,
		ProfileID:    req.ProfileID,
		PollInterval: req.PollInterval,
		Enabled:      req.Enabled,
		Labels:       req.Labels,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if device.Port == 0 {
		device.Port = 161
	}

	if err := s.repo.Create(ctx, device); err != nil {
		return nil, err
	}

	return device, nil
}

// GetByID retrieves a device by ID
func (s *DeviceService) GetByID(ctx context.Context, id string) (*domain.Device, error) {
	return s.repo.GetByID(ctx, id)
}

// GetAll retrieves all devices
func (s *DeviceService) GetAll(ctx context.Context) ([]domain.Device, error) {
	return s.repo.GetAll(ctx)
}

// GetEnabled retrieves all enabled devices
func (s *DeviceService) GetEnabled(ctx context.Context) ([]domain.Device, error) {
	return s.repo.GetEnabled(ctx)
}

// Update updates an existing device
func (s *DeviceService) Update(ctx context.Context, id string, req *domain.DeviceUpdateRequest) (*domain.Device, error) {
	device, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		device.Name = *req.Name
	}
	if req.IPAddress != nil {
		device.IPAddress = *req.IPAddress
	}
	if req.Port != nil {
		device.Port = *req.Port
	}
	if req.Community != nil {
		device.Community = *req.Community
	}
	if req.SNMPVersion != nil {
		device.SNMPVersion = *req.SNMPVersion
	}
	if req.ProfileID != nil {
		device.ProfileID = *req.ProfileID
	}
	if req.PollInterval != nil {
		device.PollInterval = *req.PollInterval
	}
	if req.Enabled != nil {
		device.Enabled = *req.Enabled
	}
	if req.Labels != nil {
		device.Labels = req.Labels
	}

	device.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, device); err != nil {
		return nil, err
	}

	return device, nil
}

// Delete deletes a device
func (s *DeviceService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// UpdateLastSeen updates the device's last seen timestamp
func (s *DeviceService) UpdateLastSeen(ctx context.Context, id string) error {
	return s.repo.UpdateLastSeen(ctx, id)
}

// TestConnection tests SNMP connection to a device
func (s *DeviceService) TestConnection(ctx context.Context, req *domain.TestConnectionRequest) (*domain.TestConnectionResponse, error) {
	port := req.Port
	if port == 0 {
		port = 161
	}

	snmpClient := createSNMPClient(req.IPAddress, port, req.Community, req.SNMPVersion)

	start := time.Now()

	if err := snmpClient.Connect(); err != nil {
		return &domain.TestConnectionResponse{
			Success: false,
			Message: fmt.Sprintf("Connection failed: %v", err),
		}, nil
	}
	defer snmpClient.Conn.Close()

	// Query system OIDs
	oids := []string{
		".1.3.6.1.2.1.1.1.0", // sysDescr
		".1.3.6.1.2.1.1.2.0", // sysObjectID
		".1.3.6.1.2.1.1.5.0", // sysName
	}

	result, err := snmpClient.Get(oids)
	if err != nil {
		return &domain.TestConnectionResponse{
			Success: false,
			Message: fmt.Sprintf("SNMP query failed: %v", err),
		}, nil
	}

	responseTime := time.Since(start).Milliseconds()

	response := &domain.TestConnectionResponse{
		Success:      true,
		Message:      "Connection successful",
		ResponseTime: responseTime,
	}

	for _, variable := range result.Variables {
		switch variable.Name {
		case ".1.3.6.1.2.1.1.1.0":
			response.SysDescr = fmt.Sprintf("%v", variable.Value)
		case ".1.3.6.1.2.1.1.2.0":
			response.SysObjectID = fmt.Sprintf("%v", variable.Value)
		case ".1.3.6.1.2.1.1.5.0":
			response.SysName = fmt.Sprintf("%v", variable.Value)
		}
	}

	return response, nil
}

func snmpVersionToGoSNMP(v domain.SNMPVersion) gosnmp.SnmpVersion {
	switch v {
	case domain.SNMPv1:
		return gosnmp.Version1
	case domain.SNMPv3:
		return gosnmp.Version3
	default:
		return gosnmp.Version2c
	}
}

// createSNMPClient creates a properly configured SNMP client based on device settings
func createSNMPClient(target string, port int, community string, version domain.SNMPVersion) *gosnmp.GoSNMP {
	client := &gosnmp.GoSNMP{
		Target:  target,
		Port:    uint16(port),
		Version: snmpVersionToGoSNMP(version),
		Timeout: time.Second * 5,
		Retries: 2,
	}

	if version == domain.SNMPv3 {
		// For SNMPv3, use community field as username (noAuthNoPriv mode)
		client.SecurityModel = gosnmp.UserSecurityModel
		client.MsgFlags = gosnmp.NoAuthNoPriv
		client.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName: community,
		}
	} else {
		// For v1/v2c, use community string
		client.Community = community
	}

	return client
}
