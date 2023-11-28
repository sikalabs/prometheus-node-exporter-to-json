package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Node struct {
	Instance string
	Hostname string
}

var VERSION = "v0.1.0"
var PROMETHEUS_URL string
var NODES = []Node{}
var HOSTNAME string

func memAvailableBytes(node string) (float64, error) {
	return queryPrometheusToValue(
		`node_memory_MemAvailable_bytes{job="node-exporter", instance="` + node + `"}`,
	)
}

func memTotalBytes(node string) (float64, error) {
	return queryPrometheusToValue(
		`node_memory_MemTotal_bytes{job="node-exporter", instance="` + node + `"}`,
	)
}

func memUsagePercent(node string) (float64, error) {
	return queryPrometheusToValue(
		`100 -
    (
      avg(node_memory_MemAvailable_bytes{job="node-exporter", instance="` + node + `"}) /
      avg(node_memory_MemTotal_bytes{job="node-exporter", instance="` + node + `"})
    * 100
    )`,
	)
}

func fileSystemAvailableBytes(node string) (float64, error) {
	return queryPrometheusToValue(
		`node_filesystem_avail_bytes{job="node-exporter", instance="` + node + `", fstype!="", mountpoint="/"}`,
	)
}

func fileSystemTotalBytes(node string) (float64, error) {
	return queryPrometheusToValue(
		`node_filesystem_size_bytes{job="node-exporter", instance="` + node + `", fstype!="", mountpoint="/"}`,
	)
}

func fileSystemUsagePercent(node string) (float64, error) {
	return queryPrometheusToValue(
		`100 -
    (
      node_filesystem_avail_bytes{job="node-exporter", instance="` + node + `", fstype!="", mountpoint="/"} /
      node_filesystem_size_bytes{job="node-exporter", instance="` + node + `", fstype!="", mountpoint="/"}
    * 100
    )`,
	)
}

func cpuUsagePercent(node string) (float64, error) {
	return queryPrometheusToValue(
		`(1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle",  instance="` + node + `"}[5m]))) * 100`,
	)
}

func cpuCoresCount(node string) (float64, error) {
	return queryPrometheusToValue(
		`count(node_cpu_seconds_total{mode="idle", instance="` + node + `"})`,
	)
}

type NodeMetrics struct {
	Instance                 string  `json:"instance"`
	Hostname                 string  `json:"hostname"`
	MemTotalBytes            float64 `json:"mem_total_bytes"`
	MemAvailableBytes        float64 `json:"mem_available_bytes"`
	MemUsagePercent          float64 `json:"mem_usage_percent"`
	FileSystemAvailableBytes float64 `json:"filesystem_available_bytes"`
	FileSystemTotalBytes     float64 `json:"filesystem_total_bytes"`
	FileSystemUsagePercent   float64 `json:"filesystem_usage_percent"`
	CpuUsagePercent          float64 `json:"cpu_usage_percent"`
	CpuCoresCount            float64 `json:"cpu_cores_count"`
}

type Metadata struct {
	Hostname string `json:"hostname"`
	Version  string `json:"version"`
}

type Response struct {
	Metadata     Metadata      `json:"metadata"`
	Status       string        `json:"status"`
	ErrorCode    string        `json:"error_code"`
	ErrorMessage string        `json:"error_message"`
	Metrics      []NodeMetrics `json:"metrics"`
}

