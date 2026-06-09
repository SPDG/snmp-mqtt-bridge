package mqtt

import (
	"testing"
	"time"

	"snmp-mqtt-bridge/internal/domain"
	"snmp-mqtt-bridge/internal/service"
)

func TestPublisher_RegisterUnregisterDevice(t *testing.T) {
	mockMqtt := &mockMqttClient{connected: true}
	client := &Client{
		client:      mockMqtt,
		connected:   true,
		topicPrefix: "snmp-bridge",
		handlers:    make(map[string]CommandHandler),
	}
	discovery := NewDiscovery(client, "homeassistant", "snmp-bridge")

	profile := &domain.Profile{
		ID:           "profile1",
		Manufacturer: "APC",
		Model:        "UPS",
		OIDMappings: []domain.OIDMapping{
			{Name: "Voltage", OID: ".1.2.3.1"},
		},
	}

	profileRepo := &mockProfileRepo{
		profiles: map[string]*domain.Profile{
			"profile1": profile,
		},
	}

	pub := NewPublisher(client, discovery, nil, profileRepo)

	device := &domain.Device{
		ID:        "dev1",
		Name:      "Main UPS",
		ProfileID: "profile1",
	}

	// Test Register
	err := pub.RegisterDevice(device)
	if err != nil {
		t.Fatalf("RegisterDevice failed: %v", err)
	}

	pub.devicesMu.RLock()
	info, exists := pub.devices["dev1"]
	pub.devicesMu.RUnlock()

	if !exists {
		t.Error("Expected device to be registered")
	}
	if info.profile.ID != "profile1" {
		t.Errorf("Expected profile ID 'profile1', got %q", info.profile.ID)
	}

	// Should publish discovery and subscribe to commands
	if len(mockMqtt.published) != 1 {
		t.Errorf("Expected 1 published discovery message, got %d", len(mockMqtt.published))
	}
	if len(mockMqtt.subscribed) != 1 {
		t.Errorf("Expected 1 subscribed command topic, got %d", len(mockMqtt.subscribed))
	}

	// Test Unregister
	err = pub.UnregisterDevice("dev1")
	if err != nil {
		t.Fatalf("UnregisterDevice failed: %v", err)
	}

	pub.devicesMu.RLock()
	_, exists = pub.devices["dev1"]
	pub.devicesMu.RUnlock()

	if exists {
		t.Error("Expected device to be unregistered")
	}
}

func TestPublisher_PublishState(t *testing.T) {
	mockMqtt := &mockMqttClient{connected: true}
	client := &Client{
		client:      mockMqtt,
		connected:   true,
		topicPrefix: "snmp-bridge",
		handlers:    make(map[string]CommandHandler),
	}
	discovery := NewDiscovery(client, "homeassistant", "snmp-bridge")

	profile := &domain.Profile{
		ID:           "profile1",
		Manufacturer: "APC",
		Model:        "UPS",
		OIDMappings: []domain.OIDMapping{
			{Name: "Voltage", OID: ".1.2.3.1"},
			{Name: "Battery Status", OID: ".1.2.3.2", HAComponent: domain.HAComponentBinarySensor, DeviceClass: "battery"},
			{Name: "Outlet 1 State", OID: ".1.2.3.3", HAComponent: domain.HAComponentSwitch},
		},
	}

	profileRepo := &mockProfileRepo{
		profiles: map[string]*domain.Profile{
			"profile1": profile,
		},
	}

	pub := NewPublisher(client, discovery, nil, profileRepo)

	device := &domain.Device{
		ID:        "dev1",
		Name:      "Main UPS",
		ProfileID: "profile1",
	}

	_ = pub.RegisterDevice(device)
	// Clear registration mock publishes
	mockMqtt.published = nil

	// Call publishState
	event := service.StateUpdateEvent{
		DeviceID:  "dev1",
		Timestamp: time.Now(),
		Online:    true,
		Values: map[string]interface{}{
			"Voltage":        230.1,
			"Battery Status": 1, // ON
			"Outlet 1 State": 1, // ON (APC switch)
		},
	}

	pub.publishState(event)

	// Should publish:
	// - 3 entity states (voltage, battery_status, outlet_1_state)
	// - 1 full state
	if len(mockMqtt.published) != 4 {
		t.Fatalf("Expected 4 published messages, got %d", len(mockMqtt.published))
	}

	var hasVoltage, hasBattery, hasOutlet, hasFullState bool
	for _, pub := range mockMqtt.published {
		if pub.topic == "snmp-bridge/dev1/voltage/state" {
			hasVoltage = true
			if string(pub.payload.([]byte)) != "230.1" {
				t.Errorf("Expected voltage payload 230.1, got %q", pub.payload)
			}
		} else if pub.topic == "snmp-bridge/dev1/battery_status/state" {
			hasBattery = true
			if string(pub.payload.([]byte)) != "ON" {
				t.Errorf("Expected battery status payload ON, got %q", pub.payload)
			}
		} else if pub.topic == "snmp-bridge/dev1/outlet_1_state/state" {
			hasOutlet = true
			if string(pub.payload.([]byte)) != "ON" {
				t.Errorf("Expected outlet 1 state ON, got %q", pub.payload)
			}
		} else if pub.topic == "snmp-bridge/dev1/state" {
			hasFullState = true
		}
	}

	if !hasVoltage {
		t.Error("Missing voltage publish")
	}
	if !hasBattery {
		t.Error("Missing battery status publish")
	}
	if !hasOutlet {
		t.Error("Missing outlet 1 state publish")
	}
	if !hasFullState {
		t.Error("Missing full state publish")
	}
}

func TestConvertToSwitchValue(t *testing.T) {
	tests := []struct {
		val  interface{}
		want string
	}{
		{val: "On", want: "ON"},
		{val: "off", want: "OFF"},
		{val: 1, want: "ON"},
		{val: 2, want: "OFF"},
		{val: "1", want: "ON"},
		{val: "2", want: "OFF"},
		{val: true, want: "ON"},
		{val: false, want: "OFF"},
	}

	for _, tt := range tests {
		got := convertToSwitchValue(tt.val)
		if got != tt.want {
			t.Errorf("convertToSwitchValue(%v) = %q, want %q", tt.val, got, tt.want)
		}
	}
}

func TestConvertToBinarySensorValue(t *testing.T) {
	tests := []struct {
		val         interface{}
		deviceClass string
		want        string
	}{
		{val: "ok", deviceClass: "problem", want: "OFF"},
		{val: "error", deviceClass: "problem", want: "ON"},
		{val: "normal", deviceClass: "safety", want: "OFF"},
		{val: "warning", deviceClass: "safety", want: "ON"},
		{val: "ok", deviceClass: "power", want: "ON"},
		{val: "offline", deviceClass: "power", want: "OFF"},
		{val: "normal", deviceClass: "power", want: "ON"},
		{val: "online", deviceClass: "", want: "OFF"},
	}

	for _, tt := range tests {
		got := convertToBinarySensorValue(tt.val, tt.deviceClass)
		if got != tt.want {
			t.Errorf("convertToBinarySensorValue(%v, %q) = %q, want %q", tt.val, tt.deviceClass, got, tt.want)
		}
	}
}
