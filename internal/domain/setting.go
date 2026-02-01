package domain

// Setting represents an application setting stored in the database
type Setting struct {
	Key   string `json:"key" gorm:"primaryKey;type:text"`
	Value string `json:"value" gorm:"not null;type:text"`
}

// Common setting keys
const (
	SettingMQTTBroker          = "mqtt.broker"
	SettingMQTTPort            = "mqtt.port"
	SettingMQTTUsername        = "mqtt.username"
	SettingMQTTPassword        = "mqtt.password"
	SettingMQTTTopicPrefix     = "mqtt.topic_prefix"
	SettingMQTTDiscovery       = "mqtt.discovery"
	SettingMQTTDiscoveryPrefix = "mqtt.discovery_prefix"
	SettingSNMPPollInterval    = "snmp.poll_interval"
	SettingSNMPTrapPort        = "snmp.trap_port"
	SettingUITheme             = "ui.theme"
)