func main() {
	PROMETHEUS_URL = os.Getenv("PROMETHEUS_URL")
	if PROMETHEUS_URL == "" {
		logFatal("environment vaiable PROMETHEUS_URL is required")
	}
	nodesListEnv := os.Getenv("NODES")
	if nodesListEnv == "" {
		logFatal("environment vaiable NODES is required")
	}
	nodesEnv := strings.Split(nodesListEnv, ",")
	for _, nodeEnv := range nodesEnv {
		nodeEnvSplit := strings.Split(nodeEnv, "=")
		instance := nodeEnvSplit[0]
		hostname := ""
		if len(nodeEnvSplit) > 1 {
			hostname = nodeEnvSplit[1]
		}
		NODES = append(NODES, Node{
			Instance: instance,
			Hostname: hostname,
		})
	}

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	HOSTNAME, _ = os.Hostname()

	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		isOK := true
		metrics := []NodeMetrics{}
		for _, node := range NODES {
			memTotalBytes, err := memTotalBytes(node.Instance)
			isOK = isOK && err == nil
			memAvailableBytes, err := memAvailableBytes(node.Instance)
			isOK = isOK && err == nil
			memUsagePercent, err := memUsagePercent(node.Instance)
			isOK = isOK && err == nil
			fileSystemAvailableBytes, err := fileSystemAvailableBytes(node.Instance)
			isOK = isOK && err == nil
			fileSystemTotalBytes, err := fileSystemTotalBytes(node.Instance)
			isOK = isOK && err == nil
			fileSystemUsagePercent, err := fileSystemUsagePercent(node.Instance)
			isOK = isOK && err == nil
			cpuUsagePercent, err := cpuUsagePercent(node.Instance)
			isOK = isOK && err == nil
			cpuCoresCount, err := cpuCoresCount(node.Instance)
			isOK = isOK && err == nil

			metrics = append(metrics, NodeMetrics{
				Instance:                 node.Instance,
				Hostname:                 node.Hostname,
				MemTotalBytes:            memTotalBytes,
				MemAvailableBytes:        memAvailableBytes,
				MemUsagePercent:          memUsagePercent,
				FileSystemAvailableBytes: fileSystemAvailableBytes,
				FileSystemTotalBytes:     fileSystemTotalBytes,
				FileSystemUsagePercent:   fileSystemUsagePercent,
				CpuUsagePercent:          cpuUsagePercent,
				CpuCoresCount:            cpuCoresCount,
			})
		}
		status := "OK"
		errorCode := ""
		errorMessage := ""
		if !isOK {
			status = "ERR"
			errorCode = "ERROR"
			errorMessage = "Some metrics are not available"
		}
		data, _ := json.Marshal(Response{
			Metadata: Metadata{
				Hostname: HOSTNAME,
				Version:  VERSION,
			},
			Status:       status,
			ErrorCode:    errorCode,
			ErrorMessage: errorMessage,
			Metrics:      metrics,
		})
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		if isOK {
			logDebug("GET / OK")
		} else {
			logErrorWithCode(errorCode, "GET / ERR: "+errorMessage)
		}
	})

	logInfo("Starting server on port 0.0.0.0:8000, http://127.0.0.1:8000")

	err := http.ListenAndServe(":8000", nil)
	if err != nil {
		logFatal(err.Error())
	}
}

func queryPrometheusToValue(query string) (float64, error) {
	client, err := api.NewClient(api.Config{
		Address: PROMETHEUS_URL,
	})
	if err != nil {
		return 0, err
	}
	queryClient := v1.NewAPI(client)
	result, err := queryClient.Query(context.Background(), query, time.Now())
	if err != nil {
		return 0, err
	}
	return float64(result.(model.Vector)[0].Value), nil
}

func logDebug(msg string) {
	log.Debug().
		Str("version", VERSION).
		Str("hostname", HOSTNAME).
		Msg(msg)
}

func logInfo(msg string) {
	log.Info().
		Str("version", VERSION).
		Str("hostname", HOSTNAME).
		Msg(msg)
}

func logErrorWithCode(code, msg string) {
	log.Error().
		Str("version", VERSION).
		Str("hostname", HOSTNAME).
		Str("error_code", code).
		Msg(msg)
}

func logFatal(msg string) {
	log.Fatal().
		Str("version", VERSION).
		Str("hostname", HOSTNAME).
		Msg(msg)
	os.Exit(1)
}
