package main

import (
	"sync"
	"testing"
	"time"

	"github.com/ivpn/dns/proxy/collector"
	"github.com/ivpn/dns/proxy/mocks"
	"github.com/stretchr/testify/assert"
)

// specRef: proxy-statistics-behaviour.md #Y18
func TestTrackRun_WaitCoversARestartedRun(t *testing.T) {
	var wg sync.WaitGroup
	calls := 0
	run := trackRun(&wg, func() {
		calls++
		if calls == 1 {
			panic("collector crashed")
		}
	})

	assert.Panics(t, run)
	assert.False(t, waitFor(&wg, 30*time.Millisecond), "a crashed run is not a finished flush")

	run() // the restart, as safelyRun does
	assert.True(t, waitFor(&wg, time.Second))
}

// specRef: proxy-statistics-behaviour.md #Y18
func TestCollectorFlushTimeoutCoversBothEmits(t *testing.T) {
	assert.GreaterOrEqual(t, collectorFlushTimeout, 2*collector.EmitTimeout)
}

// shutdown runs twice on a signal exit (explicitly, then deferred).
func TestShutdown_RunsOnce(t *testing.T) {
	e := mocks.NewEmitter(t)
	e.On("Disconnect").Return(nil).Once()
	stops := 0

	shutdown(nil, e, nil, nil, func() { stops++ })
	shutdown(nil, e, nil, nil, func() { stops++ })

	assert.Equal(t, 1, stops)
}
