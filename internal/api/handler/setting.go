package handler

import (
	"context"
	"net/http"
	"strconv"

	"snmp-mqtt-bridge/internal/config"
	"snmp-mqtt-bridge/internal/mqtt"
	"snmp-mqtt-bridge/internal/service"

	"github.com/gin-gonic/gin"
)

// MQTTReconnector interface for MQTT client reconnection
type MQTTReconnector interface {
	Reconnect(cfg *config.MQTTConfig) error
	IsConnected() bool
	GetConfig() *config.MQTTConfig
}

// SettingHandler handles setting-related HTTP requests
type SettingHandler struct {
	settingService *service.SettingService
	mqttClient     MQTTReconnector
}

// NewSettingHandler creates a new setting handler
func NewSettingHandler(settingService *service.SettingService) *SettingHandler {
	return &SettingHandler{settingService: settingService}
}

// SetMQTTClient sets the MQTT client for reconnection support
func (h *SettingHandler) SetMQTTClient(client *mqtt.Client) {
	h.mqttClient = client
}

// List returns all settings
func (h *SettingHandler) List(c *gin.Context) {
	settings, err := h.settingService.GetAll(c.Request.Context())
	if err != nil {
		RespondInternalError(c, err.Error())
		return
	}

	// Convert to map for easier frontend consumption
	settingsMap := make(map[string]string)
	for _, s := range settings {
		settingsMap[s.Key] = s.Value
	}

	RespondOK(c, settingsMap)
}

// Get returns a setting by key
func (h *SettingHandler) Get(c *gin.Context) {
	key := c.Param("key")

	value, err := h.settingService.Get(c.Request.Context(), key)
	if err != nil {
		RespondInternalError(c, err.Error())
		return
	}

	if value == "" {
		RespondNotFound(c, "Setting not found")
		return
	}

	RespondOK(c, gin.H{"key": key, "value": value})
}

// Set creates or updates a setting
func (h *SettingHandler) Set(c *gin.Context) {
	key := c.Param("key")

	var req struct {
		Value string `json:"value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}

	if err := h.settingService.Set(c.Request.Context(), key, req.Value); err != nil {
		RespondInternalError(c, err.Error())
		return
	}

	RespondOK(c, gin.H{"key": key, "value": req.Value})
}

// Delete deletes a setting
func (h *SettingHandler) Delete(c *gin.Context) {
	key := c.Param("key")

	if err := h.settingService.Delete(c.Request.Context(), key); err != nil {
		RespondInternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// ReconnectMQTT reconnects the MQTT client with settings from the database
func (h *SettingHandler) ReconnectMQTT(c *gin.Context) {
	if h.mqttClient == nil {
		RespondInternalError(c, "MQTT client not configured")
		return
	}

	// Load MQTT settings from database
	cfg, err := h.loadMQTTConfig(c.Request.Context())
	if err != nil {
		RespondInternalError(c, "Failed to load MQTT settings: "+err.Error())
		return
	}

	// Reconnect with new config
	if err := h.mqttClient.Reconnect(cfg); err != nil {
		RespondOK(c, gin.H{
			"success":   false,
			"connected": false,
			"message":   "Failed to connect: " + err.Error(),
		})
		return
	}

	RespondOK(c, gin.H{
		"success":   true,
		"connected": h.mqttClient.IsConnected(),
		"message":   "MQTT reconnected successfully",
	})
}

// GetMQTTStatus returns the current MQTT connection status
func (h *SettingHandler) GetMQTTStatus(c *gin.Context) {
	if h.mqttClient == nil {
		RespondOK(c, gin.H{
			"connected": false,
			"broker":    "",
		})
		return
	}

	cfg := h.mqttClient.GetConfig()
	RespondOK(c, gin.H{
		"connected": h.mqttClient.IsConnected(),
		"broker":    cfg.Broker,
		"port":      cfg.Port,
	})
}

// TestMQTTConnection tests MQTT connection with provided settings (without saving)
func (h *SettingHandler) TestMQTTConnection(c *gin.Context) {
	var req struct {
		Broker   string `json:"broker" binding:"required"`
		Port     int    `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}

	if req.Port == 0 {
		req.Port = 1883
	}

	// Create a temporary MQTT client to test connection
	cfg := &config.MQTTConfig{
		Broker:   req.Broker,
		Port:     req.Port,
		Username: req.Username,
		Password: req.Password,
		ClientID: "snmp-mqtt-bridge-test",
	}

	testClient := mqtt.NewClient(cfg)
	err := testClient.Connect()

	if err != nil {
		RespondOK(c, gin.H{
			"success": false,
			"message": "Connection failed: " + err.Error(),
		})
		return
	}

	// Connection successful, disconnect test client
	testClient.Disconnect()

	RespondOK(c, gin.H{
		"success": true,
		"message": "Connection successful",
	})
}

// loadMQTTConfig loads MQTT configuration from database settings
func (h *SettingHandler) loadMQTTConfig(ctx context.Context) (*config.MQTTConfig, error) {
	cfg := &config.MQTTConfig{
		Broker:          "localhost",
		Port:            1883,
		ClientID:        "snmp-mqtt-bridge",
		TopicPrefix:     "snmp-bridge",
		Discovery:       true,
		DiscoveryPrefix: "homeassistant",
	}

	if broker, _ := h.settingService.Get(ctx, "mqtt.broker"); broker != "" {
		cfg.Broker = broker
	}
	if portStr, _ := h.settingService.Get(ctx, "mqtt.port"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			cfg.Port = port
		}
	}
	if username, _ := h.settingService.Get(ctx, "mqtt.username"); username != "" {
		cfg.Username = username
	}
	if password, _ := h.settingService.Get(ctx, "mqtt.password"); password != "" {
		cfg.Password = password
	}
	if clientID, _ := h.settingService.Get(ctx, "mqtt.client_id"); clientID != "" {
		cfg.ClientID = clientID
	}
	if topicPrefix, _ := h.settingService.Get(ctx, "mqtt.topic_prefix"); topicPrefix != "" {
		cfg.TopicPrefix = topicPrefix
	}
	if discoveryPrefix, _ := h.settingService.Get(ctx, "mqtt.discovery_prefix"); discoveryPrefix != "" {
		cfg.DiscoveryPrefix = discoveryPrefix
	}

	return cfg, nil
}
