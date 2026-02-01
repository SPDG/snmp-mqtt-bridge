package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// SNMPVersion represents SNMP protocol version
type SNMPVersion string

const (
	SNMPv1  SNMPVersion = "v1"
	SNMPv2c SNMPVersion = "v2c"
	SNMPv3  SNMPVersion = "v3"
)

// Labels is a map for custom outlet/port labels
type Labels map[string]string

func (l Labels) Value() (driver.Value, error) {
	if l == nil {
		return "{}", nil
	}
	return json.Marshal(l)
}

func (l *Labels) Scan(value interface{}) error {
	if value == nil {
		*l = make(Labels)
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return errors.New("unsupported type for Labels")
	}

	return json.Unmarshal(data, l)
}

// Device represents an SNMP device
type Device struct {
	ID           string      `json:"id" gorm:"primaryKey;type:text"`
	Name         string      `json:"name" gorm:"not null;type:text"`
	IPAddress    string      `json:"ip_address" gorm:"not null;type:text"`
	Port         int         `json:"port" gorm:"default:161"`
	Community    string      `json:"community" gorm:"not null;type:text"`
	SNMPVersion  SNMPVersion `json:"snmp_version" gorm:"not null;type:text"`
	ProfileID    string      `json:"profile_id" gorm:"type:text"`
	PollInterval int         `json:"poll_interval" gorm:"type:integer"` // seconds, 0 = use default
	Enabled      bool        `json:"enabled" gorm:"default:true"`
	Labels       Labels      `json:"labels" gorm:"type:text"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
	LastSeen     *time.Time  `json:"last_seen,omitempty"`
}

// DeviceCreateRequest is used for creating a new device
type DeviceCreateRequest struct {
	Name         string            `json:"name" binding:"required"`
	IPAddress    string            `json:"ip_address" binding:"required,ip"`
	Port         int               `json:"port"`
	Community    string            `json:"community" binding:"required"`
	SNMPVersion  SNMPVersion       `json:"snmp_version" binding:"required,oneof=v1 v2c v3"`
	ProfileID    string            `json:"profile_id"`
	PollInterval int               `json:"poll_interval"`
	Enabled      bool              `json:"enabled"`
	Labels       map[string]string `json:"labels"`
}

// DeviceUpdateRequest is used for updating an existing device
type DeviceUpdateRequest struct {
	Name         *string           `json:"name,omitempty"`
	IPAddress    *string           `json:"ip_address,omitempty" binding:"omitempty,ip"`
	Port         *int              `json:"port,omitempty"`
	Community    *string           `json:"community,omitempty"`
	SNMPVersion  *SNMPVersion      `json:"snmp_version,omitempty" binding:"omitempty,oneof=v1 v2c v3"`
	ProfileID    *string           `json:"profile_id,omitempty"`
	PollInterval *int              `json:"poll_interval,omitempty"`
	Enabled      *bool             `json:"enabled,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
}

// DeviceState represents the current state of a device
type DeviceState struct {
	DeviceID   string                 `json:"device_id"`
	Online     bool                   `json:"online"`
	LastPoll   time.Time              `json:"last_poll"`
	Values     map[string]interface{} `json:"values"`
	Errors     []string               `json:"errors,omitempty"`
}

// TestConnectionRequest is used for testing SNMP connection
type TestConnectionRequest struct {
	IPAddress   string      `json:"ip_address" binding:"required,ip"`
	Port        int         `json:"port"`
	Community   string      `json:"community" binding:"required"`
	SNMPVersion SNMPVersion `json:"snmp_version" binding:"required,oneof=v1 v2c v3"`
}

// TestConnectionResponse contains the result of a connection test
type TestConnectionResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	SysDescr     string `json:"sys_descr,omitempty"`
	SysName      string `json:"sys_name,omitempty"`
	SysObjectID  string `json:"sys_object_id,omitempty"`
	ResponseTime int64  `json:"response_time_ms,omitempty"`
}
