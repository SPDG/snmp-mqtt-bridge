package mqtt

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"snmp-mqtt-bridge/internal/config"
	"snmp-mqtt-bridge/internal/domain"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// CommandHandler is a function that handles MQTT commands
type CommandHandler func(deviceID, entityID string, payload []byte)

// Client wraps the MQTT client with convenience methods
type Client struct {
	cfg           *config.MQTTConfig
	client        mqtt.Client
	connected     bool
	mu            sync.RWMutex
	topicPrefix   string
	handlers      map[string]CommandHandler
	handlersMu    sync.RWMutex
}

// NewClient creates a new MQTT client
func NewClient(cfg *config.MQTTConfig) *Client {
	return &Client{
		cfg:         cfg,
		topicPrefix: cfg.TopicPrefix,
		handlers:    make(map[string]CommandHandler),
	}
}

// Connect establishes connection to the MQTT broker
func (c *Client) Connect() error {
	broker := fmt.Sprintf("tcp://%s:%d", c.cfg.Broker, c.cfg.Port)

	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(c.cfg.ClientID)
	opts.SetConnectTimeout(10 * time.Second)

	if c.cfg.Username != "" {
		opts.SetUsername(c.cfg.Username)
		opts.SetPassword(c.cfg.Password)
	}

	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(false) // Don't block on initial connect
	opts.SetConnectRetryInterval(5 * time.Second)
	opts.SetMaxReconnectInterval(5 * time.Minute)

	opts.SetOnConnectHandler(func(client mqtt.Client) {
		c.mu.Lock()
		c.connected = true
		c.mu.Unlock()
		log.Printf("MQTT connected to %s", broker)

		// Publish online status
		c.Publish(fmt.Sprintf("%s/bridge/status", c.topicPrefix), "online", true)

		// Resubscribe to command topics
		c.resubscribe()
	})

	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		c.mu.Lock()
		c.connected = false
		c.mu.Unlock()
		log.Printf("MQTT connection lost: %v", err)
	})

	// Set LWT (Last Will and Testament)
	opts.SetWill(
		fmt.Sprintf("%s/bridge/status", c.topicPrefix),
		"offline",
		1,
		true,
	)

	c.client = mqtt.NewClient(opts)

	token := c.client.Connect()
	if token.WaitTimeout(10*time.Second) && token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}

	return nil
}

// Disconnect closes the MQTT connection
func (c *Client) Disconnect() {
	if c.client != nil && c.client.IsConnected() {
		// Publish offline status
		c.Publish(fmt.Sprintf("%s/bridge/status", c.topicPrefix), "offline", true)
		c.client.Disconnect(250)
	}
}

// Reconnect disconnects and reconnects with new configuration
func (c *Client) Reconnect(cfg *config.MQTTConfig) error {
	c.mu.Lock()
	// Disconnect existing connection
	if c.client != nil && c.client.IsConnected() {
		c.client.Disconnect(250)
	}
	c.connected = false
	c.cfg = cfg
	c.topicPrefix = cfg.TopicPrefix
	c.mu.Unlock()

	// Connect with new config
	return c.Connect()
}

// GetConfig returns the current MQTT configuration
func (c *Client) GetConfig() *config.MQTTConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cfg
}

// IsConnected returns true if connected to the broker
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// Publish publishes a message to a topic
func (c *Client) Publish(topic string, payload interface{}, retain bool) error {
	if !c.client.IsConnected() {
		return fmt.Errorf("not connected to MQTT broker")
	}

	var data []byte
	switch v := payload.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		var err error
		data, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %w", err)
		}
	}

	token := c.client.Publish(topic, 0, retain, data)
	token.Wait()
	return token.Error()
}

// PublishState publishes device state to MQTT
func (c *Client) PublishState(deviceID string, state *domain.DeviceState) error {
	topic := fmt.Sprintf("%s/%s/state", c.topicPrefix, deviceID)
	return c.Publish(topic, state, false)
}

// PublishEntityState publishes a single entity state
func (c *Client) PublishEntityState(deviceID, entityID string, value interface{}) error {
	topic := fmt.Sprintf("%s/%s/%s/state", c.topicPrefix, deviceID, entityID)

	var payload string
	switch v := value.(type) {
	case string:
		payload = v
	case bool:
		if v {
			payload = "ON"
		} else {
			payload = "OFF"
		}
	default:
		payload = fmt.Sprintf("%v", v)
	}

	return c.Publish(topic, payload, true)
}

// Subscribe subscribes to a topic with a handler
func (c *Client) Subscribe(topic string, handler mqtt.MessageHandler) error {
	token := c.client.Subscribe(topic, 0, handler)
	token.Wait()
	return token.Error()
}

// SubscribeCommands subscribes to command topics for a device
func (c *Client) SubscribeCommands(deviceID string, handler CommandHandler) error {
	topic := fmt.Sprintf("%s/%s/+/set", c.topicPrefix, deviceID)

	c.handlersMu.Lock()
	c.handlers[deviceID] = handler
	c.handlersMu.Unlock()

	return c.Subscribe(topic, func(client mqtt.Client, msg mqtt.Message) {
		// Extract entity ID from topic
		// Topic format: prefix/deviceID/entityID/set
		entityID := extractEntityID(msg.Topic(), c.topicPrefix, deviceID)

		c.handlersMu.RLock()
		h, exists := c.handlers[deviceID]
		c.handlersMu.RUnlock()

		if exists {
			h(deviceID, entityID, msg.Payload())
		}
	})
}

// UnsubscribeCommands unsubscribes from command topics for a device
func (c *Client) UnsubscribeCommands(deviceID string) {
	topic := fmt.Sprintf("%s/%s/+/set", c.topicPrefix, deviceID)
	c.client.Unsubscribe(topic)

	c.handlersMu.Lock()
	delete(c.handlers, deviceID)
	c.handlersMu.Unlock()
}

func (c *Client) resubscribe() {
	c.handlersMu.RLock()
	defer c.handlersMu.RUnlock()

	for deviceID := range c.handlers {
		topic := fmt.Sprintf("%s/%s/+/set", c.topicPrefix, deviceID)
		c.client.Subscribe(topic, 0, func(client mqtt.Client, msg mqtt.Message) {
			entityID := extractEntityID(msg.Topic(), c.topicPrefix, deviceID)

			c.handlersMu.RLock()
			h, exists := c.handlers[deviceID]
			c.handlersMu.RUnlock()

			if exists {
				h(deviceID, entityID, msg.Payload())
			}
		})
	}
}

func extractEntityID(topic, prefix, deviceID string) string {
	// Topic format: prefix/deviceID/entityID/set
	// We need to extract entityID
	prefixLen := len(prefix) + 1 + len(deviceID) + 1
	remaining := topic[prefixLen:]
	// remaining is "entityID/set"
	for i := 0; i < len(remaining); i++ {
		if remaining[i] == '/' {
			return remaining[:i]
		}
	}
	return remaining
}
