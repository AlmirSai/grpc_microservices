#!/bin/bash

# Function to check if a service is healthy
check_service() {
    local service=$1
    local port=$2
    local max_attempts=$3
    local attempt=1

    echo "Checking $service health..."
    while [ $attempt -le $max_attempts ]; do
        if docker-compose ps $service | grep -q "Up"; then
            if nc -z localhost $port; then
                echo "$service is ready and healthy"
                return 0
            fi
        fi
        echo "Waiting for $service (attempt $attempt/$max_attempts)..."
        sleep 5
        attempt=$((attempt + 1))
    done
    echo "$service failed to start or is unhealthy"
    docker-compose logs $service
    return 1
}

# Function to cleanup on error
cleanup() {
    local service=$1
    echo "Error occurred while starting $service. Cleaning up..."
    docker-compose logs $service
    docker-compose down
    exit 1
}

# Function to wait for service health
wait_for_service() {
    local service=$1
    local max_attempts=30
    local attempt=1

    echo "Waiting for $service to be healthy..."
    while [ $attempt -le $max_attempts ]; do
        if docker-compose ps $service | grep -q "healthy"; then
            echo "$service is healthy"
            return 0
        fi
        echo "Waiting for $service health check (attempt $attempt/$max_attempts)..."
        sleep 5
        attempt=$((attempt + 1))
    done
    echo "$service failed health check"
    return 1
}

# Trap errors with service name
trap 'cleanup "${CURRENT_SERVICE:-Unknown service}"' ERR

# Create logs directory if it doesn't exist and ensure proper permissions
echo "Creating logs directory..."
mkdir -p "$(pwd)/logs"
chmod 755 "$(pwd)/logs"

# Check if required tools are installed
for tool in grpcurl nc docker-compose; do
    if ! command -v $tool &> /dev/null; then
        echo "$tool is not installed. Please install it first."
        exit 1
    fi
done

# Start infrastructure services in sequence
echo "Starting Zookeeper..."
CURRENT_SERVICE="zookeeper"
docker-compose up -d zookeeper || cleanup "zookeeper"
wait_for_service "zookeeper" || cleanup "zookeeper"

echo "Starting Postgres instances..."
CURRENT_SERVICE="postgres"
docker-compose up -d postgres postgres_orders || cleanup "postgres"
wait_for_service "postgres" || cleanup "postgres"
wait_for_service "postgres_orders" || cleanup "postgres_orders"

# Wait for Postgres instances to be ready
check_service "Postgres (Users)" 5434 12 || cleanup
check_service "Postgres (Orders)" 5435 12 || cleanup

# Start Kafka after Zookeeper and verify its health
echo "Starting Kafka..."
CURRENT_SERVICE="kafka"
docker-compose up -d kafka || cleanup "kafka"
wait_for_service "kafka" || cleanup "kafka"

# Start the services in order
echo "Starting microservices..."
for service in service1 service2 service3; do
    CURRENT_SERVICE="$service"
    docker-compose up -d "$service" || cleanup "$service"
    wait_for_service "$service" || cleanup "$service"
done

# Check if services are healthy
check_service "User Service" 50051 6 || cleanup
check_service "Order Service" 50052 6 || cleanup
check_service "Monitoring Service" 50053 6 || cleanup

# Start the frontend service
echo "Starting frontend service..."
CURRENT_SERVICE="frontend"
docker-compose up -d frontend || cleanup "frontend"
wait_for_service "frontend" || cleanup "frontend"

# Check if frontend is healthy
check_service "Frontend Service" 80 6 || cleanup

# Collect initial metrics
echo "Collecting initial metrics..."
grpcurl -plaintext localhost:50053 monitoring.MonitoringService/GetKafkaMetrics > "$(pwd)/logs/kafka_metrics.log" 2>&1
grpcurl -plaintext localhost:50053 monitoring.MonitoringService/GetServiceMetrics > "$(pwd)/logs/service_metrics.log" 2>&1

# Collect service logs
echo "Collecting service logs..."
docker-compose logs --tail=100 service1 > "$(pwd)/logs/service1.log" 2>&1
docker-compose logs --tail=100 service2 > "$(pwd)/logs/service2.log" 2>&1
docker-compose logs --tail=100 service3 > "$(pwd)/logs/service3.log" 2>&1
docker-compose logs --tail=100 frontend > "$(pwd)/logs/frontend.log" 2>&1

echo "All services are running and logs have been collected in the 'logs' directory."

# Run load tests
echo "Running load tests..."
cd service3 && go test -v -run TestHighLoad > "$(pwd)/../logs/load_test.log" 2>&1

# Return to original directory
cd ..

# Collect post-test metrics
echo "Collecting post-test metrics..."
grpcurl -plaintext localhost:50053 monitoring.MonitoringService/GetServiceMetrics > "$(pwd)/logs/post_test_metrics.log" 2>&1

echo "Deployment completed successfully. Log files available in:"
ls -l "$(pwd)/logs/"
