package mqtt

import (
	"encoding/json"
	"strings"
	"testing"

	"snmp-mqtt-bridge/internal/domain"
)

func TestSanitizeEntityID(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{name: "Outlet 1 State", want: "outlet_1_state"},
		{name: "Active-Power", want: "active_power"},
		{name: "System.Uptime.Value", want: "system_uptime_value"},
		{name: "Special @#$ Characters", want: "special__characters"},
		{name: "UPPERCASE NAME", want: "uppercase_name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeEntityID(tt.name); got != tt.want {
				t.Errorf("sanitizeEntityID(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestDiscoveryConfig_MarshalJSON(t *testing.T) {
	config := &DiscoveryConfig{
		Name:     "Test Sensor",
		UniqueID: "test_sensor_id",
		Extra: map[string]interface{}{
			"device_class": "temperature",
			"custom_field": "custom_value",
		},
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	if m["name"] != "Test Sensor" {
		t.Errorf("Expected name to be %q, got %q", "Test Sensor", m["name"])
	}

	if m["unique_id"] != "test_sensor_id" {
		t.Errorf("Expected unique_id to be %q, got %q", "test_sensor_id", m["unique_id"])
	}

	if m["custom_field"] != "custom_value" {
		t.Errorf("Expected custom_field to be %q, got %q", "custom_value", m["custom_field"])
	}
}

func TestDiscovery_PublishBridgeDiscovery(t *testing.T) {
	mockMqtt := &mockMqttClient{connected: true}
	client := &Client{
		client:      mockMqtt,
		connected:   true,
		topicPrefix: "snmp-bridge",
	}

	discovery := NewDiscovery(client, "homeassistant", "snmp-bridge")

	err := discovery.PublishBridgeDiscovery()
	if err != nil {
		t.Fatalf("PublishBridgeDiscovery failed: %v", err)
	}

	if len(mockMqtt.published) != 1 {
		t.Fatalf("Expected 1 published message, got %d", len(mockMqtt.published))
	}

	pub := mockMqtt.published[0]
	expectedTopic := "homeassistant/sensor/snmp_bridge/status/config"
	if pub.topic != expectedTopic {
		t.Errorf("Expected topic %q, got %q", expectedTopic, pub.topic)
	}

	if !pub.retain {
		t.Error("Expected discovery message to be retained")
	}

	var payload map[string]interface{}
	err = json.Unmarshal(pub.payload.([]byte), &payload)
	if err != nil {
		t.Fatalf("Failed to parse payload JSON: %v", err)
	}

	if payload["name"] != "Bridge Status" {
		t.Errorf("Expected name 'Bridge Status', got %q", payload["name"])
	}
}

func TestDiscovery_PublishDevice(t *testing.T) {
	mockMqtt := &mockMqttClient{connected: true}
	client := &Client{
		client:      mockMqtt,
		connected:   true,
		topicPrefix: "snmp-bridge",
	}

	discovery := NewDiscovery(client, "homeassistant", "snmp-bridge")

	device := &domain.Device{
		ID:        "dev123",
		Name:      "Rack PDU",
		ProfileID: "aten-pdu-pe8108g",
	}

	profile := &domain.Profile{
		Manufacturer: "ATEN",
		Model:        "PE8108G",
		OIDMappings: []domain.OIDMapping{
			{
				OID:         ".1.3.6.1.4.1.21317.1.3.2.2.2.2.2.0",
				Name:        "Outlet 1 State",
				HAComponent: domain.HAComponentSwitch,
				Writable:    true,
			},
			{
				OID:         ".1.3.6.1.4.1.21317.1.3.2.2.2.2.10.1.2.1",
				Name:        "Outlet 1 Current",
				HAComponent: domain.HAComponentSensor,
				DeviceClass: "current",
				Unit:        "A",
			},
			{
				OID:         ".1.3.6.1.4.1.21317.1.3.2.1.0",
				Name:        "Temperature",
				HAComponent: domain.HAComponentSensor,
				DeviceClass: "temperature",
				Unit:        "C",
			},
		},
	}

	err := discovery.PublishDevice(device, profile)
	if err != nil {
		t.Fatalf("PublishDevice failed: %v", err)
	}

	// Should publish 3 entity configs
	if len(mockMqtt.published) != 3 {
		t.Fatalf("Expected 3 published messages, got %d", len(mockMqtt.published))
	}

	// Verify switch config
	var switchConfig map[string]interface{}
	var currentConfig map[string]interface{}
	var temperatureConfig map[string]interface{}

	for _, pub := range mockMqtt.published {
		if strings.Contains(pub.topic, "/switch/") {
			err = json.Unmarshal(pub.payload.([]byte), &switchConfig)
		} else if strings.Contains(pub.topic, "/sensor/") {
			var config map[string]interface{}
			err = json.Unmarshal(pub.payload.([]byte), &config)
			if err != nil {
				t.Fatalf("Failed to parse sensor discovery config: %v", err)
			}
			switch config["name"] {
			case "Outlet 1 Current":
				currentConfig = config
			case "Temperature":
				temperatureConfig = config
			}
		}
	}

	if switchConfig == nil {
		t.Fatal("Failed to find switch discovery config in published messages")
	}
	if currentConfig == nil {
		t.Fatal("Failed to find current discovery config in published messages")
	}
	if temperatureConfig == nil {
		t.Fatal("Failed to find temperature discovery config in published messages")
	}

	if switchConfig["name"] != "Outlet 1 State" {
		t.Errorf("Expected switch name 'Outlet 1 State', got %q", switchConfig["name"])
	}
	if switchConfig["command_topic"] != "snmp-bridge/dev123/outlet_1_state/set" {
		t.Errorf("Expected command topic for switch, got %q", switchConfig["command_topic"])
	}

	if currentConfig["device_class"] != "current" {
		t.Errorf("Expected current device class 'current', got %q", currentConfig["device_class"])
	}
	if currentConfig["unit_of_measurement"] != "A" {
		t.Errorf("Expected current unit 'A', got %q", currentConfig["unit_of_measurement"])
	}
	if temperatureConfig["device_class"] != "temperature" {
		t.Errorf("Expected temperature device class 'temperature', got %q", temperatureConfig["device_class"])
	}
	if temperatureConfig["unit_of_measurement"] != "°C" {
		t.Errorf("Expected temperature unit '°C', got %q", temperatureConfig["unit_of_measurement"])
	}
}
