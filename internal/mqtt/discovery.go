package mqtt

import (
	"encoding/json"
	"fmt"
	"strings"

	"snmp-mqtt-bridge/internal/domain"
)

// DiscoveryConfig represents Home Assistant MQTT discovery payload
type DiscoveryConfig struct {
	Name              string            `json:"name"`
	UniqueID          string            `json:"unique_id"`
	ObjectID          string            `json:"object_id,omitempty"`
	StateTopic        string            `json:"state_topic,omitempty"`
	CommandTopic      string            `json:"command_topic,omitempty"`
	AvailabilityTopic string            `json:"availability_topic,omitempty"`
	PayloadAvailable  string            `json:"payload_available,omitempty"`
	PayloadNotAvailable string          `json:"payload_not_available,omitempty"`
	Device            *DiscoveryDevice  `json:"device,omitempty"`
	DeviceClass       string            `json:"device_class,omitempty"`
	StateClass        string            `json:"state_class,omitempty"`
	UnitOfMeasurement string            `json:"unit_of_measurement,omitempty"`
	Icon              string            `json:"icon,omitempty"`
	EntityCategory    string            `json:"entity_category,omitempty"`
	ValueTemplate     string            `json:"value_template,omitempty"`
	PayloadOn         string            `json:"payload_on,omitempty"`
	PayloadOff        string            `json:"payload_off,omitempty"`
	Options           []string          `json:"options,omitempty"`
	Min               float64           `json:"min,omitempty"`
	Max               float64           `json:"max,omitempty"`
	Step              float64           `json:"step,omitempty"`
	Extra             map[string]interface{} `json:"-"` // For any extra fields
}

// DiscoveryDevice represents device information in discovery payload
type DiscoveryDevice struct {
	Identifiers  []string `json:"identifiers"`
	Name         string   `json:"name"`
	Manufacturer string   `json:"manufacturer,omitempty"`
	Model        string   `json:"model,omitempty"`
	SwVersion    string   `json:"sw_version,omitempty"`
	ViaDevice    string   `json:"via_device,omitempty"`
}

// Discovery manages Home Assistant MQTT auto-discovery
type Discovery struct {
	client          *Client
	discoveryPrefix string
	topicPrefix     string
}

// NewDiscovery creates a new discovery manager
func NewDiscovery(client *Client, discoveryPrefix, topicPrefix string) *Discovery {
	return &Discovery{
		client:          client,
		discoveryPrefix: discoveryPrefix,
		topicPrefix:     topicPrefix,
	}
}

// PublishDevice publishes discovery configs for all entities of a device
func (d *Discovery) PublishDevice(device *domain.Device, profile *domain.Profile) error {
	if profile == nil {
		return nil
	}

	haDevice := &DiscoveryDevice{
		Identifiers:  []string{fmt.Sprintf("snmp_bridge_%s", device.ID)},
		Name:         device.Name,
		Manufacturer: profile.Manufacturer,
		Model:        profile.Model,
		ViaDevice:    "snmp_mqtt_bridge",
	}

	availabilityTopic := fmt.Sprintf("%s/bridge/status", d.topicPrefix)

	for _, mapping := range profile.OIDMappings {
		entityID := sanitizeEntityID(mapping.Name)
		uniqueID := fmt.Sprintf("snmp_bridge_%s_%s", device.ID, entityID)

		config := &DiscoveryConfig{
			Name:              mapping.Name,
			UniqueID:          uniqueID,
			ObjectID:          entityID,
			Device:            haDevice,
			AvailabilityTopic: availabilityTopic,
			PayloadAvailable:  "online",
			PayloadNotAvailable: "offline",
		}

		// Apply custom label if available
		if device.Labels != nil {
			if label, ok := device.Labels[mapping.Name]; ok {
				config.Name = label
			}
		}

		// Set common properties
		if mapping.DeviceClass != "" {
			config.DeviceClass = mapping.DeviceClass
		}
		if mapping.StateClass != "" {
			config.StateClass = mapping.StateClass
		}
		if mapping.Unit != "" {
			config.UnitOfMeasurement = mapping.Unit
		}
		if mapping.Icon != "" {
			config.Icon = mapping.Icon
		}
		if mapping.Category != "" {
			config.EntityCategory = mapping.Category
		}

		// Build topics based on component type
		stateTopic := fmt.Sprintf("%s/%s/%s/state", d.topicPrefix, device.ID, entityID)
		config.StateTopic = stateTopic

		if mapping.Writable {
			config.CommandTopic = fmt.Sprintf("%s/%s/%s/set", d.topicPrefix, device.ID, entityID)
		}

		// Component-specific configuration
		switch mapping.HAComponent {
		case domain.HAComponentBinarySensor:
			// Publisher sends ON/OFF values, so just set the payloads
			config.PayloadOn = "ON"
			config.PayloadOff = "OFF"

		case domain.HAComponentSwitch:
			config.PayloadOn = "ON"
			config.PayloadOff = "OFF"

		case domain.HAComponentSelect:
			if mapping.EnumValues != nil {
				// Sort by key to maintain consistent order
				options := make([]string, 0, len(mapping.EnumValues))
				for i := 1; i <= len(mapping.EnumValues); i++ {
					if v, ok := mapping.EnumValues[i]; ok {
						options = append(options, v)
					}
				}
				// Add any remaining values not in sequence
				for _, v := range mapping.EnumValues {
					found := false
					for _, opt := range options {
						if opt == v {
							found = true
							break
						}
					}
					if !found {
						options = append(options, v)
					}
				}
				config.Options = options
			}

		case domain.HAComponentNumber:
			config.Min = 0
			config.Max = 100
			config.Step = 1
		}

		// Publish discovery config
		topic := fmt.Sprintf("%s/%s/%s/%s/config",
			d.discoveryPrefix,
			componentToString(mapping.HAComponent),
			device.ID,
			entityID,
		)

		if err := d.client.Publish(topic, config, true); err != nil {
			return fmt.Errorf("failed to publish discovery for %s: %w", mapping.Name, err)
		}
	}

	return nil
}

