package channel

import (
	"errors"

	"github.com/ivpn/dns/proxy/model"
)

// EventQueryLogChannel is a struct that implements the CollectorChannel interface
// and is used to receive EventQueryLog events through a channel.
type EventQueryLogChannel struct {
	Channel chan model.EventQueryLog
}

// Send method to implement the CollectorChannel interface
func (c EventQueryLogChannel) Send(data any) error {
	eventLog, ok := data.(model.EventQueryLog)
	if !ok {
		return errors.New("invalid data type")
	}
	c.Channel <- eventLog
	return nil
}

// Receive method to implement the CollectorChannel interface
func (c EventQueryLogChannel) Receive() (any, error) {
	eventLog := <-c.Channel
	return eventLog, nil
}
