package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ivpn/dns/proxy/model"
)

type CollectorConfig interface {
	GetBatchSize() int
	GetFrequency() time.Duration
	GetPopName() string
}

type BatchCollectorConfig struct {
	Type      string
	BatchSize int
	Frequency time.Duration
	// PopName labels service-wide statistics documents; set for the
	// statistics collector only.
	PopName string
}

func (b *BatchCollectorConfig) GetBatchSize() int {
	return b.BatchSize
}

func (b *BatchCollectorConfig) GetFrequency() time.Duration {
	return b.Frequency
}

func (b *BatchCollectorConfig) GetPopName() string {
	return b.PopName
}

func NewCollectorConfig(collectorType string) (CollectorConfig, error) {
	switch collectorType {
	case model.TYPE_QUERY_LOGS:
		return loadQueryLogsCollectorConfig()
	case model.TYPE_STATISTICS:
		return loadStatisticsCollectorConfig()
	default:
		return nil, errors.New("unknown collector config type")
	}
}

func loadQueryLogsCollectorConfig() (*BatchCollectorConfig, error) {
	bs := os.Getenv("COLLECTOR_QUERY_LOGS_BATCH_SIZE")
	if bs == "" {
		bs = "100"
	}
	batchSize, err := strconv.Atoi(bs)
	if err != nil {
		return nil, err
	}

	freq := os.Getenv("COLLECTOR_QUERY_LOGS_BATCH_INTERVAL")
	if freq == "" {
		freq = "10s"
	}
	interval, err := time.ParseDuration(freq)
	if err != nil {
		return nil, err
	}

	return &BatchCollectorConfig{
		Type:      model.TYPE_QUERY_LOGS,
		BatchSize: batchSize,
		Frequency: interval,
	}, nil
}

func loadStatisticsCollectorConfig() (*BatchCollectorConfig, error) {
	bs := os.Getenv("COLLECTOR_SERVICE_STATISTICS_BATCH_SIZE")
	if bs == "" {
		bs = "10000"
	}
	batchSize, err := strconv.Atoi(bs)
	if err != nil {
		return nil, err
	}

	freq := os.Getenv("COLLECTOR_SERVICE_STATISTICS_BATCH_INTERVAL")
	if freq == "" {
		freq = "30s"
	}
	interval, err := time.ParseDuration(freq)
	if err != nil {
		return nil, err
	}

	return &BatchCollectorConfig{
		Type:      model.TYPE_STATISTICS,
		BatchSize: batchSize,
		Frequency: interval,
		PopName:   loadPopName(),
	}, nil
}

const defaultPopName = "unknown"

// loadPopName is the PoP label on service-wide statistics: POP_NAME, else the
// host name, else "unknown". It never identifies a profile or a client.
func loadPopName() string {
	if v := strings.TrimSpace(os.Getenv("POP_NAME")); v != "" {
		return v
	}
	if h, err := os.Hostname(); err == nil && strings.TrimSpace(h) != "" {
		return strings.TrimSpace(h)
	}
	return defaultPopName
}
