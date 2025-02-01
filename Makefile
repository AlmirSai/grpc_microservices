.PHONY: all build test clean start stop logs proto docker-build docker-push help

# Display help information about available commands
help:
	@echo "Available commands:"
	@echo ""
	@echo "  make all           - Default target that runs build"
	@echo "  make build         - Build all services locally"
	@echo "  make docker-build  - Build Docker images for all services"
	@echo "  make docker-push   - Push Docker images to registry"
	@echo "  make test          - Run tests for all services"
	@echo "  make clean         - Clean build artifacts and stop containers"
	@echo "  make start         - Start all services using docker-compose"
	@echo "  make stop          - Stop all services"
	@echo "  make logs          - View logs of all services"
	@echo "  make proto         - Generate protobuf files"
	@echo "  make load-test     - Run load tests"
	@echo "  make status        - Show service status"
	@echo "  make restart       - Rebuild and restart a specific service (usage: make restart service=service1)"
	@echo "  make help          - Show this help message"

all: build

# Build all services
build:
	cd service1 && go build -o bin/service1
	cd service2 && go build -o bin/service2
	cd service3 && go build -o bin/service3

# Build Docker images for all services
docker-build:
	docker-compose build --no-cache

# Push Docker images to registry (if needed)
docker-push:
	docker-compose push

# Run tests for all services
test:
	cd service1 && go test -v ./...
	cd service2 && go test -v ./...
	cd service3 && go test -v ./...

# Clean build artifacts
clean:
	rm -rf service1/bin service2/bin service3/bin
	docker-compose down -v

# Start all services using docker-compose
start:
	./start.sh

# Stop all services
stop:
	docker-compose down

# View logs of all services
logs:
	docker-compose logs -f

# Generate protobuf files
proto:
	cd service1/proto && protoc --go_out=. --go-grpc_out=. user.proto
	cd service2/proto && protoc --go_out=. --go-grpc_out=. order.proto
	cd service3/proto && protoc --go_out=. --go-grpc_out=. monitoring.proto

# Run load tests
load-test:
	cd service3 && go test -v -run TestHighLoad

# Show service status
status:
	docker-compose ps

# Rebuild and restart a specific service (usage: make restart service=service1)
restart:
	docker-compose up -d --build $(service)