package mqtt

import (
	"encoding/json"
	"testing"
	"time"

	"snmp-mqtt-bridge/internal/domain"
)

func TestClient_Publish(t *testing.T) {
	mockMqtt := &mockMqttClient{connected: true}
	c := &Client{
		client:      mockMqtt,
		connected:   true,
		topicPrefix: "snmp-bridge",
	}

	err := c.Publish("test/topic", "hello", true)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	if len(mockMqtt.published) != 1 {
		t.Fatalf("Expected 1 published message, got %d", len(mockMqtt.published))
	}

	pub := mockMqtt.published[0]
	if pub.topic != "test/topic" {
		t.Errorf("Expected topic 'test/topic', got %q", pub.topic)
	}
	if string(pub.payload.([]byte)) != "hello" {
		t.Errorf("Expected payload 'hello', got %q", pub.payload)
	}
	if !pub.retain {
		t.Error("Expected retain to be true")
	}
}

func TestClient_PublishState(t *testing.T) {
	mockMqtt := &mockMqttClient{connected: true}
	c := &Client{
		client:      mockMqtt,
		connected:   true,
		topicPrefix: "snmp-bridge",
	}

	state := &domain.DeviceState{
		DeviceID: "dev1",
		Online:   true,
		LastPoll: time.Now(),
		Values: map[string]interface{}{
			"Voltage": 230.5,
		},
	}

	err := c.PublishState("dev1", state)
	if err != nil {
		t.Fatalf("PublishState failed: %v", err)
	}

	if len(mockMqtt.published) != 1 {
		t.Fatalf("Expected 1 published message, got %d", len(mockMqtt.published))
	}

	pub := mockMqtt.published[0]
	expectedTopic := "snmp-bridge/dev1/state"
	if pub.topic != expectedTopic {
		t.Errorf("Expected topic %q, got %q", expectedTopic, pub.topic)
	}

	var payload map[string]interface{}
	err = json.Unmarshal(pub.payload.([]byte), &payload)
	if err != nil {
		t.Fatalf("Failed to parse payload: %v", err)
	}

	if payload["device_id"] != "dev1" {
		t.Errorf("Expected device_id 'dev1', got %q", payload["device_id"])
	}
}

func TestClient_PublishEntityState(t *testing.T) {
	mockMqtt := &mockMqttClient{connected: true}
	c := &Client{
		client:      mockMqtt,
		connected:   true,
		topicPrefix: "snmp-bridge",
	}

	// Test bool true -> "ON"
	err := c.PublishEntityState("dev1", "status", true)
	if err != nil {
		t.Fatalf("PublishEntityState failed: %v", err)
	}

	// Test bool false -> "OFF"
	err = c.PublishEntityState("dev1", "status", false)
	if err != nil {
		t.Fatalf("PublishEntityState failed: %v", err)
	}

	// Test numeric value -> "230.5"
	err = c.PublishEntityState("dev1", "voltage", 230.5)
	if err != nil {
		t.Fatalf("PublishEntityState failed: %v", err)
	}

	if len(mockMqtt.published) != 3 {
		t.Fatalf("Expected 3 published messages, got %d", len(mockMqtt.published))
	}

	if string(mockMqtt.published[0].payload.([]byte)) != "ON" {
		t.Errorf("Expected ON, got %q", mockMqtt.published[0].payload)
	}
	if string(mockMqtt.published[1].payload.([]byte)) != "OFF" {
		t.Errorf("Expected OFF, got %q", mockMqtt.published[1].payload)
	}
	if string(mockMqtt.published[2].payload.([]byte)) != "230.5" {
		t.Errorf("Expected 230.5, got %q", mockMqtt.published[2].payload)
	}
}

func TestExtractEntityID(t *testing.T) {
	tests := []struct {
		topic    string
		prefix   string
		deviceID string
		want     string
	}{
		{topic: "snmp-bridge/dev123/outlet_1/set", prefix: "snmp-bridge", deviceID: "dev123", want: "outlet_1"},
		{topic: "prefix/dev/entity/set", prefix: "prefix", deviceID: "dev", want: "entity"},
	}

	for _, tt := range tests {
		got := extractEntityID(tt.topic, tt.prefix, tt.deviceID)
		if got != tt.want {
			t.Errorf("extractEntityID(%q) = %q, want %q", tt.topic, got, tt.want)
		}
	}
}
