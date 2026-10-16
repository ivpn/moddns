package channel

import (
	"errors"

	"github.com/ivpn/dns/proxy/model"
)

// EventStatisticsChannel is a struct that implements the CollectorChannel interface
// and is used to receive EventQueryLog events through a channel.
type EventStatisticsChannel struct {
	Channel chan model.EventStatistics
}

// Send method to implement the CollectorChannel interface
func (c EventStatisticsChannel) Send(data any) error {
	eventLog, ok := data.(model.EventStatistics)
	if !ok {
		return errors.New("invalid data type")
	}
	c.Channel <- eventLog
	return nil
}

// Receive method to implement the CollectorChannel interface
func (c EventStatisticsChannel) Receive() (any, error) {
	eventLog := <-c.Channel
	return eventLog, nil
}
