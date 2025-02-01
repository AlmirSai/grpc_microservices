package main

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"service3/db"
	pb "service3/service3/proto"
)

type server struct {
	pb.UnimplementedMonitoringServiceServer
	userPool    *db.DBPool
	orderPool   *db.DBPool
	kafkaReader *kafka.Reader
	metrics     struct {
		userService  *Metrics
		orderService *Metrics
		mutex        sync.RWMutex
	}
}

type Metrics struct {
	totalRequests      uint64
	successfulRequests uint64
	failedRequests     uint64
	totalLatency       uint64
}

func (s *server) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	start := time.Now()
	logrus.WithFields(logrus.Fields{
		"name":  req.Name,
		"email": req.Email,
	}).Info("Creating user")

	var userID int64
	tx, err := s.userPool.BeginTx()
	if err != nil {
		s.metrics.mutex.Lock()
		defer s.metrics.mutex.Unlock()
		s.metrics.userService.totalRequests++
		s.metrics.userService.failedRequests++
		logrus.WithError(err).Error("Failed to begin transaction")
		return &pb.CreateUserResponse{
			Status: "error",
			Error:  err.Error(),
		}, nil
	}
	defer s.userPool.ReleaseTx(tx, err)

	err = tx.QueryRow(
		"INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id",
		req.Name,
		req.Email,
	).Scan(&userID)

	s.metrics.mutex.Lock()
	defer s.metrics.mutex.Unlock()
	s.metrics.userService.totalRequests++
	s.metrics.userService.totalLatency += uint64(time.Since(start).Milliseconds())

	if err != nil {
		s.metrics.userService.failedRequests++
		logrus.WithError(err).Error("Failed to create user")
		return &pb.CreateUserResponse{
			Status: "error",
			Error:  err.Error(),
		}, nil
	}

	s.metrics.userService.successfulRequests++
	logrus.WithField("user_id", userID).Info("User created successfully")
	return &pb.CreateUserResponse{
		UserId: userID,
		Status: "success",
	}, nil
}

func (s *server) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	start := time.Now()
	logrus.WithField("user_id", req.UserId).Info("Getting user")

	tx, err := s.userPool.BeginTx()
	if err != nil {
		s.metrics.mutex.Lock()
		defer s.metrics.mutex.Unlock()
		s.metrics.userService.totalRequests++
		s.metrics.userService.failedRequests++
		logrus.WithError(err).Error("Failed to begin transaction")
		return &pb.GetUserResponse{
			Error: err.Error(),
		}, nil
	}
	defer s.userPool.ReleaseTx(tx, err)

	var name, email string
	err = tx.QueryRow(
		"SELECT name, email FROM users WHERE id = $1",
		req.UserId,
	).Scan(&name, &email)

	s.metrics.mutex.Lock()
	defer s.metrics.mutex.Unlock()
	s.metrics.userService.totalRequests++
	s.metrics.userService.totalLatency += uint64(time.Since(start).Milliseconds())

	if err != nil {
		s.metrics.userService.failedRequests++
		logrus.WithError(err).Error("Failed to get user")
		return &pb.GetUserResponse{
			Error: err.Error(),
		}, nil
	}

	s.metrics.userService.successfulRequests++
	logrus.WithFields(logrus.Fields{
		"user_id": req.UserId,
		"name":    name,
		"email":   email,
	}).Info("User retrieved successfully")

	return &pb.GetUserResponse{
		UserId: req.UserId,
		Name:   name,
		Email:  email,
	}, nil
}

func (s *server) GetServiceMetrics(ctx context.Context, req *pb.GetMetricsRequest) (*pb.ServiceMetricsResponse, error) {
	s.metrics.mutex.RLock()
	defer s.metrics.mutex.RUnlock()

	var metrics *Metrics
	switch req.ServiceName {
	case "user":
		metrics = s.metrics.userService
	case "order":
		metrics = s.metrics.orderService
	default:
		return nil, fmt.Errorf("unknown service: %s", req.ServiceName)
	}

	return &pb.ServiceMetricsResponse{
		TotalRequests:      metrics.totalRequests,
		SuccessfulRequests: metrics.successfulRequests,
		FailedRequests:     metrics.failedRequests,
		AverageLatencyMs:   float64(metrics.totalLatency) / float64(metrics.successfulRequests),
	}, nil
}

func (s *server) GetDatabaseMetrics(ctx context.Context, req *pb.GetMetricsRequest) (*pb.DatabaseMetricsResponse, error) {
	var pool *db.DBPool
	switch req.ServiceName {
	case "user":
		pool = s.userPool
	case "order":
		pool = s.orderPool
	default:
		return nil, fmt.Errorf("unknown service: %s", req.ServiceName)
	}

	tx, err := pool.BeginTx()
	if err != nil {
		return nil, err
	}
	defer pool.ReleaseTx(tx, err)

	var activeConnections int32
	row := tx.QueryRow("SELECT count(*) FROM pg_stat_activity WHERE state = 'active'")
	if err := row.Scan(&activeConnections); err != nil {
		return nil, err
	}

	var dbSize int64
	row = tx.QueryRow("SELECT pg_database_size(current_database())")
	if err := row.Scan(&dbSize); err != nil {
		return nil, err
	}

	return &pb.DatabaseMetricsResponse{
		ActiveConnections: activeConnections,
		DatabaseSizeMb:    float64(dbSize) / (1024 * 1024),
	}, nil
}

