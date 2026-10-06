package collector

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	"github.com/ivpn/dns/proxy/mocks"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// newNoEmitter returns an emitter mock with no expectations: any emit fails the test.
func newNoEmitter(t *testing.T) *mocks.Emitter { return mocks.NewEmitter(t) }

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) count(substr string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.Count(b.buf.String(), substr)
}

// captureLogs routes the global logger into w at debug level until restore is called.
func captureLogs(w *lockedBuffer) (restore func()) {
	old := log.Logger
	log.Logger = zerolog.New(w).Level(zerolog.DebugLevel)
	return func() { log.Logger = old }
}
