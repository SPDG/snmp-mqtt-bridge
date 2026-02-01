package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// TrapSeverity represents the severity level of a trap
type TrapSeverity string

const (
	SeverityInfo     TrapSeverity = "info"
	SeverityWarning  TrapSeverity = "warning"
	SeverityError    TrapSeverity = "error"
	SeverityCritical TrapSeverity = "critical"
)

// TrapVariables stores SNMP trap variable bindings
type TrapVariables map[string]interface{}

func (v TrapVariables) Value() (driver.Value, error) {
	if v == nil {
		return "{}", nil
	}
	return json.Marshal(v)
}

func (v *TrapVariables) Scan(value interface{}) error {
	if value == nil {
		*v = make(TrapVariables)
		return nil
	}

	var data []byte
	switch val := value.(type) {
	case []byte:
		data = val
	case string:
		data = []byte(val)
	default:
		return errors.New("unsupported type for TrapVariables")
	}

	return json.Unmarshal(data, v)
}

// TrapLog represents a received SNMP trap
type TrapLog struct {
	ID         string        `json:"id" gorm:"primaryKey;type:text"`
	DeviceID   *string       `json:"device_id,omitempty" gorm:"type:text;index"`
	SourceIP   string        `json:"source_ip" gorm:"not null;type:text"`
	TrapOID    string        `json:"trap_oid" gorm:"not null;type:text"`
	Variables  TrapVariables `json:"variables" gorm:"type:text"`
	Severity   TrapSeverity  `json:"severity" gorm:"type:text"`
	Message    string        `json:"message" gorm:"type:text"`
	ReceivedAt time.Time     `json:"received_at" gorm:"index"`
}

// TrapDefinition defines how to interpret a specific trap OID
type TrapDefinition struct {
	OID         string       `json:"oid" yaml:"oid"`
	Name        string       `json:"name" yaml:"name"`
	Description string       `json:"description,omitempty" yaml:"description,omitempty"`
	Severity    TrapSeverity `json:"severity" yaml:"severity"`
	Message     string       `json:"message,omitempty" yaml:"message,omitempty"` // Go template
}

// TrapFilter represents filter options for querying trap logs
type TrapFilter struct {
	DeviceID  string
	Severity  TrapSeverity
	StartTime *time.Time
	EndTime   *time.Time
	Limit     int
	Offset    int
}
