package collector

import (
	"errors"

	"github.com/ivpn/dns/proxy/collector/channel"
	"github.com/ivpn/dns/proxy/config"
	"github.com/ivpn/dns/proxy/emitter"
	"github.com/ivpn/dns/proxy/model"
)

type Collector interface {
	Collect() error
	GetChannel() channel.CollectorChannel
}

func NewCollector(collectorCfg config.CollectorConfig, collectorType string, stopChan chan struct{}, emitter emitter.Emitter) (Collector, error) {
	switch collectorType {
	case model.TYPE_QUERY_LOGS:
		batchSize := collectorCfg.GetBatchSize()
		freq := collectorCfg.GetFrequency()
		queryLogsChan := make(chan (model.EventQueryLog), batchSize)
		return &QueryLogsCollector{
			Type:      collectorType,
			StopChan:  stopChan,
			Frequency: freq,
			BatchSize: batchSize,
			QueryChan: queryLogsChan,
			Emitter:   emitter,
		}, nil
	case model.TYPE_STATISTICS:
		batchSize := collectorCfg.GetBatchSize()
		freq := collectorCfg.GetFrequency()
		statsChan := make(chan (model.EventStatistics), batchSize)
		return &ServiceStatisticsCollector{
			Type:      collectorType,
			StopChan:  stopChan,
			Frequency: freq,
			BatchSize: batchSize,
			StatsChan: statsChan,
			Emitter:   emitter,
			Pop:       collectorCfg.GetPopName(),
		}, nil
	default:
		return nil, errors.New("unknown collector type")
	}
}
