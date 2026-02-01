package worker

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"snmp-mqtt-bridge/internal/domain"
	"snmp-mqtt-bridge/internal/repository"
	"snmp-mqtt-bridge/internal/service"

	"github.com/google/uuid"
	"github.com/gosnmp/gosnmp"
)

// TrapReceiver listens for SNMP traps
type TrapReceiver struct {
	port       int
	deviceRepo repository.DeviceRepository
	trapRepo   repository.TrapLogRepository
	poller     *service.PollerService

	listener *gosnmp.TrapListener
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup

	// Event handlers
	onTrap func(*domain.TrapLog)
}

// NewTrapReceiver creates a new trap receiver
func NewTrapReceiver(
	port int,
	deviceRepo repository.DeviceRepository,
	trapRepo repository.TrapLogRepository,
	poller *service.PollerService,
) *TrapReceiver {
	ctx, cancel := context.WithCancel(context.Background())

	return &TrapReceiver{
		port:       port,
		deviceRepo: deviceRepo,
		trapRepo:   trapRepo,
		poller:     poller,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// OnTrap sets the trap event handler
func (r *TrapReceiver) OnTrap(handler func(*domain.TrapLog)) {
	r.onTrap = handler
}

// Start starts the trap receiver
func (r *TrapReceiver) Start() error {
	r.listener = gosnmp.NewTrapListener()
	r.listener.OnNewTrap = r.handleTrap
	r.listener.Params = gosnmp.Default

	addr := fmt.Sprintf("0.0.0.0:%d", r.port)

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		log.Printf("Starting SNMP trap listener on %s", addr)

		if err := r.listener.Listen(addr); err != nil {
			log.Printf("Trap listener error: %v", err)
		}
	}()

	return nil
}

// Stop stops the trap receiver
func (r *TrapReceiver) Stop() {
	r.cancel()

	if r.listener != nil {
		r.listener.Close()
	}

	r.wg.Wait()
	log.Println("Trap receiver stopped")
}

func (r *TrapReceiver) handleTrap(packet *gosnmp.SnmpPacket, addr *net.UDPAddr) {
	log.Printf("Received trap from %s", addr.IP.String())

	// Find device by IP
	var deviceID *string
	devices, err := r.deviceRepo.GetAll(context.Background())
	if err == nil {
		for _, d := range devices {
			if d.IPAddress == addr.IP.String() {
				id := d.ID
				deviceID = &id
				break
			}
		}
	}

	// Parse trap OID
	trapOID := ""
	variables := make(domain.TrapVariables)

	for _, variable := range packet.Variables {
		name := variable.Name
		value := r.parseVariable(variable)

		// Check for trap OID (SNMPv2-MIB::snmpTrapOID.0 = .1.3.6.1.6.3.1.1.4.1.0)
		if name == ".1.3.6.1.6.3.1.1.4.1.0" {
			if oid, ok := value.(string); ok {
				trapOID = oid
			}
		} else {
			variables[name] = value
		}
	}

	// Determine severity based on trap OID
	severity := r.determineSeverity(trapOID, variables)

	// Create trap log
	trapLog := &domain.TrapLog{
		ID:         uuid.New().String(),
		DeviceID:   deviceID,
		SourceIP:   addr.IP.String(),
		TrapOID:    trapOID,
		Variables:  variables,
		Severity:   severity,
		Message:    r.formatMessage(trapOID, variables),
		ReceivedAt: time.Now(),
	}

	// Save to database
	if err := r.trapRepo.Create(context.Background(), trapLog); err != nil {
		log.Printf("Failed to save trap: %v", err)
	}

	// Trigger immediate poll for the device if known
	if deviceID != nil && r.poller != nil {
		// The poller will update state on next poll cycle
		// For immediate update, we could add a TriggerPoll method to PollerService
		log.Printf("Trap received for device %s, will be reflected in next poll", *deviceID)
	}

	// Notify handlers
	if r.onTrap != nil {
		r.onTrap(trapLog)
	}
}

func (r *TrapReceiver) parseVariable(variable gosnmp.SnmpPDU) interface{} {
	switch variable.Type {
	case gosnmp.OctetString:
		return string(variable.Value.([]byte))
	case gosnmp.Integer, gosnmp.Counter32, gosnmp.Counter64, gosnmp.Gauge32, gosnmp.TimeTicks, gosnmp.Uinteger32:
		return variable.Value
	case gosnmp.ObjectIdentifier:
		return variable.Value.(string)
	case gosnmp.IPAddress:
		if ip, ok := variable.Value.(string); ok {
			return ip
		}
		return fmt.Sprintf("%v", variable.Value)
	default:
		return fmt.Sprintf("%v", variable.Value)
	}
}

func (r *TrapReceiver) determineSeverity(trapOID string, variables domain.TrapVariables) domain.TrapSeverity {
	// Common APC trap OID patterns for severity detection
	// This is a simplified version - real implementation would have a mapping table

	// Check for known critical trap OIDs
	criticalPatterns := []string{
		".1.3.6.1.4.1.318.2.3.1", // APC UPS on battery
		".1.3.6.1.4.1.318.2.3.5", // APC low battery
	}

	for _, pattern := range criticalPatterns {
		if trapOID == pattern {
			return domain.SeverityCritical
		}
	}

	// Check for warning patterns
	warningPatterns := []string{
		".1.3.6.1.4.1.318.2.3.2", // APC return from battery
		".1.3.6.1.4.1.318.2.3.4", // APC communication lost
	}

	for _, pattern := range warningPatterns {
		if trapOID == pattern {
			return domain.SeverityWarning
		}
	}

	// Default to info
	return domain.SeverityInfo
}

func (r *TrapReceiver) formatMessage(trapOID string, variables domain.TrapVariables) string {
	// Basic message formatting
	// Real implementation would use templates from profile

	switch trapOID {
	case ".1.3.6.1.4.1.318.2.3.1":
		return "UPS on battery power"
	case ".1.3.6.1.4.1.318.2.3.2":
		return "UPS returned to utility power"
	case ".1.3.6.1.4.1.318.2.3.5":
		return "UPS battery low"
	case ".1.3.6.1.4.1.318.2.3.4":
		return "Communication lost with UPS"
	default:
		if len(variables) > 0 {
			return fmt.Sprintf("Trap %s with %d variables", trapOID, len(variables))
		}
		return fmt.Sprintf("Trap %s", trapOID)
	}
}
