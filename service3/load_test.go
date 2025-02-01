package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	pb "service3/service3/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	writeRPS              = 10000 // Write requests per second
	readRPS               = 1000  // Read requests per second
	testDuration          = 60 * time.Second
	numConcurrentRequests = 1000 // Number of concurrent goroutines
)

func TestHighLoad(t *testing.T) {
	// Ensure logs directory exists
	logsDir := filepath.Join("..", "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("Failed to create logs directory: %v", err)
	}

	// Create log files with absolute paths
	file, err := os.Create(filepath.Join(logsDir, "load_test.log"))
	if err != nil {
		t.Fatalf("Failed to create load test log file: %v", err)
	}
	defer file.Close()

	userLog, err := os.Create(filepath.Join(logsDir, "user_operations.log"))
	if err != nil {
		t.Fatalf("Failed to create user operations log file: %v", err)
	}
	defer userLog.Close()

	onlineLog, err := os.Create(filepath.Join(logsDir, "online_operations.log"))
	if err != nil {
		t.Fatalf("Failed to create online operations log file: %v", err)
	}
	defer onlineLog.Close()

	// Connect to the monitoring service
	conn, err := grpc.Dial(":50053", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewMonitoringServiceClient(conn)

	// Track metrics
	var (
		totalRequests      uint64
		successfulRequests uint64
		failedRequests     uint64
		totalLatency       time.Duration
		userCreations      uint64
		userRetrieves      uint64
		userFailures       uint64
		onlineRequests     uint64
		onlineFailures     uint64
		mutex              sync.Mutex
	)

	// Start load test
	start := time.Now()
	var wg sync.WaitGroup

	// Run concurrent requests
	for i := 0; i < numConcurrentRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Since(start) < testDuration {
				// Test all endpoints including user operations
				endpoints := []struct {
					name string
					fn   func() error
				}{
					{"ServiceMetrics", func() error {
						_, err := client.GetServiceMetrics(context.Background(), &pb.GetMetricsRequest{ServiceName: "user"})
						return err
					}},
					{"DatabaseMetrics", func() error {
						_, err := client.GetDatabaseMetrics(context.Background(), &pb.GetMetricsRequest{ServiceName: "user"})
						return err
					}},
					{"KafkaMetrics", func() error {
						_, err := client.GetKafkaMetrics(context.Background(), &pb.GetMetricsRequest{ServiceName: "test"})
						return err
					}},
					{"CreateUser", func() error {
						resp, err := client.CreateUser(context.Background(), &pb.CreateUserRequest{
							Name:  fmt.Sprintf("TestUser%d", time.Now().UnixNano()),
							Email: fmt.Sprintf("test%d@example.com", time.Now().UnixNano()),
						})
						if err != nil {
							return err
						}
						if resp.Status != "success" {
							return fmt.Errorf("create user failed: %s", resp.Error)
						}
						mutex.Lock()
						userCreations++
						mutex.Unlock()
						return nil
					}},
					{"GetUser", func() error {
						// Try to get a user with a random ID between 1 and current number of created users
						mutex.Lock()
						userID := int64(1 + (time.Now().UnixNano() % int64(userCreations+1)))
						mutex.Unlock()

						resp, err := client.GetUser(context.Background(), &pb.GetUserRequest{UserId: userID})
						if err != nil {
							return err
						}
						if resp.Error != "" {
							return fmt.Errorf("get user failed: %s", resp.Error)
						}
						mutex.Lock()
						userRetrieves++
						mutex.Unlock()
						return nil
					}},
				}

				for _, endpoint := range endpoints {
					start := time.Now()
					err := endpoint.fn()
					latency := time.Since(start)

					mutex.Lock()
					totalRequests++
					if err != nil {
						failedRequests++
						if endpoint.name == "CreateUser" || endpoint.name == "GetUser" {
							userFailures++
							fmt.Fprintf(userLog, "[%s] Error in %s: %v\n", time.Now().Format(time.RFC3339), endpoint.name, err)
						} else {
							fmt.Fprintf(file, "Error in %s: %v\n", endpoint.name, err)
						}
					} else {
						successfulRequests++
						totalLatency += latency
					}
					mutex.Unlock()
				}
			}
		}()
	}

	wg.Wait()

	// Write final results
	avgLatency := totalLatency / time.Duration(successfulRequests)
	fmt.Fprintf(file, "\nLoad Test Results:\n")
	fmt.Fprintf(file, "Total Requests: %d\n", totalRequests)
	fmt.Fprintf(file, "Successful Requests: %d\n", successfulRequests)
	fmt.Fprintf(file, "Failed Requests: %d\n", failedRequests)
	fmt.Fprintf(file, "Average Latency: %v\n", avgLatency)
	fmt.Fprintf(file, "Test Duration: %v\n", testDuration)
	fmt.Fprintf(file, "Requests/Second: %.2f\n", float64(totalRequests)/testDuration.Seconds())

	// Write user operation results
	fmt.Fprintf(userLog, "\nUser Operations Summary:\n")
	fmt.Fprintf(userLog, "Total User Creations: %d\n", userCreations)
	fmt.Fprintf(userLog, "Total User Retrievals: %d\n", userRetrieves)
	fmt.Fprintf(userLog, "Total User Operation Failures: %d\n", userFailures)
	fmt.Fprintf(userLog, "User Operations Success Rate: %.2f%%\n",
		100*float64(userCreations+userRetrieves-userFailures)/float64(userCreations+userRetrieves))

	// Write online operation results
	fmt.Fprintf(onlineLog, "\nOnline Operations Summary:\n")
	fmt.Fprintf(onlineLog, "Total Online Requests: %d\n", onlineRequests)
	fmt.Fprintf(onlineLog, "Online Operation Failures: %d\n", onlineFailures)
	fmt.Fprintf(onlineLog, "Online Operations Success Rate: %.2f%%\n",
		100*float64(onlineRequests-onlineFailures)/float64(onlineRequests))
}
