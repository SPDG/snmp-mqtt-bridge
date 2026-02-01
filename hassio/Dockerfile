# Home Assistant Addon Dockerfile
ARG BUILD_FROM=ghcr.io/hassio-addons/base:15.0.8
FROM ${BUILD_FROM}

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata

# Copy pre-built binary
COPY snmp-bridge /app/snmp-bridge
COPY profiles/ /app/profiles/
COPY run.sh /

RUN chmod +x /run.sh /app/snmp-bridge && \
    mkdir -p /data

WORKDIR /app

CMD [ "/run.sh" ]
