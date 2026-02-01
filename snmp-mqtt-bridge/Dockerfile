# Home Assistant Addon Dockerfile
ARG BUILD_FROM=ghcr.io/hassio-addons/base:15.0.8
ARG BUILD_ARCH=amd64
FROM ${BUILD_FROM}

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata

# Copy pre-built binary (built by CI or manually)
COPY snmp-bridge /app/snmp-bridge
COPY profiles/ /app/profiles/
COPY run.sh /run.sh

RUN chmod +x /run.sh /app/snmp-bridge && \
    mkdir -p /data

WORKDIR /app

CMD [ "/run.sh" ]
