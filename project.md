# SNMP-MQTT Bridge

A Go application that bridges SNMP devices (UPS, ATS, PDU) to Home Assistant via MQTT with auto-discovery support.

## Project Status

### Completed Features

#### Phase 1: Foundation
- [x] Go project initialization (`go mod init`)
- [x] Project directory structure
- [x] Configuration management (Viper + YAML + env vars)
- [x] Database layer (GORM + SQLite with migrations)
- [x] HTTP server (Gin) with health endpoint
- [x] Embedded frontend SPA

#### Phase 2: Device Management
- [x] CRUD API for devices (`/api/devices`)
- [x] SNMP connection testing
- [x] Device profiles (YAML loader)
- [x] Custom labels for outlets/ports
- [x] Profile: APC ATS (AP4421)
- [x] Profile: APC Switched Rack PDU (AP7921)
- [x] SNMP v1/v2c/v3 support (v3 with noAuthNoPriv)

#### Phase 3: SNMP Polling Engine
- [x] Worker poller with configurable interval
- [x] Smart polling (frequent vs static OIDs)
- [x] Event channel for state updates
- [x] In-memory state cache with accumulation
- [x] Batch polling with fallback for SNMP v1
- [x] OID normalization for consistent matching

#### Phase 4: MQTT Integration
- [x] MQTT client with reconnect logic
- [x] Home Assistant Auto-Discovery
  - [x] sensor entities
  - [x] binary_sensor entities
  - [x] switch entities (with control)
  - [x] select entities
- [x] State publishing (`snmp-bridge/<deviceID>/state`)
- [x] Command subscription (`snmp-bridge/<deviceID>/+/set`)
- [x] SNMP SET command execution from HA
- [x] Dynamic source name updates for ATS

#### Phase 5: SNMP Trap Receiver
- [x] UDP listener (configurable port)
- [x] Trap parsing and device matching
- [x] Trap logging to database
- [x] Immediate state update on trap

#### Phase 6-7: Web UI (Vue.js 3 + Vite)
- [x] Dashboard with device status overview
- [x] Device management (add/edit/delete)
- [x] Device detail view with live data
- [x] ATS source control panel
- [x] PDU outlet control panel
- [x] Outlet on/off/reboot controls
- [x] Source/outlet name editing
- [x] Trap log viewer
- [x] Settings page
- [x] Real-time updates via WebSocket
- [x] Dark mode with Dracula theme
- [x] Responsive design

### Pending Features

#### Phase 8: Docker & HA Addon
- [ ] Multi-stage Dockerfile
- [ ] S6 Overlay services
- [ ] HA Addon config (Ingress support)
- [ ] bashio integration for MQTT credentials
- [ ] GitHub Container Registry publishing

#### Future Enhancements
- [ ] SNMP v3 with authPriv mode
- [ ] MIB browser for custom OID discovery
- [ ] Additional device profiles (Eaton, CyberPower, etc.)
- [ ] Email/webhook notifications
- [ ] Historical data charts
- [ ] Backup/restore configuration
- [ ] Unit and integration tests

## Technology Stack

| Component | Technology |
|-----------|------------|
| Backend | Go 1.22+ |
| Web Framework | Gin |
| SNMP | gosnmp |
| MQTT | paho.mqtt.golang |
| Database | GORM + SQLite |
| Frontend | Vue.js 3 + Vite + Tailwind CSS |
| Container | Docker + S6 Overlay (planned) |

## Project Structure

```
snmp-mqtt-bridge/
├── cmd/snmp-bridge/main.go      # Entry point
├── internal/
│   ├── api/                     # HTTP API handlers
│   │   ├── handler/             # Request handlers
│   │   ├── static/              # Embedded frontend
│   │   └── server.go            # Gin server setup
│   ├── config/                  # Configuration management
│   ├── domain/                  # Domain entities
│   ├── embed/                   # Frontend embedding
│   ├── mqtt/                    # MQTT client & discovery
│   ├── repository/sqlite/       # Database repositories
│   ├── service/                 # Business logic
│   └── worker/                  # Background workers
├── frontend/                    # Vue.js SPA source
├── profiles/                    # YAML device profiles
├── config.yaml                  # Default configuration
├── go.mod
└── go.sum
```

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/health` | Health check |
| GET | `/api/devices` | List all devices |
| POST | `/api/devices` | Create device |
| GET | `/api/devices/:id` | Get device |
| PUT | `/api/devices/:id` | Update device |
| DELETE | `/api/devices/:id` | Delete device |
| POST | `/api/devices/:id/test` | Test SNMP connection |
| POST | `/api/test-connection` | Test new connection |
| GET | `/api/profiles` | List profiles |
| GET | `/api/profiles/:id` | Get profile |
| GET | `/api/traps` | List trap logs |
| GET | `/api/settings` | Get settings |
| PUT | `/api/settings` | Update settings |
| POST | `/api/command/switch-source` | Switch ATS source |
| POST | `/api/command/set-source-name` | Set source name |
| POST | `/api/command/set-outlet-state` | Set PDU outlet state |
| POST | `/api/command/set-outlet-name` | Set PDU outlet name |
| POST | `/api/command/reboot-outlet` | Reboot PDU outlet |
| GET | `/api/ws` | WebSocket for real-time updates |
