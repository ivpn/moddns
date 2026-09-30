package main

import (
	"sync"
	"testing"
	"time"

	"github.com/ivpn/dns/proxy/collector"
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