// UpdateSelectOptions updates the options for a select entity
func (d *Discovery) UpdateSelectOptions(device *domain.Device, profile *domain.Profile, mapping domain.OIDMapping, options []string) error {
	entityID := sanitizeEntityID(mapping.Name)
	uniqueID := fmt.Sprintf("snmp_bridge_%s_%s", device.ID, entityID)

	haDevice := &DiscoveryDevice{
		Identifiers:  []string{fmt.Sprintf("snmp_bridge_%s", device.ID)},
		Name:         device.Name,
		Manufacturer: profile.Manufacturer,
		Model:        profile.Model,
		ViaDevice:    "snmp_mqtt_bridge",
	}

	config := &DiscoveryConfig{
		Name:     mapping.Name,
		UniqueID: uniqueID,
		ObjectID: entityID,
		Device:   haDevice,
		AvailabilityTopic:   fmt.Sprintf("%s/bridge/status", d.topicPrefix),
		PayloadAvailable:    "online",
		PayloadNotAvailable: "offline",
		StateTopic:          fmt.Sprintf("%s/%s/%s/state", d.topicPrefix, device.ID, entityID),
		Options:             options,
	}

	if mapping.Writable {
		config.CommandTopic = fmt.Sprintf("%s/%s/%s/set", d.topicPrefix, device.ID, entityID)
	}

	if mapping.Icon != "" {
		config.Icon = mapping.Icon
	}

	topic := fmt.Sprintf("%s/%s/%s/%s/config",
		d.discoveryPrefix,
		componentToString(mapping.HAComponent),
		device.ID,
		entityID,
	)

	return d.client.Publish(topic, config, true)
}

// RemoveDevice removes all discovery configs for a device
func (d *Discovery) RemoveDevice(deviceID string, profile *domain.Profile) error {
	if profile == nil {
		return nil
	}

	for _, mapping := range profile.OIDMappings {
		entityID := sanitizeEntityID(mapping.Name)

		topic := fmt.Sprintf("%s/%s/%s/%s/config",
			d.discoveryPrefix,
			componentToString(mapping.HAComponent),
			deviceID,
			entityID,
		)

		// Publish empty payload to remove discovery
		if err := d.client.Publish(topic, "", true); err != nil {
			return fmt.Errorf("failed to remove discovery for %s: %w", mapping.Name, err)
		}
	}

	return nil
}

// MarshalJSON customizes JSON marshaling to include extra fields
func (c *DiscoveryConfig) MarshalJSON() ([]byte, error) {
	type Alias DiscoveryConfig
	m := make(map[string]interface{})

	// Marshal the struct
	data, err := json.Marshal((*Alias)(c))
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	// Add extra fields
	for k, v := range c.Extra {
		m[k] = v
	}

	return json.Marshal(m)
}

func componentToString(c domain.HAComponent) string {
	return string(c)
}

func sanitizeEntityID(name string) string {
	// Convert to lowercase and replace spaces/special chars with underscores
	result := strings.ToLower(name)
	result = strings.ReplaceAll(result, " ", "_")
	result = strings.ReplaceAll(result, "-", "_")
	result = strings.ReplaceAll(result, ".", "_")

	// Remove any non-alphanumeric characters except underscore
	var sb strings.Builder
	for _, r := range result {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			sb.WriteRune(r)
		}
	}

	return sb.String()
}
