.PHONY: all build run test clean frontend docker embed

# Variables
BINARY_NAME=snmp-bridge
VERSION=1.0.0

all: frontend embed build

# Build frontend
frontend:
	cd frontend && npm install && npm run build

# Copy frontend dist to embed directory
embed: frontend
	rm -rf internal/embed/frontend
	mkdir -p internal/embed/frontend
	cp -r frontend/dist/* internal/embed/frontend/

# Build Go binary
build:
	CGO_ENABLED=1 go build -ldflags="-w -s -X main.Version=$(VERSION)" -o $(BINARY_NAME) ./cmd/snmp-bridge

# Build without frontend (faster for development)
build-backend:
	CGO_ENABLED=1 go build -o $(BINARY_NAME) ./cmd/snmp-bridge

# Run the application
run: build-backend
	./$(BINARY_NAME)

# Run with hot reload (requires air: go install github.com/cosmtrek/air@latest)
dev:
	air

# Run frontend dev server
dev-frontend:
	cd frontend && npm run dev

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -rf frontend/dist
	rm -rf frontend/node_modules

# Build Docker image
docker:
	docker build -f docker/Dockerfile -t snmp-mqtt-bridge:$(VERSION) .

# Run Docker container
docker-run:
	docker run -d \
		-p 8080:8080 \
		-p 162:162/udp \
		-v snmp-bridge-data:/data \
		--name snmp-bridge \
		snmp-mqtt-bridge:$(VERSION)

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# Download dependencies
deps:
	go mod download
	cd frontend && npm install

# Generate mocks (if using mockgen)
generate:
	go generate ./...

# Tidy dependencies
tidy:
	go mod tidy
