package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// DeviceCategory represents the type of device
type DeviceCategory string

const (
	CategoryUPS DeviceCategory = "ups"
	CategoryATS DeviceCategory = "ats"
	CategoryPDU DeviceCategory = "pdu"
)

// OIDType represents the data type of an OID value
type OIDType string

const (
	OIDTypeString          OIDType = "string"
	OIDTypeInteger         OIDType = "integer"
	OIDTypeGauge           OIDType = "gauge"
	OIDTypeCounter         OIDType = "counter"
	OIDTypeBool            OIDType = "bool"
	OIDTypeEnum            OIDType = "enum"
	OIDTypeCompositeSwitch OIDType = "composite_switch" // For Energenie-style comma-separated outlet status
)

// HAComponent represents Home Assistant component type
type HAComponent string

const (
	HAComponentSensor       HAComponent = "sensor"
	HAComponentBinarySensor HAComponent = "binary_sensor"
	HAComponentSwitch       HAComponent = "switch"
	HAComponentButton       HAComponent = "button"
	HAComponentNumber       HAComponent = "number"
	HAComponentSelect       HAComponent = "select"
)

// OIDMapping defines how to map an SNMP OID to Home Assistant
type OIDMapping struct {
	OID          string                 `json:"oid" yaml:"oid"`
	Name         string                 `json:"name" yaml:"name"`
	Description  string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Type         OIDType                `json:"type" yaml:"type"`
	Unit         string                 `json:"unit,omitempty" yaml:"unit,omitempty"`
	Scale        float64                `json:"scale,omitempty" yaml:"scale,omitempty"`
	HAComponent  HAComponent            `json:"ha_component" yaml:"ha_component"`
	DeviceClass  string                 `json:"device_class,omitempty" yaml:"device_class,omitempty"`
	StateClass   string                 `json:"state_class,omitempty" yaml:"state_class,omitempty"`
	Icon         string                 `json:"icon,omitempty" yaml:"icon,omitempty"`
	EnumValues   map[int]string         `json:"enum_values,omitempty" yaml:"enum_values,omitempty"`
	Writable     bool                   `json:"writable,omitempty" yaml:"writable,omitempty"`
	WriteOID     string                 `json:"write_oid,omitempty" yaml:"write_oid,omitempty"`
	PollGroup    string                 `json:"poll_group,omitempty" yaml:"poll_group,omitempty"` // "frequent" or "static"
	Category     string                 `json:"category,omitempty" yaml:"category,omitempty"`     // HA entity category: config, diagnostic
	Extra        map[string]interface{} `json:"extra,omitempty" yaml:"extra,omitempty"`

	// Composite value handling (for Energenie-style comma-separated outlet status)
	CompositeIndex     int    `json:"composite_index,omitempty" yaml:"composite_index,omitempty"`         // Index in comma-separated string (0-based)
	CompositeSeparator string `json:"composite_separator,omitempty" yaml:"composite_separator,omitempty"` // Separator (default: ",")
}

// IndexedOIDMapping represents an OID mapping that should be polled with an index (e.g., outlets)
type IndexedOIDMapping struct {
	OIDMapping `yaml:",inline"`
	BaseOID    string `json:"base_oid" yaml:"base_oid"`
	IndexStart int    `json:"index_start" yaml:"index_start"`
	IndexEnd   int    `json:"index_end" yaml:"index_end"`
	NameFormat string `json:"name_format" yaml:"name_format"` // e.g., "Outlet %d"
}

// OIDMappings is a slice of OID mappings that can be stored in the database
type OIDMappings []OIDMapping

func (m OIDMappings) Value() (driver.Value, error) {
	return json.Marshal(m)
}

func (m *OIDMappings) Scan(value interface{}) error {
	if value == nil {
		*m = make(OIDMappings, 0)
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return errors.New("unsupported type for OIDMappings")
	}

	return json.Unmarshal(data, m)
}

// Profile represents a device profile with OID mappings
type Profile struct {
	ID           string         `json:"id" gorm:"primaryKey;type:text"`
	Name         string         `json:"name" gorm:"not null;type:text"`
	Manufacturer string         `json:"manufacturer" gorm:"type:text"`
	Model        string         `json:"model,omitempty" gorm:"type:text"`
	Category     DeviceCategory `json:"category" gorm:"type:text"`
	SysObjectID  string         `json:"sys_object_id,omitempty" gorm:"type:text"` // For auto-detection
	OIDMappings  OIDMappings    `json:"oid_mappings" gorm:"type:text"`
	IsBuiltin    bool           `json:"is_builtin" gorm:"default:false"`
}

// ProfileYAML represents the YAML structure for profile files
type ProfileYAML struct {
	ID             string              `yaml:"id"`
	Name           string              `yaml:"name"`
	Manufacturer   string              `yaml:"manufacturer"`
	Model          string              `yaml:"model,omitempty"`
	Category       DeviceCategory      `yaml:"category"`
	SysObjectID    string              `yaml:"sys_object_id,omitempty"`
	OIDMappings    []OIDMapping        `yaml:"oid_mappings"`
	IndexedOIDs    []IndexedOIDMapping `yaml:"indexed_oids,omitempty"`
	PollGroups     map[string]int      `yaml:"poll_groups,omitempty"` // group name -> interval multiplier
}
