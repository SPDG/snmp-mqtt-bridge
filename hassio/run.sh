#!/usr/bin/with-contenv bashio
# shellcheck shell=bash
set -e

bashio::log.info "Starting SNMP-MQTT Bridge addon..."

# Read addon configuration
MQTT_HOST=$(bashio::config 'mqtt_host')
MQTT_PORT=$(bashio::config 'mqtt_port')
MQTT_USERNAME=$(bashio::config 'mqtt_username')
MQTT_PASSWORD=$(bashio::config 'mqtt_password')
MQTT_DISCOVERY_PREFIX=$(bashio::config 'mqtt_discovery_prefix')
MQTT_TOPIC_PREFIX=$(bashio::config 'mqtt_topic_prefix')
POLL_INTERVAL=$(bashio::config 'poll_interval')
TRAP_PORT=$(bashio::config 'trap_port')
LOG_LEVEL=$(bashio::config 'log_level')

# If MQTT host is empty, try to get from HA Supervisor MQTT service
if bashio::var.is_empty "${MQTT_HOST}" && bashio::services.available "mqtt"; then
    bashio::log.info "MQTT service available, using Supervisor MQTT credentials"
    MQTT_HOST=$(bashio::services mqtt "host")
    MQTT_PORT=$(bashio::services mqtt "port")
    MQTT_USERNAME=$(bashio::services mqtt "username")
    MQTT_PASSWORD=$(bashio::services mqtt "password")
fi

# Fallback to core-mosquitto if still empty
if bashio::var.is_empty "${MQTT_HOST}"; then
    bashio::log.warning "No MQTT configuration found, using default core-mosquitto"
    MQTT_HOST="core-mosquitto"
    MQTT_PORT="1883"
fi

# Get ingress entry for URL rewriting
INGRESS_ENTRY=$(bashio::addon.ingress_entry)
bashio::log.info "Ingress entry: ${INGRESS_ENTRY}"

# Export environment variables for the application
export SNMP_BRIDGE_SERVER_HOST="0.0.0.0"
export SNMP_BRIDGE_SERVER_PORT="8099"
export SNMP_BRIDGE_DATABASE_DRIVER="sqlite"
export SNMP_BRIDGE_DATABASE_DSN="/data/snmp-bridge.db"
export SNMP_BRIDGE_MQTT_BROKER="${MQTT_HOST}"
export SNMP_BRIDGE_MQTT_PORT="${MQTT_PORT}"
export SNMP_BRIDGE_MQTT_USERNAME="${MQTT_USERNAME}"
export SNMP_BRIDGE_MQTT_PASSWORD="${MQTT_PASSWORD}"
export SNMP_BRIDGE_MQTT_DISCOVERY_PREFIX="${MQTT_DISCOVERY_PREFIX}"
export SNMP_BRIDGE_MQTT_TOPIC_PREFIX="${MQTT_TOPIC_PREFIX}"
export SNMP_BRIDGE_SNMP_POLL_INTERVAL="${POLL_INTERVAL}s"
export SNMP_BRIDGE_SNMP_TRAP_PORT="${TRAP_PORT}"
export SNMP_BRIDGE_LOGGING_LEVEL="${LOG_LEVEL}"
export SNMP_BRIDGE_INGRESS_PATH="${INGRESS_ENTRY}"

bashio::log.info "Configuration:"
bashio::log.info "  MQTT Broker: ${MQTT_HOST}:${MQTT_PORT}"
bashio::log.info "  Discovery Prefix: ${MQTT_DISCOVERY_PREFIX}"
bashio::log.info "  Topic Prefix: ${MQTT_TOPIC_PREFIX}"
bashio::log.info "  Poll Interval: ${POLL_INTERVAL}s"
bashio::log.info "  Trap Port: ${TRAP_PORT}"
bashio::log.info "  Log Level: ${LOG_LEVEL}"

cd /app
exec /app/snmp-bridge