func (s *server) GetKafkaMetrics(ctx context.Context, req *pb.GetMetricsRequest) (*pb.KafkaMetricsResponse, error) {
	stats := s.kafkaReader.Stats()
	return &pb.KafkaMetricsResponse{
		MessagesReceived: stats.Messages,
		BytesReceived:    stats.Bytes,
		Lag:              stats.Lag,
	}, nil
}

func main() {
	if err := godotenv.Load(); err != nil {
		logrus.Warn("No .env file found or error reading it; proceeding with environment variables.")
	}

	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

	// Connect to both databases for monitoring
	userDBConnStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("USER_POSTGRES_HOST"),
		os.Getenv("USER_POSTGRES_PORT"),
		os.Getenv("USER_POSTGRES_USER"),
		os.Getenv("USER_POSTGRES_PASSWORD"),
		os.Getenv("USER_POSTGRES_DB"))

	orderDBConnStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("ORDER_POSTGRES_HOST"),
		os.Getenv("ORDER_POSTGRES_PORT"),
		os.Getenv("ORDER_POSTGRES_USER"),
		os.Getenv("ORDER_POSTGRES_PASSWORD"),
		os.Getenv("ORDER_POSTGRES_DB"))

	userDB, err := sql.Open("postgres", userDBConnStr)
	if err != nil {
		logrus.Fatalf("Failed to connect to user database: %v", err)
	}
	defer userDB.Close()

	orderDB, err := sql.Open("postgres", orderDBConnStr)
	if err != nil {
		logrus.Fatalf("Failed to connect to order database: %v", err)
	}
	defer orderDB.Close()

	// Initialize Kafka reader
	kafkaHost := os.Getenv("KAFKA_HOST")
	kafkaPort := os.Getenv("KAFKA_PORT")
	kafkaAddress := fmt.Sprintf("%s:%s", kafkaHost, kafkaPort)
	kafkaReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{kafkaAddress},
		Topic:   "user-events",
		GroupID: "monitoring-service",
	})
	defer kafkaReader.Close()

	// Initialize server with metrics
	userPool, err := db.NewDBPool(userDBConnStr, 10)
	if err != nil {
		logrus.Fatalf("Failed to create user database pool: %v", err)
	}
	orderPool, err := db.NewDBPool(orderDBConnStr, 10)
	if err != nil {
		logrus.Fatalf("Failed to create order database pool: %v", err)
	}

	srv := &server{
		userPool:    userPool,
		orderPool:   orderPool,
		kafkaReader: kafkaReader,
		metrics: struct {
			userService  *Metrics
			orderService *Metrics
			mutex        sync.RWMutex
		}{
			userService:  &Metrics{},
			orderService: &Metrics{},
		},
	}

	// Start metrics collection
	go reportMetrics("User Service", srv.metrics.userService)
	go reportMetrics("Order Service", srv.metrics.orderService)
	go monitorDatabase("User Service", srv.userPool)
	go monitorDatabase("Order Service", srv.orderPool)
	go monitorKafka(kafkaAddress)

	// Start gRPC server
	lis, err := net.Listen("tcp", ":50053")
	if err != nil {
		logrus.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterMonitoringServiceServer(grpcServer, srv)
	reflection.Register(grpcServer)

	logrus.Info("Monitoring service started on :50053")
	if err := grpcServer.Serve(lis); err != nil {
		logrus.Fatalf("Failed to serve: %v", err)
	}
}

func reportMetrics(serviceName string, metrics *Metrics) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		logrus.WithFields(logrus.Fields{
			"service":            serviceName,
			"total_requests":     metrics.totalRequests,
			"successful":         metrics.successfulRequests,
			"failed":             metrics.failedRequests,
			"average_latency_ms": float64(metrics.totalLatency) / float64(metrics.successfulRequests),
		}).Info("Service Metrics")
	}
}

func monitorDatabase(serviceName string, dbPool *db.DBPool) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Get number of active connections
		var activeConnections int
		tx, err := dbPool.BeginTx()
		if err != nil {
			logrus.Errorf("%s: Failed to begin transaction: %v", serviceName, err)
			continue
		}

		row := tx.QueryRow("SELECT count(*) FROM pg_stat_activity WHERE state = 'active'")
		if err := row.Scan(&activeConnections); err != nil {
			logrus.Errorf("%s: Failed to get active connections: %v", serviceName, err)
			dbPool.ReleaseTx(tx, err)
			continue
		}

		// Get database statistics
		var dbSize int64
		row = tx.QueryRow("SELECT pg_database_size(current_database())")
		if err := row.Scan(&dbSize); err != nil {
			logrus.Errorf("%s: Failed to get database size: %v", serviceName, err)
			dbPool.ReleaseTx(tx, err)
			continue
		}

		if err := dbPool.ReleaseTx(tx, nil); err != nil {
			logrus.Errorf("%s: Failed to release transaction: %v", serviceName, err)
			continue
		}

		logrus.WithFields(logrus.Fields{
			"service":            serviceName,
			"active_connections": activeConnections,
			"database_size_mb":   float64(dbSize) / (1024 * 1024),
		}).Info("Database Metrics")
	}
}

func monitorKafka(kafkaAddress string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{kafkaAddress},
		Topic:   "user-events",
		GroupID: "monitoring-service",
	})
	defer reader.Close()

	for range ticker.C {
		// Get topic statistics
		stats := reader.Stats()
		logrus.WithFields(logrus.Fields{
			"topic":             "user-events",
			"messages_received": stats.Messages,
			"bytes_received":    stats.Bytes,
			"lag":               stats.Lag,
		}).Info("Kafka Metrics")
	}
}
