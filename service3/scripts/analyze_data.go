package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

type ServiceMetrics struct {
	TotalRequests      uint64  `json:"total_requests"`
	SuccessfulRequests uint64  `json:"successful_requests"`
	FailedRequests     uint64  `json:"failed_requests"`
	AverageLatencyMs   float64 `json:"average_latency_ms"`
}

type DatabaseStats struct {
	TotalRows        int     `json:"total_rows"`
	TableSizeMB      float64 `json:"table_size_mb"`
	FailedOperations int     `json:"failed_operations"`
}

func main() {
	if err := godotenv.Load("../.env"); err != nil {
		logrus.Warn("No .env file found or error reading it; proceeding with environment variables.")
	}

	// Connect to databases
	userDB, err := connectToDatabase("USER")
	if err != nil {
		logrus.Fatalf("Failed to connect to user database: %v", err)
	}
	defer userDB.Close()

	orderDB, err := connectToDatabase("ORDER")
	if err != nil {
		logrus.Fatalf("Failed to connect to order database: %v", err)
	}
	defer orderDB.Close()

	// Create report directory
	reportDir := "../reports"
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		logrus.Fatalf("Failed to create report directory: %v", err)
	}

	// Analyze logs
	logDir := "../logs"
	logStats := analyzeServiceLogs(logDir)

	// Get database statistics
	userStats := getDatabaseStats(userDB, "users")
	orderStats := getDatabaseStats(orderDB, "orders")

	// Generate report
	report := map[string]interface{}{
		"timestamp":     time.Now().Format(time.RFC3339),
		"service_stats": logStats,
		"database_stats": map[string]DatabaseStats{
			"users":  userStats,
			"orders": orderStats,
		},
	}

	// Write report to file
	reportFile := filepath.Join(reportDir, fmt.Sprintf("system_report_%s.json",
		time.Now().Format("2006-01-02_15-04-05")))

	reportJSON, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		logrus.Fatalf("Failed to marshal report: %v", err)
	}

	if err := os.WriteFile(reportFile, reportJSON, 0644); err != nil {
		logrus.Fatalf("Failed to write report: %v", err)
	}

	logrus.Infof("Report generated successfully: %s", reportFile)
}

func connectToDatabase(prefix string) (*sql.DB, error) {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv(prefix+"_POSTGRES_HOST"),
		os.Getenv(prefix+"_POSTGRES_PORT"),
		os.Getenv(prefix+"_POSTGRES_USER"),
		os.Getenv(prefix+"_POSTGRES_PASSWORD"),
		os.Getenv(prefix+"_POSTGRES_DB"))

	return sql.Open("postgres", connStr)
}

func analyzeServiceLogs(logDir string) map[string]ServiceMetrics {
	serviceStats := make(map[string]ServiceMetrics)

	files, err := os.ReadDir(logDir)
	if err != nil {
		logrus.Errorf("Failed to read log directory: %v", err)
		return serviceStats
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".log") {
			filePath := filepath.Join(logDir, file.Name())
			content, err := os.ReadFile(filePath)
			if err != nil {
				logrus.Errorf("Failed to read log file %s: %v", file.Name(), err)
				continue
			}

			// Extract service name from filename
			serviceName := strings.TrimSuffix(file.Name(), ".log")
			metrics := ServiceMetrics{}

			// Count failed operations
			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				if strings.Contains(line, "error") || strings.Contains(line, "failed") {
					metrics.FailedRequests++
				}
				metrics.TotalRequests++
			}

			metrics.SuccessfulRequests = metrics.TotalRequests - metrics.FailedRequests
			if metrics.SuccessfulRequests > 0 {
				metrics.AverageLatencyMs = float64(metrics.TotalRequests) / float64(metrics.SuccessfulRequests)
			}

			serviceStats[serviceName] = metrics
		}
	}

	return serviceStats
}

func getDatabaseStats(db *sql.DB, tableName string) DatabaseStats {
	var stats DatabaseStats

	// Get total rows
	row := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName))
	if err := row.Scan(&stats.TotalRows); err != nil {
		logrus.Errorf("Failed to get row count for %s: %v", tableName, err)
	}

	// Get table size
	row = db.QueryRow(fmt.Sprintf("SELECT pg_total_relation_size('%s') / (1024 * 1024.0)", tableName))
	if err := row.Scan(&stats.TableSizeMB); err != nil {
		logrus.Errorf("Failed to get table size for %s: %v", tableName, err)
	}

	// Get failed operations count (assuming there's an error_log table or similar)
	row = db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE status = 'failed' OR error IS NOT NULL", tableName))
	if err := row.Scan(&stats.FailedOperations); err != nil {
		logrus.Errorf("Failed to get failed operations count for %s: %v", tableName, err)
	}

	return stats
}
