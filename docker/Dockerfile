# Build stage for frontend
FROM node:20-alpine AS frontend-builder

WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Build stage for Go backend
FROM golang:1.24-alpine AS backend-builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Copy frontend dist to embed directory
RUN rm -rf internal/embed/frontend && mkdir -p internal/embed/frontend
COPY --from=frontend-builder /app/frontend/dist ./internal/embed/frontend/

RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o /snmp-bridge ./cmd/snmp-bridge

# Final stage
FROM alpine:3.19

RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    s6-overlay

# Create non-root user
RUN addgroup -g 1000 bridge && \
    adduser -u 1000 -G bridge -h /app -D bridge

WORKDIR /app

# Copy binary and assets
COPY --from=backend-builder /snmp-bridge /app/snmp-bridge
COPY profiles/ /app/profiles/

# Copy S6 overlay services
COPY docker/rootfs/ /

# Create data directory
RUN mkdir -p /data && chown bridge:bridge /data

# Environment variables
ENV SNMP_BRIDGE_SERVER_HOST=0.0.0.0 \
    SNMP_BRIDGE_SERVER_PORT=8080 \
    SNMP_BRIDGE_DATABASE_DRIVER=sqlite \
    SNMP_BRIDGE_DATABASE_DSN=/data/snmp-bridge.db

EXPOSE 8080
EXPOSE 162/udp

VOLUME ["/data"]

ENTRYPOINT ["/init"]
