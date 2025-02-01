package visualization

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
)

type MetricsData struct {
	Timestamp        time.Time
	TotalRequests    uint64
	SuccessRequests  uint64
	FailedRequests   uint64
	AverageLatency   float64
	ActiveConns      int32
	DatabaseSizeMB   float64
	MessagesReceived int64
}

func GenerateMetricsVisuals(metricsFile string, outputDir string) error {
	// Read metrics data
	data, err := os.ReadFile(metricsFile)
	if err != nil {
		return fmt.Errorf("failed to read metrics file: %v", err)
	}

	var metrics []MetricsData
	if err := json.Unmarshal(data, &metrics); err != nil {
		return fmt.Errorf("failed to parse metrics data: %v", err)
	}

	// Create visualizations
	if err := generateRequestsChart(metrics, outputDir); err != nil {
		return err
	}

	if err := generateLatencyChart(metrics, outputDir); err != nil {
		return err
	}

	if err := generateDatabaseChart(metrics, outputDir); err != nil {
		return err
	}

	return nil
}

func generateRequestsChart(metrics []MetricsData, outputDir string) error {
	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{
			Title: "Request Metrics Over Time",
		}),
		charts.WithTooltipOpts(opts.Tooltip{Show: opts.Bool(true)}),
		charts.WithLegendOpts(opts.Legend{Show: opts.Bool(true)}),
		charts.WithInitializationOpts(opts.Initialization{
			Theme: types.ThemeWesteros,
		}),
	)

	xAxis := make([]string, len(metrics))
	totalReqs := make([]opts.LineData, len(metrics))
	successReqs := make([]opts.LineData, len(metrics))
	failedReqs := make([]opts.LineData, len(metrics))

	for i, m := range metrics {
		xAxis[i] = m.Timestamp.Format("15:04:05")
		totalReqs[i] = opts.LineData{Value: m.TotalRequests}
		successReqs[i] = opts.LineData{Value: m.SuccessRequests}
		failedReqs[i] = opts.LineData{Value: m.FailedRequests}
	}

	line.SetXAxis(xAxis).
		AddSeries("Total Requests", totalReqs).
		AddSeries("Successful Requests", successReqs).
		AddSeries("Failed Requests", failedReqs)

	f, err := os.Create(fmt.Sprintf("%s/requests.html", outputDir))
	if err != nil {
		return err
	}
	defer f.Close()

	return line.Render(f)
}

func generateLatencyChart(metrics []MetricsData, outputDir string) error {
	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{
			Title: "Average Latency Over Time",
		}),
		charts.WithTooltipOpts(opts.Tooltip{Show: opts.Bool(true)}),
		charts.WithYAxisOpts(opts.YAxis{
			Name: "Latency (ms)",
		}),
	)

	xAxis := make([]string, len(metrics))
	latency := make([]opts.LineData, len(metrics))

	for i, m := range metrics {
		xAxis[i] = m.Timestamp.Format("15:04:05")
		latency[i] = opts.LineData{Value: m.AverageLatency}
	}

	line.SetXAxis(xAxis).AddSeries("Average Latency", latency)

	f, err := os.Create(fmt.Sprintf("%s/latency.html", outputDir))
	if err != nil {
		return err
	}
	defer f.Close()

	return line.Render(f)
}

func generateDatabaseChart(metrics []MetricsData, outputDir string) error {
	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{
			Title: "Database Metrics Over Time",
		}),
		charts.WithTooltipOpts(opts.Tooltip{Show: opts.Bool(true)}),
		charts.WithLegendOpts(opts.Legend{Show: opts.Bool(true)}),
	)

	xAxis := make([]string, len(metrics))
	activeConns := make([]opts.LineData, len(metrics))
	dbSize := make([]opts.LineData, len(metrics))

	for i, m := range metrics {
		xAxis[i] = m.Timestamp.Format("15:04:05")
		activeConns[i] = opts.LineData{Value: m.ActiveConns}
		dbSize[i] = opts.LineData{Value: m.DatabaseSizeMB}
	}

	line.SetXAxis(xAxis).
		AddSeries("Active Connections", activeConns).
		AddSeries("Database Size (MB)", dbSize)

	f, err := os.Create(fmt.Sprintf("%s/database.html", outputDir))
	if err != nil {
		return err
	}
	defer f.Close()

	return line.Render(f)
}
