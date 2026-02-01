#!/usr/bin/env bash
set -e

CONFIG_PATH=/data/options.json

# Read configuration using bashio
MQTT_HOST=$(jq --raw-output '.mqtt_host // empty' $CONFIG_PATH)
MQTT_PORT=$(jq --raw-output '.mqtt_port // 1883' $CONFIG_PATH)
MQTT_USERNAME=$(jq --raw-output '.mqtt_username // empty' $CONFIG_PATH)
MQTT_PASSWORD=$(jq --raw-output '.mqtt_password // empty' $CONFIG_PATH)
MQTT_DISCOVERY_PREFIX=$(jq --raw-output '.mqtt_discovery_prefix // "homeassistant"' $CONFIG_PATH)
POLL_INTERVAL=$(jq --raw-output '.poll_interval // 30' $CONFIG_PATH)
LOG_LEVEL=$(jq --raw-output '.log_level // "info"' $CONFIG_PATH)

# If MQTT host is empty, try to get from HA services
if [ -z "$MQTT_HOST" ]; then
    # Try bashio for MQTT service discovery
    if command -v bashio &> /dev/null; then
        MQTT_HOST=$(bashio::services "mqtt" "host" 2>/dev/null || echo "")
        MQTT_PORT=$(bashio::services "mqtt" "port" 2>/dev/null || echo "1883")
        MQTT_USERNAME=$(bashio::services "mqtt" "username" 2>/dev/null || echo "")
        MQTT_PASSWORD=$(bashio::services "mqtt" "password" 2>/dev/null || echo "")
    fi
fi

# Default to core-mosquitto if still empty
if [ -z "$MQTT_HOST" ]; then
    MQTT_HOST="core-mosquitto"
fi

# Export environment variables
export SNMP_BRIDGE_SERVER_HOST="0.0.0.0"
export SNMP_BRIDGE_SERVER_PORT="8080"
export SNMP_BRIDGE_DATABASE_DRIVER="sqlite"
export SNMP_BRIDGE_DATABASE_DSN="/data/snmp-bridge.db"
export SNMP_BRIDGE_MQTT_BROKER="$MQTT_HOST"
export SNMP_BRIDGE_MQTT_PORT="$MQTT_PORT"
export SNMP_BRIDGE_MQTT_USERNAME="$MQTT_USERNAME"
export SNMP_BRIDGE_MQTT_PASSWORD="$MQTT_PASSWORD"
export SNMP_BRIDGE_MQTT_DISCOVERY_PREFIX="$MQTT_DISCOVERY_PREFIX"
export SNMP_BRIDGE_SNMP_POLL_INTERVAL="${POLL_INTERVAL}s"
export SNMP_BRIDGE_LOGGING_LEVEL="$LOG_LEVEL"

echo "Starting SNMP-MQTT Bridge..."
echo "MQTT Broker: ${MQTT_HOST}:${MQTT_PORT}"

cd /app
exec /app/snmp-bridge
