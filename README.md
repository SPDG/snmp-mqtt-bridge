# SNMP-MQTT Bridge

[![Build](https://github.com/twopoint71/snmp-mqtt-bridge/actions/workflows/build.yml/badge.svg)](https://github.com/twopoint71/snmp-mqtt-bridge/actions/workflows/build.yml)
[![Release](https://img.shields.io/github/v/release/twopoint71/snmp-mqtt-bridge)](https://github.com/twopoint71/snmp-mqtt-bridge/releases)
[![License](https://img.shields.io/github/license/twopoint71/snmp-mqtt-bridge)](LICENSE)

A lightweight Go application that bridges SNMP-enabled power devices (UPS, ATS, PDU) to Home Assistant via MQTT with automatic device discovery.

## Features

- **SNMP Polling**: Configurable polling intervals with smart polling (frequent vs static OIDs)
- **SNMP Trap Receiver**: Real-time event notifications from devices
- **MQTT Integration**: Full Home Assistant auto-discovery support
- **Device Control**: Control PDU outlets and ATS sources from Home Assistant
- **Web Interface**: Modern Vue.js dashboard with dark mode support
- **Device Profiles**: Pre-configured profiles for popular devices

## Supported Devices

### Built-in Profiles

| Manufacturer | Model | Type |
|--------------|-------|------|
| APC | AP4421 | Automatic Transfer Switch |
| APC | AP7921 | Switched Rack PDU |

Additional profiles can be added via YAML configuration files.

## Quick Start

### Prerequisites

- Go 1.22 or later
- Node.js 18+ (for frontend development)
- MQTT broker (e.g., Mosquitto)

### Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/twopoint71/snmp-mqtt-bridge.git
   cd snmp-mqtt-bridge
   ```

2. Build the application:
   ```bash
   # Build frontend
   cd frontend
   npm install
   npm run build
   cd ..

   # Copy frontend to embed directory
   cp -r frontend/dist internal/api/static/

   # Build Go binary
   go build -o snmp-bridge ./cmd/snmp-bridge
   ```

3. Configure the application:
   ```bash
   cp config.example.yaml config.yaml
   # Edit config.yaml with your settings
   ```

4. Run:
   ```bash
   ./snmp-bridge
   ```

5. Open the web interface at `http://localhost:8080`

## Configuration

Create a `config.yaml` file:

```yaml
http:
  address: "0.0.0.0"
  port: 8080

mqtt:
  broker: "tcp://localhost:1883"
  username: ""
  password: ""
  client_id: "snmp-bridge"
  topic_prefix: "snmp-bridge"
  discovery_prefix: "homeassistant"

snmp:
  default_community: "public"
  default_version: "v2c"
  poll_interval: 30
  trap_port: 10162

database:
  path: "data/snmp-bridge.db"
```

### Environment Variables

Configuration can also be set via environment variables:

| Variable | Description |
|----------|-------------|
| `HTTP_PORT` | HTTP server port |
| `MQTT_BROKER` | MQTT broker URL |
| `MQTT_USERNAME` | MQTT username |
| `MQTT_PASSWORD` | MQTT password |
| `DB_PATH` | SQLite database path |

## Home Assistant Integration

The bridge automatically publishes MQTT discovery messages for Home Assistant. Devices will appear automatically in Home Assistant once configured in the bridge.

### Entity Types

- **Sensors**: Voltage, current, power, load percentage
- **Binary Sensors**: Online status, redundancy state, fault indicators
- **Switches**: PDU outlet control
- **Selects**: ATS source selection, transfer settings

## API Reference

### REST Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/health` | Health check |
| GET | `/api/devices` | List devices |
| POST | `/api/devices` | Add device |
| GET | `/api/devices/:id` | Get device |
| PUT | `/api/devices/:id` | Update device |
| DELETE | `/api/devices/:id` | Delete device |
| POST | `/api/devices/:id/test` | Test connection |
| GET | `/api/profiles` | List profiles |
| GET | `/api/traps` | Get trap logs |
| GET | `/api/ws` | WebSocket for real-time updates |

## Development

### Project Structure

```
├── cmd/snmp-bridge/     # Application entry point
├── internal/
│   ├── api/             # HTTP handlers
│   ├── config/          # Configuration
│   ├── domain/          # Business entities
│   ├── mqtt/            # MQTT client
│   ├── repository/      # Data access
│   ├── service/         # Business logic
│   └── worker/          # Background workers
├── frontend/            # Vue.js SPA
└── profiles/            # Device profiles
```

### Running in Development

```bash
# Terminal 1: Run backend
go run ./cmd/snmp-bridge

# Terminal 2: Run frontend dev server
cd frontend
npm run dev
```

### Building for Release

```bash
# Build frontend
cd frontend && npm run build && cd ..

# Build for current platform
go build -o snmp-bridge ./cmd/snmp-bridge

# Build for Linux (from any platform)
GOOS=linux GOARCH=amd64 go build -o snmp-bridge-linux-amd64 ./cmd/snmp-bridge
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes using [Conventional Commits](https://www.conventionalcommits.org/)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [gosnmp](https://github.com/gosnmp/gosnmp) - SNMP library for Go
- [paho.mqtt.golang](https://github.com/eclipse/paho.mqtt.golang) - MQTT client
- [Gin](https://github.com/gin-gonic/gin) - HTTP web framework
- [Vue.js](https://vuejs.org/) - Frontend framework
- [Tailwind CSS](https://tailwindcss.com/) - CSS framework
