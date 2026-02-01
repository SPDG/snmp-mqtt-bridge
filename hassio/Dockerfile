# Home Assistant Addon Dockerfile
ARG BUILD_FROM
FROM ${BUILD_FROM}

# Install bashio for HA integration
RUN apk add --no-cache \
    bash \
    jq

# Copy pre-built binary (for HA addon build)
COPY snmp-bridge /app/snmp-bridge
COPY profiles/ /app/profiles/
COPY run.sh /

RUN chmod +x /run.sh /app/snmp-bridge

CMD [ "/run.sh" ]
