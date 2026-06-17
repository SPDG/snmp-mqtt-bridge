package mqtt

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"snmp-mqtt-bridge/internal/config"
	"snmp-mqtt-bridge/internal/domain"
	"snmp-mqtt-bridge/internal/service"

	"github.com/gosnmp/gosnmp"
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

func TestPublisher_RefreshDiscoveryUsesCurrentMQTTConfig(t *testing.T) {
	mockMqtt := &mockMqttClient{connected: true}
	client := &Client{
		cfg: &config.MQTTConfig{
			TopicPrefix:     "new-prefix",
			DiscoveryPrefix: "new-homeassistant",
		},
		client:      mockMqtt,
		connected:   true,
		topicPrefix: "new-prefix",
		handlers:    make(map[string]CommandHandler),
	}
	discovery := NewDiscovery(client, "old-homeassistant", "old-prefix")

	profile := &domain.Profile{
		ID:           "profile1",
		Manufacturer: "APC",
		Model:        "ATS",
		OIDMappings: []domain.OIDMapping{
			{Name: "Preferred Source", OID: ".1.2.3.1", HAComponent: domain.HAComponentSelect, Writable: true},
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
		Name:      "Main ATS",
		ProfileID: "profile1",
		Enabled:   true,
	}

	if err := pub.RegisterDevice(device); err != nil {
		t.Fatalf("RegisterDevice failed: %v", err)
	}
	mockMqtt.published = nil

	if err := pub.RefreshDiscovery(); err != nil {
		t.Fatalf("RefreshDiscovery failed: %v", err)
	}

	if len(mockMqtt.published) != 2 {
		t.Fatalf("expected bridge and device discovery publishes, got %d", len(mockMqtt.published))
	}
	if mockMqtt.published[0].topic != "new-homeassistant/sensor/snmp_bridge/status/config" {
		t.Fatalf("expected bridge discovery on new prefix, got %q", mockMqtt.published[0].topic)
	}
	if mockMqtt.published[1].topic != "new-homeassistant/select/dev1/preferred_source/config" {
		t.Fatalf("expected device discovery on new prefix, got %q", mockMqtt.published[1].topic)
	}

	var payload DiscoveryConfig
	if err := json.Unmarshal(mockMqtt.published[1].payload.([]byte), &payload); err != nil {
		t.Fatalf("failed to unmarshal discovery payload: %v", err)
	}
	if payload.StateTopic != "new-prefix/dev1/preferred_source/state" {
		t.Fatalf("expected state topic to use new prefix, got %q", payload.StateTopic)
	}
	if payload.CommandTopic != "new-prefix/dev1/preferred_source/set" {
		t.Fatalf("expected command topic to use new prefix, got %q", payload.CommandTopic)
	}
	if payload.AvailabilityTopic != "new-prefix/bridge/status" {
		t.Fatalf("expected availability topic to use new prefix, got %q", payload.AvailabilityTopic)
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

type mockSnmpClient struct {
	connectErr error
	getPacket  *gosnmp.SnmpPacket
	getErr     error
	setPacket  *gosnmp.SnmpPacket
	setErr     error
	calledSet  []gosnmp.SnmpPDU
	calledGet  []string
	closed     bool
}

func (m *mockSnmpClient) Connect() error {
	return m.connectErr
}

func (m *mockSnmpClient) Get(oids []string) (*gosnmp.SnmpPacket, error) {
	m.calledGet = oids
	return m.getPacket, m.getErr
}

func (m *mockSnmpClient) Set(pdus []gosnmp.SnmpPDU) (*gosnmp.SnmpPacket, error) {
	m.calledSet = pdus
	return m.setPacket, m.setErr
}

func (m *mockSnmpClient) Close() error {
	m.closed = true
	return nil
}

func TestPublisher_HandleCommand_Success(t *testing.T) {
	mockMqtt := &mockMqttClient{connected: true}
	client := &Client{
		client:      mockMqtt,
		connected:   true,
		topicPrefix: "snmp-bridge",
		handlers:    make(map[string]CommandHandler),
	}
	discovery := NewDiscovery(client, "homeassistant", "snmp-bridge")

	profile := &domain.Profile{
		ID: "profile1",
		OIDMappings: []domain.OIDMapping{
			{Name: "Outlet 1 State", OID: ".1.2.3.3", HAComponent: domain.HAComponentSwitch, Writable: true},
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

	// Inject mock SNMP client
	mockSnmp := &mockSnmpClient{
		setPacket: &gosnmp.SnmpPacket{
			Error: gosnmp.NoError,
		},
	}
	pub.createSNMPClientFunc = func(d *domain.Device, community string) snmpClient {
		return mockSnmp
	}

	// Call handleCommand (payload "ON" -> should send SNMP integer 1)
	pub.handleCommand("dev1", "outlet_1_state", []byte("ON"))

	if len(mockSnmp.calledSet) != 1 {
		t.Fatalf("Expected 1 SNMP SET call, got %d", len(mockSnmp.calledSet))
	}
	pdu := mockSnmp.calledSet[0]
	if pdu.Name != ".1.2.3.3" {
		t.Errorf("Expected OID .1.2.3.3, got %s", pdu.Name)
	}
	if pdu.Type != gosnmp.Integer || pdu.Value.(int) != 1 {
		t.Errorf("Expected type Integer and value 1, got type %v and value %v", pdu.Type, pdu.Value)
	}
	if !mockSnmp.closed {
		t.Error("Expected SNMP client to be closed")
	}
}

func TestPublisher_HandleCommand_SNMPError(t *testing.T) {
	mockMqtt := &mockMqttClient{connected: true}
	client := &Client{
		client:      mockMqtt,
		connected:   true,
		topicPrefix: "snmp-bridge",
		handlers:    make(map[string]CommandHandler),
	}
	discovery := NewDiscovery(client, "homeassistant", "snmp-bridge")

	profile := &domain.Profile{
		ID: "profile1",
		OIDMappings: []domain.OIDMapping{
			{Name: "Outlet 1 State", OID: ".1.2.3.3", HAComponent: domain.HAComponentSwitch, Writable: true},
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

	// 1. Connection error
	mockSnmp := &mockSnmpClient{
		connectErr: fmt.Errorf("connection timeout"),
	}
	pub.createSNMPClientFunc = func(d *domain.Device, community string) snmpClient {
		return mockSnmp
	}
	pub.handleCommand("dev1", "outlet_1_state", []byte("ON")) // Should log and return gracefully
	if len(mockSnmp.calledSet) != 0 {
		t.Error("Expected no SNMP SET call on connection failure")
	}

	// 2. SNMP Packet Error (e.g., NoSuchName)
	mockSnmpPacketErr := &mockSnmpClient{
		setPacket: &gosnmp.SnmpPacket{
			Error:      gosnmp.NoSuchName,
			ErrorIndex: 1,
		},
	}
	pub.createSNMPClientFunc = func(d *domain.Device, community string) snmpClient {
		return mockSnmpPacketErr
	}
	pub.handleCommand("dev1", "outlet_1_state", []byte("ON")) // Should log and return gracefully
	if len(mockSnmpPacketErr.calledSet) != 1 {
		t.Fatalf("Expected 1 SNMP SET call, got %d", len(mockSnmpPacketErr.calledSet))
	}
}
