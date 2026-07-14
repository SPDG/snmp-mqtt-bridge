package mqtt

import (
	"context"
	"fmt"
	"time"

	"snmp-mqtt-bridge/internal/domain"
	"snmp-mqtt-bridge/internal/repository"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type mockToken struct {
	err      error
	complete bool
}

func (t *mockToken) Wait() bool {
	return true
}

func (t *mockToken) WaitTimeout(d time.Duration) bool {
	return t.complete
}

func (t *mockToken) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (t *mockToken) Error() error {
	return t.err
}

type mockMqttClient struct {
	mqtt.Client
	connected    bool
	publishToken mqtt.Token
	published    []struct {
		topic   string
		qos     byte
		retain  bool
		payload interface{}
	}
	subscribed []struct {
		topic string
		qos   byte
	}
}

func (m *mockMqttClient) IsConnected() bool {
	return m.connected
}

func (m *mockMqttClient) Connect() mqtt.Token {
	m.connected = true
	return &mockToken{complete: true}
}

func (m *mockMqttClient) Disconnect(quiesce uint) {
	m.connected = false
}

func (m *mockMqttClient) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	m.published = append(m.published, struct {
		topic   string
		qos     byte
		retain  bool
		payload interface{}
	}{topic, qos, retained, payload})
	if m.publishToken != nil {
		return m.publishToken
	}
	return &mockToken{complete: true}
}

func (m *mockMqttClient) Subscribe(topic string, qos byte, callback mqtt.MessageHandler) mqtt.Token {
	m.subscribed = append(m.subscribed, struct {
		topic string
		qos   byte
	}{topic, qos})
	return &mockToken{complete: true}
}

func (m *mockMqttClient) Unsubscribe(topics ...string) mqtt.Token {
	return &mockToken{complete: true}
}

type mockProfileRepo struct {
	repository.ProfileRepository
	profiles map[string]*domain.Profile
}

func (r *mockProfileRepo) GetByID(ctx context.Context, id string) (*domain.Profile, error) {
	if p, ok := r.profiles[id]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("profile not found")
}
