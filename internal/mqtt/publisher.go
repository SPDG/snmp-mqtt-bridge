package mqtt

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"snmp-mqtt-bridge/internal/domain"
	"snmp-mqtt-bridge/internal/repository"
	"snmp-mqtt-bridge/internal/service"

	"github.com/gosnmp/gosnmp"
)

// deviceInfo holds device and profile information for MQTT publishing
type deviceInfo struct {
	device  *domain.Device
	profile *domain.Profile
}

// Publisher handles publishing device states to MQTT
type Publisher struct {
	client      *Client
	discovery   *Discovery
	poller      *service.PollerService
	profileRepo repository.ProfileRepository
	devices     map[string]*deviceInfo
	devicesMu   sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewPublisher creates a new MQTT publisher
func NewPublisher(
	client *Client,
	discovery *Discovery,
	poller *service.PollerService,
	profileRepo repository.ProfileRepository,
) *Publisher {
	ctx, cancel := context.WithCancel(context.Background())

	return &Publisher{
		client:      client,
		discovery:   discovery,
		poller:      poller,
		profileRepo: profileRepo,
		devices:     make(map[string]*deviceInfo),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start starts the publisher
func (p *Publisher) Start() error {
	// Subscribe to poller events
	eventChan := p.poller.Subscribe()

	go p.handleEvents(eventChan)

	log.Println("MQTT publisher started")
	return nil
}

// Stop stops the publisher
func (p *Publisher) Stop() {
	p.cancel()
}

// RegisterDevice registers a device for MQTT publishing and discovery
func (p *Publisher) RegisterDevice(device *domain.Device) error {
	var profile *domain.Profile
	if device.ProfileID != "" {
		var err error
		profile, err = p.profileRepo.GetByID(context.Background(), device.ProfileID)
		if err != nil {
			log.Printf("Failed to get profile for device %s: %v", device.ID, err)
		}
	}

	p.devicesMu.Lock()
	p.devices[device.ID] = &deviceInfo{
		device:  device,
		profile: profile,
	}
	p.devicesMu.Unlock()

	// Publish discovery config
	if profile != nil && p.client.IsConnected() {
		if err := p.discovery.PublishDevice(device, profile); err != nil {
			log.Printf("Failed to publish discovery for device %s: %v", device.ID, err)
		}
	}

	// Subscribe to commands
	if err := p.client.SubscribeCommands(device.ID, p.handleCommand); err != nil {
		log.Printf("Failed to subscribe to commands for device %s: %v", device.ID, err)
	}

	return nil
}

// UnregisterDevice removes a device from MQTT publishing
func (p *Publisher) UnregisterDevice(deviceID string) error {
	p.devicesMu.Lock()
	info := p.devices[deviceID]
	delete(p.devices, deviceID)
	p.devicesMu.Unlock()

	// Remove discovery config
	if info != nil && info.profile != nil && p.client.IsConnected() {
		if err := p.discovery.RemoveDevice(deviceID, info.profile); err != nil {
			log.Printf("Failed to remove discovery for device %s: %v", deviceID, err)
		}
	}

	// Unsubscribe from commands
	p.client.UnsubscribeCommands(deviceID)

	return nil
}

func (p *Publisher) handleEvents(eventChan chan service.StateUpdateEvent) {
	for {
		select {
		case <-p.ctx.Done():
			return
		case event, ok := <-eventChan:
			if !ok {
				return
			}
			p.publishState(event)
		}
	}
}

func (p *Publisher) publishState(event service.StateUpdateEvent) {
	if !p.client.IsConnected() {
		return
	}

	p.devicesMu.RLock()
	info := p.devices[event.DeviceID]
	p.devicesMu.RUnlock()

	if info == nil || info.profile == nil {
		return
	}

	device := info.device
	profile := info.profile

	// Calculate Active Power if it's 0 or missing (P = V × I)
	p.calculatePowerIfNeeded(event.Values)

	// Check for source names to update select options
	sourceAName, hasSourceA := event.Values["Source A Name"]
	sourceBName, hasSourceB := event.Values["Source B Name"]
	if hasSourceA && hasSourceB {
		p.updateSelectOptionsWithSourceNames(device, profile,
			fmt.Sprintf("%v", sourceAName), fmt.Sprintf("%v", sourceBName))
	}

	// Publish individual entity states
	for _, mapping := range profile.OIDMappings {
		value, exists := event.Values[mapping.Name]
		if !exists {
			// Try by OID
			value, exists = event.Values[mapping.OID]
		}

		if exists {
			entityID := sanitizeEntityID(mapping.Name)

			// Convert value for binary sensors and switches
			publishValue := value
			if mapping.HAComponent == domain.HAComponentBinarySensor {
				publishValue = convertToBinarySensorValue(value, mapping.DeviceClass)
			} else if mapping.HAComponent == domain.HAComponentSwitch {
				publishValue = convertToSwitchValue(value)
			}

			// For select entities showing "Selected Source" or "Preferred Source",
			// replace generic names with actual source names
			if mapping.HAComponent == domain.HAComponentSelect ||
			   (mapping.Name == "Selected Source" || mapping.Name == "Preferred Source") {
				if hasSourceA && hasSourceB {
					strVal := fmt.Sprintf("%v", publishValue)
					if strVal == "Source A" || strVal == "1" {
						publishValue = sourceAName
					} else if strVal == "Source B" || strVal == "2" {
						publishValue = sourceBName
					}
				}
			}

			if err := p.client.PublishEntityState(event.DeviceID, entityID, publishValue); err != nil {
				log.Printf("Failed to publish state for %s/%s: %v", event.DeviceID, entityID, err)
			}
		}
	}

	// Publish full state
	state := &domain.DeviceState{
		DeviceID: event.DeviceID,
		Online:   event.Online,
		LastPoll: event.Timestamp,
		Values:   event.Values,
	}

	if err := p.client.PublishState(event.DeviceID, state); err != nil {
		log.Printf("Failed to publish full state for %s: %v", event.DeviceID, err)
	}
}

func (p *Publisher) handleCommand(deviceID, entityID string, payload []byte) {
	log.Printf("Received command for %s/%s: %s", deviceID, entityID, string(payload))

	// Get device info
	p.devicesMu.RLock()
	info := p.devices[deviceID]
	p.devicesMu.RUnlock()

	if info == nil || info.profile == nil || info.device == nil {
		log.Printf("Device or profile not found for %s", deviceID)
		return
	}

	device := info.device
	profile := info.profile

	// Find the mapping for this entity
	var mapping *domain.OIDMapping
	for i := range profile.OIDMappings {
		m := &profile.OIDMappings[i]
		if sanitizeEntityID(m.Name) == entityID {
			mapping = m
			break
		}
	}

	if mapping == nil {
		log.Printf("Mapping not found for entity %s", entityID)
		return
	}

	if !mapping.Writable {
		log.Printf("Mapping %s is not writable", mapping.Name)
		return
	}

	// Determine the OID to write to
	writeOID := mapping.WriteOID
	if writeOID == "" {
		writeOID = mapping.OID
	}

	// Convert payload to SNMP value
	payloadStr := strings.TrimSpace(string(payload))
	var snmpValue interface{}
	var err error

	// Handle composite_switch type (Energenie-style comma-separated outlet status)
	if mapping.Type == domain.OIDTypeCompositeSwitch {
		snmpValue, err = p.convertCompositePayloadToSNMPValue(device, payloadStr, mapping)
		if err != nil {
			log.Printf("Failed to convert composite payload: %v", err)
			return
		}
	} else {
		snmpValue, err = p.convertPayloadToSNMPValue(payloadStr, mapping)
		if err != nil {
			log.Printf("Failed to convert payload: %v", err)
			return
		}
	}

	// Send SNMP SET command
	if err := p.sendSNMPSet(device, writeOID, snmpValue); err != nil {
		log.Printf("Failed to send SNMP SET: %v", err)
		return
	}

	log.Printf("SNMP SET successful for %s/%s: %s -> %v", deviceID, entityID, payloadStr, snmpValue)

	// Trigger immediate poll to confirm state change
	p.poller.TriggerPoll(deviceID)
}

// convertPayloadToSNMPValue converts MQTT payload to appropriate SNMP value
func (p *Publisher) convertPayloadToSNMPValue(payload string, mapping *domain.OIDMapping) (interface{}, error) {
	payloadUpper := strings.ToUpper(payload)

	// For switches (ON/OFF -> integer)
	if mapping.HAComponent == domain.HAComponentSwitch {
		// Check enum_values to find the correct integer value
		if mapping.EnumValues != nil {
			for k, v := range mapping.EnumValues {
				if strings.EqualFold(v, "On") && payloadUpper == "ON" {
					return k, nil
				}
				if strings.EqualFold(v, "Off") && payloadUpper == "OFF" {
					return k, nil
				}
			}
		}
		// Default: ON=1, OFF=2 (common for APC PDUs)
		if payloadUpper == "ON" {
			return 1, nil
		}
		return 2, nil
	}

	// For select entities, find the enum value
	if mapping.HAComponent == domain.HAComponentSelect {
		if mapping.EnumValues != nil {
			for k, v := range mapping.EnumValues {
				if strings.EqualFold(v, payload) {
					return k, nil
				}
			}
		}
		return nil, fmt.Errorf("unknown select value: %s", payload)
	}

	// For numbers, return as integer
	if mapping.HAComponent == domain.HAComponentNumber {
		var val int
		if _, err := fmt.Sscanf(payload, "%d", &val); err != nil {
			return nil, fmt.Errorf("invalid number: %s", payload)
		}
		return val, nil
	}

	// Default: return as string
	return payload, nil
}

// convertCompositePayloadToSNMPValue handles composite_switch type - modifies specific index in comma-separated string
func (p *Publisher) convertCompositePayloadToSNMPValue(device *domain.Device, payload string, mapping *domain.OIDMapping) (string, error) {
	payloadUpper := strings.ToUpper(payload)

	// Determine the value to set at the index
	var newValue string
	if mapping.EnumValues != nil {
		for k, v := range mapping.EnumValues {
			if strings.EqualFold(v, "On") && payloadUpper == "ON" {
				newValue = fmt.Sprintf("%d", k)
				break
			}
			if strings.EqualFold(v, "Off") && payloadUpper == "OFF" {
				newValue = fmt.Sprintf("%d", k)
				break
			}
		}
	}
	if newValue == "" {
		// Default: ON=1, OFF=0 (Energenie style)
		if payloadUpper == "ON" {
			newValue = "1"
		} else {
			newValue = "0"
		}
	}

	// Read current value from device
	currentValue, err := p.readSNMPValue(device, mapping.OID)
	if err != nil {
		return "", fmt.Errorf("failed to read current value: %w", err)
	}

	currentStr, ok := currentValue.(string)
	if !ok {
		return "", fmt.Errorf("current value is not a string: %T", currentValue)
	}

	separator := mapping.CompositeSeparator
	if separator == "" {
		separator = ","
	}

	parts := strings.Split(currentStr, separator)
	if mapping.CompositeIndex >= len(parts) {
		return "", fmt.Errorf("composite index %d out of range (len=%d)", mapping.CompositeIndex, len(parts))
	}

	// Modify the specific index
	parts[mapping.CompositeIndex] = newValue

	return strings.Join(parts, separator), nil
}

// readSNMPValue reads a single OID value from the device
func (p *Publisher) readSNMPValue(device *domain.Device, oid string) (interface{}, error) {
	client := p.createSNMPClient(device)

	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Conn.Close()

	result, err := client.Get([]string{oid})
	if err != nil {
		return nil, fmt.Errorf("SNMP GET failed: %w", err)
	}

	if len(result.Variables) == 0 {
		return nil, fmt.Errorf("no result for OID %s", oid)
	}

	variable := result.Variables[0]
	switch variable.Type {
	case gosnmp.OctetString:
		return string(variable.Value.([]byte)), nil
	case gosnmp.Integer:
		return variable.Value, nil
	default:
		return variable.Value, nil
	}
}

// sendSNMPSet sends an SNMP SET command to the device
func (p *Publisher) sendSNMPSet(device *domain.Device, oid string, value interface{}) error {
	client := p.createSNMPClientForWrite(device)

	if err := client.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Conn.Close()

	// Determine PDU type based on value type
	var pdu gosnmp.SnmpPDU
	pdu.Name = oid

	switch v := value.(type) {
	case int:
		pdu.Type = gosnmp.Integer
		pdu.Value = v
	case string:
		pdu.Type = gosnmp.OctetString
		pdu.Value = v
	default:
		return fmt.Errorf("unsupported value type: %T", value)
	}

	_, err := client.Set([]gosnmp.SnmpPDU{pdu})
	if err != nil {
		return fmt.Errorf("SNMP SET failed: %w", err)
	}

	return nil
}

// createSNMPClient creates an SNMP client for the device (for reading)
func (p *Publisher) createSNMPClient(device *domain.Device) *gosnmp.GoSNMP {
	return p.createSNMPClientWithCommunity(device, device.Community)
}

// createSNMPClientForWrite creates an SNMP client for SET operations, using write community if available
func (p *Publisher) createSNMPClientForWrite(device *domain.Device) *gosnmp.GoSNMP {
	community := device.Community
	if device.WriteCommunity != "" {
		community = device.WriteCommunity
	}
	return p.createSNMPClientWithCommunity(device, community)
}

// createSNMPClientWithCommunity creates an SNMP client with specified community
func (p *Publisher) createSNMPClientWithCommunity(device *domain.Device, community string) *gosnmp.GoSNMP {
	client := &gosnmp.GoSNMP{
		Target:  device.IPAddress,
		Port:    uint16(device.Port),
		Timeout: time.Second * 5,
		Retries: 2,
	}

	switch device.SNMPVersion {
	case domain.SNMPv1:
		client.Version = gosnmp.Version1
		client.Community = community
	case domain.SNMPv3:
		client.Version = gosnmp.Version3
		client.SecurityModel = gosnmp.UserSecurityModel
		client.MsgFlags = gosnmp.NoAuthNoPriv
		client.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName: community,
		}
	default:
		client.Version = gosnmp.Version2c
		client.Community = community
	}

	return client
}

// updateSelectOptionsWithSourceNames updates the discovery config for select entities
// to use actual source names instead of generic "Source A"/"Source B"
func (p *Publisher) updateSelectOptionsWithSourceNames(device *domain.Device, profile *domain.Profile, sourceAName, sourceBName string) {
	if sourceAName == "" || sourceBName == "" {
		return
	}

	for _, mapping := range profile.OIDMappings {
		// Only update select entities for source selection
		if mapping.HAComponent != domain.HAComponentSelect {
			continue
		}
		if mapping.Name != "Preferred Source" && mapping.Name != "Selected Source" {
			continue
		}

		// Re-publish discovery with updated options
		p.discovery.UpdateSelectOptions(device, profile, mapping, []string{sourceAName, sourceBName})
	}
}

// convertToSwitchValue converts a value to ON/OFF for switches
// Handles: "On"/"Off" (enum), 1/2 (SNMP integer), "1"/"2" (string)
func convertToSwitchValue(value interface{}) string {
	strValue := fmt.Sprintf("%v", value)
	strLower := strings.ToLower(strValue)

	// Map "on" values to ON
	onValues := map[string]bool{
		"on":  true,
		"1":   true,
		"true": true,
	}

	if onValues[strLower] {
		return "ON"
	}
	return "OFF"
}

// calculatePowerIfNeeded calculates Active Power from Voltage × Current if not available
func (p *Publisher) calculatePowerIfNeeded(values map[string]interface{}) {
	// Check if Active Power is 0 or missing
	activePower, hasPower := values["Active Power"]
	powerIsZero := !hasPower

	if hasPower {
		switch v := activePower.(type) {
		case float64:
			powerIsZero = v == 0
		case int:
			powerIsZero = v == 0
		case string:
			powerIsZero = v == "0" || v == ""
		}
	}

	if powerIsZero {
		// Try to calculate from Voltage × Total Current
		voltage := toFloat64(values["Voltage"])
		current := toFloat64(values["Total Current"])

		if voltage > 0 && current > 0 {
			calculatedPower := voltage * current
			// Round to 1 decimal place
			values["Active Power"] = float64(int(calculatedPower*10)) / 10
		}
	}
}

// toFloat64 converts various types to float64
func toFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		var f float64
		fmt.Sscanf(val, "%f", &f)
		return f
	default:
		return 0
	}
}

// convertToBinarySensorValue converts a value to ON/OFF for binary sensors
// For device_class: problem, safety, power - "good" states should be OFF, "bad" states should be ON
func convertToBinarySensorValue(value interface{}, deviceClass string) string {
	strValue := fmt.Sprintf("%v", value)
	strLower := strings.ToLower(strValue)

	// Define "good" values that mean no problem (OFF for problem sensors)
	goodValues := map[string]bool{
		"ok":        true,
		"normal":    true,
		"redundant": true,
		"on":        true,
		"connected": true,
		"online":    true,
		"healthy":   true,
		"good":      true,
		"active":    true,
	}

	// For device_class: problem, safety - good values = OFF (no problem)
	// For device_class: power - we might want different logic
	if deviceClass == "problem" || deviceClass == "safety" {
		if goodValues[strLower] {
			return "OFF"
		}
		return "ON"
	}

	// For device_class: power - "OK" means power is good (ON)
	if deviceClass == "power" {
		if goodValues[strLower] {
			return "ON"
		}
		return "OFF"
	}

	// Default: good values = OFF
	if goodValues[strLower] {
		return "OFF"
	}
	return "ON"
}
