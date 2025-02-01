package main

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	pb "service1/service1/proto"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type server struct {
	pb.UnimplementedUserServiceServer
	db          *sql.DB
	kafkaWriter *kafka.Writer
}

// main starts the gRPC server and listens on port 50051 for incoming requests.
//
// First, it loads environment variables from a .env file, if present.
// Then, it sets up logging with a full timestamp.
//
// Next, it connects to the Postgres database specified by the environment
// variables USER_POSTGRES_HOST, USER_POSTGRES_PORT, USER_POSTGRES_USER,
// USER_POSTGRES_PASSWORD, and USER_POSTGRES_DB. It creates the "users" table
// if it doesn't already exist.
//
// Then, it sets up a Kafka writer to produce messages to the topic "user-events"
// on the broker specified by the environment variables KAFKA_HOST and KAFKA_PORT.
//
// Finally, it starts the gRPC server and registers the UserServiceServer with it.
// It serves on port 50051.
func main() {
	if err := godotenv.Load(); err != nil {
		logrus.Warn("No .env file found or error reading it; proceeding with environment variables.")
	}

	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

	postgresHost := os.Getenv("USER_POSTGRES_HOST")
	postgresPort := os.Getenv("USER_POSTGRES_PORT")
	postgresUser := os.Getenv("USER_POSTGRES_USER")
	postgresPassword := os.Getenv("USER_POSTGRES_PASSWORD")
	postgresDB := os.Getenv("USER_POSTGRES_DB")

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		postgresHost, postgresPort, postgresUser, postgresPassword, postgresDB)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		logrus.Fatalf("Unable to connect to database: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			name TEXT,
			email TEXT
		)
	`)
	if err != nil {
		logrus.Fatalf("Failed to create table: %v", err)
	}

	kafkaHost := os.Getenv("KAFKA_HOST")
	kafkaPort := os.Getenv("KAFKA_PORT")

	kafkaAddress := fmt.Sprintf("%s:%s", kafkaHost, kafkaPort)

	kafkaWriter := kafka.NewWriter(kafka.WriterConfig{
		Brokers: []string{kafkaAddress},
		Topic:   "user-events",
	})

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		logrus.Fatalf("Failed to listen: %v", err)
	}

	srv := &server{
		db:          db,
		kafkaWriter: kafkaWriter,
	}

	grpcServer := grpc.NewServer()
	pb.RegisterUserServiceServer(grpcServer, srv)
	reflection.Register(grpcServer)
	logrus.Info("UserService gRPC server listening on :50051")
	if err := grpcServer.Serve(lis); err != nil {
		logrus.Fatalf("Failed to serve: %v", err)
	}
}

func (s *server) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	logrus.Infof("Received CreateUser request: name=%s, email=%s", req.Name, req.Email)

	// Start a database transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		logrus.Errorf("Failed to begin transaction: %v", err)
		return nil, fmt.Errorf("internal error: failed to begin transaction")
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				logrus.Errorf("Failed to rollback transaction: %v", rbErr)
			}
		}
	}()

	var id int
	err = tx.QueryRowContext(ctx, `
		INSERT INTO users (name, email)
		VALUES ($1, $2) RETURNING id
	`, req.Name, req.Email).Scan(&id)
	if err != nil {
		logrus.Errorf("Failed to insert user: %v", err)
		return nil, fmt.Errorf("failed to create user: %v", err)
	}

	// Prepare Kafka message
	msg := kafka.Message{
		Key:   []byte(fmt.Sprintf("%d", id)),
		Value: []byte(fmt.Sprintf("User %s created", req.Name)),
	}

	// Retry Kafka message writing with exponential backoff
	for retries := 0; retries < 3; retries++ {
		err = s.kafkaWriter.WriteMessages(ctx, msg)
		if err == nil {
			break
		}
		logrus.Warnf("Failed to write Kafka message (attempt %d/3): %v", retries+1, err)
		time.Sleep(time.Duration(retries+1) * 100 * time.Millisecond)
	}

	if err != nil {
		logrus.Errorf("Failed to write Kafka message after all retries: %v", err)
		return nil, fmt.Errorf("failed to publish event: %v", err)
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		logrus.Errorf("Failed to commit transaction: %v", err)
		return nil, fmt.Errorf("internal error: failed to commit transaction")
	}

	logrus.Infof("Successfully created user with ID: %d", id)
	return &pb.CreateUserResponse{Id: int32(id)}, nil
}
