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
	cfg               *config.MQTTConfig
	client            mqtt.Client
	connected         bool
	mu                sync.RWMutex
	topicPrefix       string
	handlers          map[string]CommandHandler
	handlersMu        sync.RWMutex
	onConnectHandlers []func()
	generation        uint64
}

const mqttOperationTimeout = 5 * time.Second
const mqttStartupWaitTimeout = 2 * time.Second

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
	return c.connect(true)
}

// ConnectOnce establishes a single MQTT connection attempt without background initial retry.
func (c *Client) ConnectOnce() error {
	return c.connect(false)
}

func (c *Client) connect(connectRetry bool) error {
	c.mu.Lock()
	c.generation++
	generation := c.generation
	c.connected = false
	cfg := *c.cfg
	c.mu.Unlock()

	broker := fmt.Sprintf("tcp://%s:%d", cfg.Broker, cfg.Port)

	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(cfg.ClientID)
	opts.SetConnectTimeout(10 * time.Second)

	if cfg.Username != "" {
		opts.SetUsername(cfg.Username)
		opts.SetPassword(cfg.Password)
	}

	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(connectRetry)
	opts.SetConnectRetryInterval(5 * time.Second)
	opts.SetMaxReconnectInterval(5 * time.Minute)

	opts.SetOnConnectHandler(func(client mqtt.Client) {
		c.mu.Lock()
		if generation != c.generation {
			c.mu.Unlock()
			return
		}
		c.connected = true
		c.mu.Unlock()
		log.Printf("MQTT connected to %s", broker)

		// Publish online status
		c.Publish(fmt.Sprintf("%s/bridge/status", cfg.TopicPrefix), "online", true)

		// Resubscribe to command topics
		c.resubscribe()
		c.runOnConnectHandlers()
	})

	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		c.mu.Lock()
		if generation != c.generation {
			c.mu.Unlock()
			return
		}
		c.connected = false
		c.mu.Unlock()
		log.Printf("MQTT connection lost: %v", err)
	})

	// Set LWT (Last Will and Testament)
	opts.SetWill(
		fmt.Sprintf("%s/bridge/status", cfg.TopicPrefix),
		"offline",
		1,
		true,
	)

	client := mqtt.NewClient(opts)
	c.mu.Lock()
	if generation != c.generation {
		c.mu.Unlock()
		client.Disconnect(0)
		return fmt.Errorf("MQTT connection attempt superseded")
	}
	c.client = client
	c.mu.Unlock()

	token := client.Connect()
	waitTimeout := mqttStartupWaitTimeout
	if !connectRetry {
		waitTimeout = 10 * time.Second
	}
	if !token.WaitTimeout(waitTimeout) {
		if connectRetry {
			return fmt.Errorf("timed out connecting to MQTT broker; retrying in background")
		}
		client.Disconnect(0)
		return fmt.Errorf("timed out connecting to MQTT broker")
	}
	if token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}

	return nil
}

// AddOnConnectHandler registers a callback that runs after every successful MQTT connection.
func (c *Client) AddOnConnectHandler(handler func()) {
	c.mu.Lock()
	c.onConnectHandlers = append(c.onConnectHandlers, handler)
	connected := c.connected
	c.mu.Unlock()

	if connected {
		go handler()
	}
}

func (c *Client) runOnConnectHandlers() {
	c.mu.RLock()
	handlers := append([]func(){}, c.onConnectHandlers...)
	c.mu.RUnlock()

	for _, handler := range handlers {
		handler()
	}
}

// Disconnect closes the MQTT connection
func (c *Client) Disconnect() {
	c.mu.Lock()
	client := c.client
	topicPrefix := c.topicPrefix
	c.generation++
	c.connected = false
	c.mu.Unlock()

	if client != nil && client.IsConnected() {
		// Publish offline status
		token := client.Publish(fmt.Sprintf("%s/bridge/status", topicPrefix), 0, true, []byte("offline"))
		token.WaitTimeout(mqttOperationTimeout)
	}
	if client != nil {
		client.Disconnect(250)
	}
}

// Reconnect disconnects and reconnects with new configuration
func (c *Client) Reconnect(cfg *config.MQTTConfig) error {
	c.mu.Lock()
	oldClient := c.client
	c.generation++
	c.connected = false
	c.cfg = cfg
	c.topicPrefix = cfg.TopicPrefix
	c.mu.Unlock()

	// Stop background reconnect attempts from the previous client as well.
	if oldClient != nil {
		go oldClient.Disconnect(250)
	}

	// Connect with new config
	return c.Connect()
}

