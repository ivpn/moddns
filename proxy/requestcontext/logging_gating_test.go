package requestcontext

// Tests for conditional logging of sensitive fields (domains, client IPs).

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/libs/logging"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// testLogger returns a logger bound to an in-memory buffer.
func testLogger(enabled bool, logDomains bool) (logging.LoggerInterface, *bytes.Buffer) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).With().Timestamp().Logger()
	orig := log.Logger
	log.Logger = logger
	factory := logging.NewFactory(zerolog.DebugLevel)
	base := factory.ForProfile("test-profile", enabled)
	cfg := base.Config()
	cfg.LogDomains = logDomains
	if logDomains {
		base = factory.ForRequest(cfg)
	}
	log.Logger = orig
	return base, &buf
}

func TestAddDomain_DomainLoggingEnabled(t *testing.T) {
	logger, buf := testLogger(true, true)
	rc := NewRequestContext(context.Background(), &proxy.Proxy{}, "pid", "did",
		map[string]string{},
		map[string]string{"log_domains": "true", "enabled": "true"},
		map[string]string{},
		map[string]string{},
		map[string]string{},
		logger,
	)
	ev := rc.Logger.Info()
	rc.AddDomain(ev, "example.com").Msg("test message")
	out := buf.String()
	if !bytes.Contains([]byte(out), []byte("example.com")) {
		t.Fatalf("expected domain to be logged, output: %s", out)
	}
}

func TestAddClientIP_ClientIPLoggingEnabled(t *testing.T) {
	logger, buf := testLogger(true, false)
	cfg := logger.Config()
	rc := &RequestContext{Logger: logger, LoggerConfig: cfg}
	rc.LoggerConfig.LogClientIPs = true
	logger.Debug().Msg("reset")
	ev := logger.Debug()
	rc.AddClientIP(ev, "1.2.3.4").Msg("test client ip enabled")
	out := buf.String()
	if !strings.Contains(out, "1.2.3.4") {
		t.Fatalf("expected client ip in log, got: %s", out)
	}
}

func TestAddClientIP_ClientIPLoggingDisabled(t *testing.T) {
	logger, buf := testLogger(true, false)
	cfg := logger.Config()
	rc := &RequestContext{Logger: logger, LoggerConfig: cfg}
	rc.LoggerConfig.LogClientIPs = false
	logger.Debug().Msg("reset")
	ev := logger.Debug()
	rc.AddClientIP(ev, "1.2.3.4").Msg("test client ip disabled")
	out := buf.String()
	if strings.Contains(out, "1.2.3.4") {
		t.Fatalf("did not expect client ip in log, got: %s", out)
	}
}

func TestAddDomain_DomainLoggingDisabled(t *testing.T) {
	logger, buf := testLogger(true, false)
	rc := NewRequestContext(context.Background(), &proxy.Proxy{}, "pid", "did",
		map[string]string{},
		map[string]string{"log_domains": "false", "enabled": "true"},
		map[string]string{},
		map[string]string{},
		map[string]string{},
		logger,
	)
	ev := rc.Logger.Info()
	rc.AddDomain(ev, "example.com").Msg("test message")
	out := buf.String()
	if bytes.Contains([]byte(out), []byte("example.com")) {
		t.Fatalf("did not expect domain to be logged, output: %s", out)
	}
}

func TestMaybeDomain_DomainLoggingEnabled(t *testing.T) {
	logger, buf := testLogger(true, true)
	rc := NewRequestContext(context.Background(), &proxy.Proxy{}, "pid", "did",
		map[string]string{},
		map[string]string{"log_domains": "true", "enabled": "true"},
		map[string]string{},
		map[string]string{},
		map[string]string{},
		logger,
	)
	ev := rc.Logger.Info()
	rc.MaybeDomain(ev, "candidate", "sub.example.com").Msg("candidate")
	out := buf.String()
	if !bytes.Contains([]byte(out), []byte("sub.example.com")) {
		t.Fatalf("expected candidate domain to be logged, output: %s", out)
	}
}
