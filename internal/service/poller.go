package service

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"snmp-mqtt-bridge/internal/domain"
	"snmp-mqtt-bridge/internal/repository"

	"github.com/gosnmp/gosnmp"
)

// StateUpdateEvent is sent when a device state changes
type StateUpdateEvent struct {
	DeviceID  string                 `json:"device_id"`
	Timestamp time.Time              `json:"timestamp"`
	Values    map[string]interface{} `json:"values"`
	Online    bool                   `json:"online"`
}

// PollerService manages SNMP polling for all devices
type PollerService struct {
	deviceRepo  repository.DeviceRepository
	profileRepo repository.ProfileRepository

	devices     map[string]*devicePoller
	devicesMu   sync.RWMutex

	states   map[string]*domain.DeviceState
	statesMu sync.RWMutex

	subscribers []chan StateUpdateEvent
	subMu       sync.RWMutex

	defaultInterval time.Duration
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
}

type devicePoller struct {
	device      *domain.Device
	profile     *domain.Profile
	client      *gosnmp.GoSNMP
	interval    time.Duration
	stopCh      chan struct{}
	triggerCh   chan struct{}
	pollCount   int
	missingOIDs map[string]bool // OIDs that returned NoSuchInstance - skip polling these
}

// NewPollerService creates a new poller service
func NewPollerService(deviceRepo repository.DeviceRepository, profileRepo repository.ProfileRepository, defaultInterval time.Duration) *PollerService {
	ctx, cancel := context.WithCancel(context.Background())

	return &PollerService{
		deviceRepo:      deviceRepo,
		profileRepo:     profileRepo,
		devices:         make(map[string]*devicePoller),
		states:          make(map[string]*domain.DeviceState),
		subscribers:     make([]chan StateUpdateEvent, 0),
		defaultInterval: defaultInterval,
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Start starts the poller service
func (s *PollerService) Start(ctx context.Context) error {
	// Load enabled devices
	devices, err := s.deviceRepo.GetEnabled(ctx)
	if err != nil {
		return fmt.Errorf("failed to load devices: %w", err)
	}

	for i := range devices {
		s.AddDevice(&devices[i])
	}

	log.Printf("Poller started with %d devices", len(devices))
	return nil
}

// Stop stops the poller service
func (s *PollerService) Stop() {
	s.cancel()

	s.devicesMu.Lock()
	for _, dp := range s.devices {
		close(dp.stopCh)
	}
	s.devicesMu.Unlock()

	s.wg.Wait()

	// Close subscriber channels
	s.subMu.Lock()
	for _, ch := range s.subscribers {
		close(ch)
	}
	s.subscribers = nil
	s.subMu.Unlock()

	log.Println("Poller stopped")
}

// Subscribe returns a channel for state update events
func (s *PollerService) Subscribe() chan StateUpdateEvent {
	ch := make(chan StateUpdateEvent, 100)
	s.subMu.Lock()
	s.subscribers = append(s.subscribers, ch)
	s.subMu.Unlock()
	return ch
}

// AddDevice adds a device to the poller
func (s *PollerService) AddDevice(device *domain.Device) {
	s.devicesMu.Lock()
	defer s.devicesMu.Unlock()

	if _, exists := s.devices[device.ID]; exists {
		return
	}

	// Get profile
	var profile *domain.Profile
	if device.ProfileID != "" {
		p, err := s.profileRepo.GetByID(context.Background(), device.ProfileID)
		if err == nil {
			profile = p
		}
	}

	interval := s.defaultInterval
	if device.PollInterval > 0 {
		interval = time.Duration(device.PollInterval) * time.Second
	}

	dp := &devicePoller{
		device:      device,
		profile:     profile,
		interval:    interval,
		stopCh:      make(chan struct{}),
		triggerCh:   make(chan struct{}, 1),
		missingOIDs: make(map[string]bool),
	}

	s.devices[device.ID] = dp

	// Initialize state
	s.statesMu.Lock()
	s.states[device.ID] = &domain.DeviceState{
		DeviceID: device.ID,
		Online:   false,
		Values:   make(map[string]interface{}),
	}
	s.statesMu.Unlock()

	// Start polling goroutine
	s.wg.Add(1)
	go s.pollDevice(dp)
}

// UpdateDevice updates a device in the poller
func (s *PollerService) UpdateDevice(device *domain.Device) {
	s.RemoveDevice(device.ID)
	if device.Enabled {
		s.AddDevice(device)
	}
}

// RemoveDevice removes a device from the poller
func (s *PollerService) RemoveDevice(id string) {
	s.devicesMu.Lock()
	dp, exists := s.devices[id]
	if exists {
		close(dp.stopCh)
		delete(s.devices, id)
	}
	s.devicesMu.Unlock()

	s.statesMu.Lock()
	delete(s.states, id)
	s.statesMu.Unlock()
}

// GetDeviceState returns the current state of a device
func (s *PollerService) GetDeviceState(id string) *domain.DeviceState {
	s.statesMu.RLock()
	defer s.statesMu.RUnlock()

	if state, exists := s.states[id]; exists {
		return state
	}
	return nil
}

// GetAllDeviceStates returns all device states
func (s *PollerService) GetAllDeviceStates() map[string]*domain.DeviceState {
	s.statesMu.RLock()
	defer s.statesMu.RUnlock()

	result := make(map[string]*domain.DeviceState, len(s.states))
	for k, v := range s.states {
		result[k] = v
	}
	return result
}

// TriggerPoll triggers an immediate poll for a device
func (s *PollerService) TriggerPoll(deviceID string) {
	s.devicesMu.RLock()
	dp, exists := s.devices[deviceID]
	s.devicesMu.RUnlock()

	if exists {
		select {
		case dp.triggerCh <- struct{}{}:
		default:
			// Channel full, poll already pending
		}
	}
}

func (s *PollerService) pollDevice(dp *devicePoller) {
	defer s.wg.Done()

	ticker := time.NewTicker(dp.interval)
	defer ticker.Stop()

	// Initial poll
	s.doPoll(dp)

	for {
		select {
		case <-dp.stopCh:
			if dp.client != nil && dp.client.Conn != nil {
				dp.client.Conn.Close()
			}
			return
		case <-s.ctx.Done():
			return
		case <-dp.triggerCh:
			s.doPoll(dp)
		case <-ticker.C:
			s.doPoll(dp)
		}
	}
}

func (s *PollerService) doPoll(dp *devicePoller) {
	dp.pollCount++

	// Create SNMP client if not exists
	if dp.client == nil {
		dp.client = createSNMPClient(dp.device.IPAddress, dp.device.Port, dp.device.Community, dp.device.SNMPVersion)
	}

	// Connect if not connected
	if dp.client.Conn == nil {
		if err := dp.client.Connect(); err != nil {
			s.updateState(dp.device.ID, nil, false, []string{err.Error()})
			return
		}
	}

	// Get OIDs to poll
	oids := s.getOIDsToPoll(dp)
	if len(oids) == 0 {
		// No profile, just do a basic poll
		oids = []string{
			".1.3.6.1.2.1.1.1.0", // sysDescr
			".1.3.6.1.2.1.1.3.0", // sysUpTime
		}
	}

	// Debug: log which OIDs are being polled for this device
	if dp.pollCount <= 3 {
		log.Printf("[DEBUG] Polling %d OIDs for device %s (poll #%d)", len(oids), dp.device.ID, dp.pollCount)
	}

	// Build OID to mappings lookup for faster matching
	// Multiple mappings can share the same OID (e.g., composite_switch for individual outlets)
	oidToMappings := make(map[string][]*domain.OIDMapping)
	if dp.profile != nil {
		for i := range dp.profile.OIDMappings {
			mapping := &dp.profile.OIDMappings[i]
			normalizedOID := normalizeOID(mapping.OID)
			oidToMappings[normalizedOID] = append(oidToMappings[normalizedOID], mapping)
		}
	}

	// Determine batch size based on SNMP version
	// SNMPv1 devices often have trouble with batch requests (packet sanity errors)
	// so we use individual queries for maximum compatibility
	batchSize := 10
	if dp.client.Version == gosnmp.Version1 {
		batchSize = 1 // Individual queries for SNMP v1 - more reliable
	}

	// Poll in batches
	values := make(map[string]interface{})
	errors := make([]string, 0)

	for i := 0; i < len(oids); i += batchSize {
		end := i + batchSize
		if end > len(oids) {
			end = len(oids)
		}

		batchOIDs := oids[i:end]
		result, err := dp.client.Get(batchOIDs)
		if err != nil {
			log.Printf("[DEBUG] SNMP GET error for device %s batch %d-%d: %v", dp.device.ID, i, end, err)

			// For SNMP v1, try individual OIDs if batch fails (noSuchName causes whole batch to fail)
			if dp.client.Version == gosnmp.Version1 {
				log.Printf("[DEBUG] Falling back to individual OID queries for batch %d-%d", i, end)
				// Close and reopen connection to ensure clean state
				if dp.client.Conn != nil {
					dp.client.Conn.Close()
					dp.client.Conn = nil
				}
				if connErr := dp.client.Connect(); connErr != nil {
					errors = append(errors, connErr.Error())
					continue
				}
				for _, singleOID := range batchOIDs {
					singleResult, singleErr := dp.client.Get([]string{singleOID})
					if singleErr != nil {
						// Skip this OID silently - it may not exist on this device
						log.Printf("[DEBUG] OID %s not available: %v", singleOID, singleErr)
						continue
					}
					for _, variable := range singleResult.Variables {
						value := s.parseValue(variable)
						if value != nil {
							normalizedOID := normalizeOID(variable.Name)
							// Apply transformations for all mappings that use this OID
							if mappings, exists := oidToMappings[normalizedOID]; exists {
								for _, mapping := range mappings {
									transformedValue := s.transformValue(value, mapping)
									values[mapping.Name] = transformedValue
								}
							}
							values[variable.Name] = value
						}
					}
				}
			} else {
				errors = append(errors, err.Error())
				if dp.client.Conn != nil {
					dp.client.Conn.Close()
					dp.client.Conn = nil
				}
			}
			continue
		}

		for _, variable := range result.Variables {
			normalizedOID := normalizeOID(variable.Name)

			// Check for missing OIDs and track them to skip in future polls
			if variable.Type == gosnmp.NoSuchInstance || variable.Type == gosnmp.NoSuchObject {
				if !dp.missingOIDs[normalizedOID] {
					log.Printf("[INFO] OID %s not available on device %s - will skip in future polls", normalizedOID, dp.device.Name)
					dp.missingOIDs[normalizedOID] = true
				}
				continue
			}

			value := s.parseValue(variable)
			if value == nil {
				continue
			}

			// Apply profile transformations for all mappings that use this OID
			if mappings, exists := oidToMappings[normalizedOID]; exists {
				for _, mapping := range mappings {
					transformedValue := s.transformValue(value, mapping)
					values[mapping.Name] = transformedValue
				}
			}
			values[variable.Name] = value
		}
	}

	// Calculate derived values (e.g., Active Power = Voltage × Current)
	s.calculateDerivedValues(values)

	online := len(errors) == 0
	s.updateState(dp.device.ID, values, online, errors)

	// Update last seen
	if online {
		_ = s.deviceRepo.UpdateLastSeen(context.Background(), dp.device.ID)
	}
}

// calculateDerivedValues computes values that can be derived from other measurements
func (s *PollerService) calculateDerivedValues(values map[string]interface{}) {
	// Calculate Active Power if it's 0 or missing (P = V × I)
	activePower := toFloat64(values["Active Power"])
	if activePower == 0 {
		voltage := toFloat64(values["Voltage"])
		current := toFloat64(values["Total Current"])
		if voltage > 0 && current > 0 {
			// Round to 1 decimal place
			values["Active Power"] = math.Round(voltage*current*10) / 10
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

func (s *PollerService) getOIDsToPoll(dp *devicePoller) []string {
	if dp.profile == nil {
		return nil
	}

	// Use a map to deduplicate OIDs (composite_switch mappings share the same OID)
	oidSet := make(map[string]bool)
	pollGroups := map[string]int{
		"frequent": 1,
		"static":   10,
	}

	for _, mapping := range dp.profile.OIDMappings {
		group := mapping.PollGroup
		if group == "" {
			group = "frequent"
		}

		interval, exists := pollGroups[group]
		if !exists {
			interval = 1
		}

		if dp.pollCount%interval == 0 {
			// Skip OIDs that previously returned NoSuchInstance/NoSuchObject
			normalizedOID := normalizeOID(mapping.OID)
			if dp.missingOIDs[normalizedOID] {
				continue
			}
			oidSet[mapping.OID] = true
		}
	}

	// Convert set to slice
	oids := make([]string, 0, len(oidSet))
	for oid := range oidSet {
		oids = append(oids, oid)
	}

	return oids
}

func (s *PollerService) parseValue(variable gosnmp.SnmpPDU) interface{} {
	switch variable.Type {
	case gosnmp.OctetString:
		return string(variable.Value.([]byte))
	case gosnmp.Integer, gosnmp.Counter32, gosnmp.Counter64, gosnmp.Gauge32, gosnmp.TimeTicks, gosnmp.Uinteger32:
		return variable.Value
	case gosnmp.ObjectIdentifier:
		return variable.Value.(string)
	case gosnmp.NoSuchObject, gosnmp.NoSuchInstance:
		// OID doesn't exist on this device - normal for optional features
		return nil
	case gosnmp.Null:
		return nil
	default:
		return variable.Value
	}
}

func (s *PollerService) transformValue(value interface{}, mapping *domain.OIDMapping) interface{} {
	// Handle composite_switch type - extract value at specified index from comma-separated string
	if mapping.Type == domain.OIDTypeCompositeSwitch {
		return s.extractCompositeValue(value, mapping)
	}

	// Apply scale
	if mapping.Scale != 0 {
		var numericValue float64
		var hasNumeric bool

		switch v := value.(type) {
		case int:
			numericValue = float64(v)
			hasNumeric = true
		case int64:
			numericValue = float64(v)
			hasNumeric = true
		case uint:
			numericValue = float64(v)
			hasNumeric = true
		case uint64:
			numericValue = float64(v)
			hasNumeric = true
		case float64:
			numericValue = v
			hasNumeric = true
		case string:
			// Try to parse string as number (some devices return numbers as strings)
			if parsed, err := strconv.ParseFloat(v, 64); err == nil {
				numericValue = parsed
				hasNumeric = true
			}
		}

		if hasNumeric {
			scaled := numericValue * mapping.Scale
			// Round to appropriate decimal places based on scale
			if mapping.Scale < 0.01 {
				return math.Round(scaled*1000) / 1000
			}
			return math.Round(scaled*100) / 100
		}
	}

	// Apply enum mapping
	if mapping.Type == domain.OIDTypeEnum && mapping.EnumValues != nil {
		switch v := value.(type) {
		case int:
			if name, ok := mapping.EnumValues[v]; ok {
				return name
			}
		case int64:
			if name, ok := mapping.EnumValues[int(v)]; ok {
				return name
			}
		}
	}

	return value
}

// extractCompositeValue extracts a value from a comma-separated string at the specified index
// Used for Energenie PDU style outlet status (e.g., "1,1,0,-1,-1,-1,-1,-1")
func (s *PollerService) extractCompositeValue(value interface{}, mapping *domain.OIDMapping) interface{} {
	strValue, ok := value.(string)
	if !ok {
		return value
	}

	separator := mapping.CompositeSeparator
	if separator == "" {
		separator = ","
	}

	parts := strings.Split(strValue, separator)
	if mapping.CompositeIndex >= len(parts) {
		log.Printf("[DEBUG] Composite index %d out of range for value %q (len=%d)", mapping.CompositeIndex, strValue, len(parts))
		return nil
	}

	partValue := strings.TrimSpace(parts[mapping.CompositeIndex])

	// Convert to integer if possible for enum mapping
	var intVal int
	if _, err := fmt.Sscanf(partValue, "%d", &intVal); err == nil {
		// Apply enum mapping if available
		if mapping.EnumValues != nil {
			if name, ok := mapping.EnumValues[intVal]; ok {
				return name
			}
		}
		return intVal
	}

	return partValue
}

func (s *PollerService) updateState(deviceID string, values map[string]interface{}, online bool, errors []string) {
	s.statesMu.Lock()
	state, exists := s.states[deviceID]
	if !exists {
		state = &domain.DeviceState{
			DeviceID: deviceID,
			Values:   make(map[string]interface{}),
		}
		s.states[deviceID] = state
	}

	state.Online = online
	state.LastPoll = time.Now()
	state.Errors = errors

	if values != nil {
		for k, v := range values {
			state.Values[k] = v
		}
	}

	// Copy the full accumulated state values for the event
	fullValues := make(map[string]interface{}, len(state.Values))
	for k, v := range state.Values {
		fullValues[k] = v
	}
	s.statesMu.Unlock()

	// Notify subscribers with full accumulated state
	event := StateUpdateEvent{
		DeviceID:  deviceID,
		Timestamp: time.Now(),
		Values:    fullValues,
		Online:    online,
	}

	s.subMu.RLock()
	for _, ch := range s.subscribers {
		select {
		case ch <- event:
		default:
			// Channel full, skip
		}
	}
	s.subMu.RUnlock()
}

// normalizeOID strips leading dot from OID for consistent comparison
func normalizeOID(oid string) string {
	if len(oid) > 0 && oid[0] == '.' {
		return oid[1:]
	}
	return oid
}
