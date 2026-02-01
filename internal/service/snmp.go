package service

import (
	"context"
	"fmt"

	"snmp-mqtt-bridge/internal/repository"

	"github.com/gosnmp/gosnmp"
)

// SNMPService handles SNMP operations
type SNMPService struct {
	deviceRepo  repository.DeviceRepository
	profileRepo repository.ProfileRepository
}

// NewSNMPService creates a new SNMP service
func NewSNMPService(deviceRepo repository.DeviceRepository, profileRepo repository.ProfileRepository) *SNMPService {
	return &SNMPService{
		deviceRepo:  deviceRepo,
		profileRepo: profileRepo,
	}
}

// SetValue sets an SNMP value on a device
func (s *SNMPService) SetValue(ctx context.Context, deviceID, oid string, value interface{}) error {
	device, err := s.deviceRepo.GetByID(ctx, deviceID)
	if err != nil {
		return fmt.Errorf("device not found: %w", err)
	}

	// Use write community if set, otherwise use read community
	community := device.Community
	if device.WriteCommunity != "" {
		community = device.WriteCommunity
	}

	client := createSNMPClient(device.IPAddress, device.Port, community, device.SNMPVersion)

	if err := client.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Conn.Close()

	// Determine PDU type based on value type
	var pdu gosnmp.SnmpPDU
	pdu.Name = oid

	switch v := value.(type) {
	case int:
		pdu.Type = gosnmp.Integer
		pdu.Value = v
	case int64:
		pdu.Type = gosnmp.Integer
		pdu.Value = int(v)
	case float64:
		pdu.Type = gosnmp.Integer
		pdu.Value = int(v)
	case string:
		pdu.Type = gosnmp.OctetString
		pdu.Value = v
	default:
		return fmt.Errorf("unsupported value type: %T", value)
	}

	_, err = client.Set([]gosnmp.SnmpPDU{pdu})
	if err != nil {
		return fmt.Errorf("SNMP SET failed: %w", err)
	}

	return nil
}

// GetValue gets a single SNMP value from a device
func (s *SNMPService) GetValue(ctx context.Context, deviceID, oid string) (interface{}, error) {
	device, err := s.deviceRepo.GetByID(ctx, deviceID)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}

	client := createSNMPClient(device.IPAddress, device.Port, device.Community, device.SNMPVersion)

	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Conn.Close()

	result, err := client.Get([]string{oid})
	if err != nil {
		return nil, fmt.Errorf("SNMP GET failed: %w", err)
	}

	if len(result.Variables) == 0 {
		return nil, fmt.Errorf("no value returned")
	}

	variable := result.Variables[0]
	switch variable.Type {
	case gosnmp.OctetString:
		return string(variable.Value.([]byte)), nil
	case gosnmp.Integer, gosnmp.Counter32, gosnmp.Counter64, gosnmp.Gauge32, gosnmp.TimeTicks, gosnmp.Uinteger32:
		return variable.Value, nil
	default:
		return variable.Value, nil
	}
}

// CommandRequest represents a command to execute on a device
type CommandRequest struct {
	DeviceID string      `json:"device_id" binding:"required"`
	OID      string      `json:"oid" binding:"required"`
	Value    interface{} `json:"value" binding:"required"`
}

// CommandResponse represents the result of a command
type CommandResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}
