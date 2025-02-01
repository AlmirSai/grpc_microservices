# Microservices System with gRPC, Kafka, and PostgreSQL

A high-performance microservices architecture implementing user management, order processing, and real-time monitoring using gRPC, Kafka, and PostgreSQL.

## System Architecture

### Components

1. **Service 1 (User Service)**
   - Handles user management operations
   - Exposes gRPC endpoints on port 50051
   - Uses PostgreSQL for user data storage
   - Produces events to Kafka for user-related activities

2. **Service 2 (Order Service)**
   - Manages order processing
   - Exposes gRPC endpoints on port 50052
   - Uses PostgreSQL for order data storage
   - Consumes user events from Kafka

3. **Service 3 (Monitoring Service)**
   - Provides real-time system monitoring
   - Exposes gRPC endpoints on port 50053
   - Monitors both PostgreSQL databases and Kafka metrics
   - Includes a frontend visualization component

### Infrastructure

- **Kafka**
  - Message broker for event-driven communication
  - Uses Zookeeper for coordination
  - Configured with 3 partitions and replication factor 1
  - Topic: `user-events`

- **PostgreSQL Databases**
  - Users Database (Service 1)
    - Max Connections: 200
    - Shared Buffers: 512MB
    - Effective Cache Size: 1536MB
    - Work Memory: 16MB

  - Orders Database (Service 2)
    - Max Connections: 200
    - Shared Buffers: 1000MB
    - Effective Cache Size: 2000MB
    - Work Memory: 24MB

## Getting Started

### Prerequisites

- Docker and Docker Compose
- Go 1.x
- gRPCurl
- netcat (nc)

### Environment Configuration

Create a `.env` file with the following configurations:

```env
# User Service Database
USER_POSTGRES_HOST=postgres
USER_POSTGRES_PORT=5432
USER_POSTGRES_USER=postgres
USER_POSTGRES_PASSWORD=postgres
USER_POSTGRES_DB=users_db

# Order Service Database
ORDER_POSTGRES_HOST=postgres_orders
ORDER_POSTGRES_PORT=5432
ORDER_POSTGRES_USER=postgres
ORDER_POSTGRES_PASSWORD=postgres
ORDER_POSTGRES_DB=orders_db

# Kafka Configuration
KAFKA_HOST=kafka
KAFKA_PORT=9092
```

### Deployment

1. Start the system:
   ```bash
   ./start.sh
   ```

2. The script will start services in the following order:
   - Zookeeper
   - PostgreSQL instances
   - Kafka
   - Service 1 (User Service)
   - Service 2 (Order Service)
   - Service 3 (Monitoring Service)
   - Frontend Service

### Resource Allocation

#### Services
- Service 1: 1 CPU, 1GB RAM
- Service 2: 1 CPU, 1GB RAM
- Service 3: 3 CPU, 3GB RAM
- Frontend: 0.5 CPU, 512MB RAM

#### Infrastructure
- Zookeeper: 0.5 CPU, 512MB RAM
- Kafka: 2 CPU, 2GB RAM
- Users PostgreSQL: 2 CPU, 2GB RAM
- Orders PostgreSQL: 4 CPU, 4GB RAM

## Monitoring and Metrics

### Available Metrics

1. **Service Metrics**
   - Total requests
   - Successful requests
   - Failed requests
   - Average latency

2. **Database Metrics**
   - Active connections
   - Database size
   - Query performance

3. **Kafka Metrics**
   - Messages received
   - Bytes received
   - Consumer lag

### Accessing Metrics

- Frontend Dashboard: http://localhost:80
- gRPC Monitoring Service: localhost:50053

## Database Schema

### Users Database
```sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name TEXT,
    email TEXT
);
```

### Orders Database
```sql
CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    user_id INT,
    product TEXT
);
```

## Event Flow

1. User Creation:
   - Service 1 creates user in PostgreSQL
   - Publishes event to Kafka topic "user-events"
   - Service 2 consumes the event
   - Service 3 monitors the event flow

## Load Testing

The system includes load testing capabilities in Service 3:
- Concurrent user simulation
- Database connection pool management
- Kafka consumer group testing

## Health Checks

- All services implement health checks
- PostgreSQL instances check using `pg_isready`
- Kafka health verified through topic listing
- Services checked using port availability

## Network Configuration

- All services communicate through a dedicated Docker network (microservices-network)
- Bridge network driver for container communication
- Exposed ports for external access:
  - User Service: 50051
  - Order Service: 50052
  - Monitoring Service: 50053
  - Frontend: 80
  - Kafka: 9093
  - Zookeeper: 2182
  - Users PostgreSQL: 5434
  - Orders PostgreSQL: 5435