// GetConfig returns the current MQTT configuration
func (c *Client) GetConfig() *config.MQTTConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cfg := *c.cfg
	return &cfg
}

// IsConnected returns true if connected to the broker
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// Publish publishes a message to a topic
func (c *Client) Publish(topic string, payload interface{}, retain bool) error {
	c.mu.RLock()
	client := c.client
	connected := c.connected
	c.mu.RUnlock()
	if client == nil || !connected || !client.IsConnected() {
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

	token := client.Publish(topic, 0, retain, data)
	if !token.WaitTimeout(mqttOperationTimeout) {
		return fmt.Errorf("timed out publishing to MQTT topic %s", topic)
	}
	return token.Error()
}

// PublishState publishes device state to MQTT
func (c *Client) PublishState(deviceID string, state *domain.DeviceState) error {
	topic := fmt.Sprintf("%s/%s/state", c.getTopicPrefix(), deviceID)
	return c.Publish(topic, state, false)
}

// PublishEntityState publishes a single entity state
func (c *Client) PublishEntityState(deviceID, entityID string, value interface{}) error {
	topic := fmt.Sprintf("%s/%s/%s/state", c.getTopicPrefix(), deviceID, entityID)

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
	c.mu.RLock()
	client := c.client
	connected := c.connected
	c.mu.RUnlock()
	if client == nil || !connected || !client.IsConnected() {
		return fmt.Errorf("not connected to MQTT broker")
	}

	token := client.Subscribe(topic, 0, handler)
	if !token.WaitTimeout(mqttOperationTimeout) {
		return fmt.Errorf("timed out subscribing to MQTT topic %s", topic)
	}
	return token.Error()
}

// SubscribeCommands subscribes to command topics for a device
func (c *Client) SubscribeCommands(deviceID string, handler CommandHandler) error {
	topic := fmt.Sprintf("%s/%s/+/set", c.getTopicPrefix(), deviceID)

	c.handlersMu.Lock()
	c.handlers[deviceID] = handler
	c.handlersMu.Unlock()
	if !c.IsConnected() {
		return nil
	}

	return c.Subscribe(topic, c.commandMessageHandler(deviceID))
}

// UnsubscribeCommands unsubscribes from command topics for a device
func (c *Client) UnsubscribeCommands(deviceID string) {
	topic := fmt.Sprintf("%s/%s/+/set", c.getTopicPrefix(), deviceID)
	c.mu.RLock()
	client := c.client
	connected := c.connected
	c.mu.RUnlock()
	if client != nil && connected && client.IsConnected() {
		token := client.Unsubscribe(topic)
		if !token.WaitTimeout(mqttOperationTimeout) {
			log.Printf("Timed out unsubscribing from MQTT topic %s", topic)
		}
	}

	c.handlersMu.Lock()
	delete(c.handlers, deviceID)
	c.handlersMu.Unlock()
}

func (c *Client) resubscribe() {
	c.handlersMu.RLock()
	deviceIDs := make([]string, 0, len(c.handlers))
	for deviceID := range c.handlers {
		deviceIDs = append(deviceIDs, deviceID)
	}
	c.handlersMu.RUnlock()

	topicPrefix := c.getTopicPrefix()
	for _, deviceID := range deviceIDs {
		topic := fmt.Sprintf("%s/%s/+/set", topicPrefix, deviceID)
		if err := c.Subscribe(topic, c.commandMessageHandler(deviceID)); err != nil {
			log.Printf("Failed to resubscribe to MQTT topic %s: %v", topic, err)
		}
	}
}

func (c *Client) getTopicPrefix() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.topicPrefix
}

func (c *Client) commandMessageHandler(deviceID string) mqtt.MessageHandler {
	return func(client mqtt.Client, msg mqtt.Message) {
		c.mu.RLock()
		topicPrefix := c.topicPrefix
		c.mu.RUnlock()
		entityID := extractEntityID(msg.Topic(), topicPrefix, deviceID)

		c.handlersMu.RLock()
		h, exists := c.handlers[deviceID]
		c.handlersMu.RUnlock()

		if exists {
			h(deviceID, entityID, msg.Payload())
		}
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
