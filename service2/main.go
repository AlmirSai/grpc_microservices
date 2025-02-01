package main

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"

	pb "service2/service2/proto" // Import the generated proto package.

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type server struct {
	pb.UnimplementedOrderServiceServer
	db          *sql.DB
	kafkaReader *kafka.Reader
}

func main() {
	// Load environment variables from .env file.
	if err := godotenv.Load(); err != nil {
		logrus.Warn("No .env file found or error reading it; proceeding with environment variables.")
	}

	// Set up logging.
	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

	// Build the Postgres connection string using environment variables.
	postgresHost := os.Getenv("ORDER_POSTGRES_HOST")
	postgresPort := os.Getenv("ORDER_POSTGRES_PORT")
	postgresUser := os.Getenv("ORDER_POSTGRES_USER")
	postgresPassword := os.Getenv("ORDER_POSTGRES_PASSWORD")
	postgresDB := os.Getenv("ORDER_POSTGRES_DB")
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		postgresHost, postgresPort, postgresUser, postgresPassword, postgresDB)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		logrus.Fatalf("Failed to connect to Postgres: %v", err)
	}

	// Ensure the "orders" table exists.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS orders (
			id SERIAL PRIMARY KEY,
			user_id INT,
			product TEXT
		)
	`)
	if err != nil {
		logrus.Fatalf("Failed to create table: %v", err)
	}

	// Build Kafka address using environment variables.
	kafkaHost := os.Getenv("KAFKA_HOST")
	kafkaPort := os.Getenv("KAFKA_PORT")
	kafkaAddress := fmt.Sprintf("%s:%s", kafkaHost, kafkaPort)

	// Set up Kafka reader to consume messages from the "user-events" topic.
	kafkaReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{kafkaAddress},
		GroupID: "order-service-group",
		Topic:   "user-events",
	})
	go consumeKafkaMessages(kafkaReader)

	// Start the gRPC server.
	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		logrus.Fatalf("Failed to listen: %v", err)
	}

	srv := &server{
		db:          db,
		kafkaReader: kafkaReader,
	}

	grpcServer := grpc.NewServer()
	pb.RegisterOrderServiceServer(grpcServer, srv)
	reflection.Register(grpcServer)
	logrus.Info("OrderService gRPC server listening on :50052")
	if err := grpcServer.Serve(lis); err != nil {
		logrus.Fatalf("Failed to serve: %v", err)
	}
}

// CreateOrder writes a new order into Postgres.
func (s *server) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.CreateOrderResponse, error) {
	logrus.Infof("Received CreateOrder request: user_id=%d, product=%s", req.UserId, req.Product)

	var id int
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO orders (user_id, product)
		VALUES ($1, $2) RETURNING id
	`, req.UserId, req.Product).Scan(&id)
	if err != nil {
		logrus.Errorf("Failed to insert order: %v", err)
		return nil, err
	}
	return &pb.CreateOrderResponse{Id: int32(id)}, nil
}

// consumeKafkaMessages continuously reads messages from Kafka.
func consumeKafkaMessages(reader *kafka.Reader) {
	for {
		m, err := reader.ReadMessage(context.Background())
		if err != nil {
			logrus.Errorf("Failed to read Kafka message: %v", err)
			continue
		}
		logrus.Infof("Consumed Kafka message: key=%s, value=%s", string(m.Key), string(m.Value))
	}
}
