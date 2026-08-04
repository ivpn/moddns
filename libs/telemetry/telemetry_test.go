package telemetry

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	sentryzerolog "github.com/getsentry/sentry-go/zerolog"
	"github.com/rs/zerolog"
)

// mockTransport records events the Sentry client would deliver.
type mockTransport struct {
	mu     sync.Mutex
	events []*sentry.Event
}

func (t *mockTransport) Configure(sentry.ClientOptions) {}
func (t *mockTransport) SendEvent(event *sentry.Event) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, event)
}
func (t *mockTransport) Flush(time.Duration) bool { return true }
func (t *mockTransport) Close()                   {}

func (t *mockTransport) Events() []*sentry.Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]*sentry.Event(nil), t.events...)
}

// testConfig uses a syntactically valid DSN: an empty DSN silently disables
// the client and would make every assertion pass vacuously.
var testConfig = Config{DSN: "https://public@sentry.invalid/1", Environment: "test"}

func newTestLogger(t *testing.T) (zerolog.Logger, *mockTransport) {
	t.Helper()
	transport := &mockTransport{}
	cfg := LogWriterConfig(testConfig)
	cfg.ClientOptions.Transport = transport
	writer, err := sentryzerolog.New(cfg)
	if err != nil {
		t.Fatalf("failed to create sentry writer: %v", err)
	}
	t.Cleanup(func() { _ = writer.Close() })
	return zerolog.New(writer), transport
}

// specRef: logging-behaviour.md D1, D3 — sensitive fields are dropped from
// delivered events while other fields pass through.
func TestErrorEventFieldsScrubbed(t *testing.T) {
	logger, transport := newTestLogger(t)

	logger.Error().
		Str("email", "user@example.com").
		Str("session_id", "sess-123").
		Str("account_id", "acc-123").
		Str("search", "example.org").
		Str("profile_id", "prof-123").
		Str("key", "203.0.113.7").
		Str("request_id", "control-1").
		Msg("operation failed")

	events := transport.Events()
	if len(events) != 1 {
		t.Fatalf("Expected exactly one delivered event, got %d", len(events))
	}
	extra := events[0].Extra

	// Control assertion: proves the harness observes real deliveries.
	if got, ok := extra["request_id"]; !ok || got != "control-1" {
		t.Fatalf("Expected control field request_id=control-1 in Extra, got %v", extra)
	}

	for _, key := range []string{"email", "session_id", "account_id", "search", "profile_id", "key"} {
		if _, ok := extra[key]; ok {
			t.Errorf("Expected sensitive field %q to be scrubbed from Extra, got %v", key, extra[key])
		}
	}
}

// specRef: logging-behaviour.md C2 — only Error and above become events.
func TestWarnProducesNoEvent(t *testing.T) {
	logger, transport := newTestLogger(t)

	logger.Warn().Str("email", "user@example.com").Msg("something odd")
	logger.Info().Str("email", "user@example.com").Msg("informational")

	if events := transport.Events(); len(events) != 0 {
		t.Fatalf("Expected no delivered events for sub-error levels, got %d", len(events))
	}
}

// specRef: logging-behaviour.md D2 — addresses embedded in error strings are
// redacted before delivery.
func TestErrorStringEmailRedacted(t *testing.T) {
	logger, transport := newTestLogger(t)

	logger.Error().
		Err(errors.New(`address "user@example.com" rejected`)).
		Msg("send failed for user@example.com")

	events := transport.Events()
	if len(events) != 1 {
		t.Fatalf("Expected exactly one delivered event, got %d", len(events))
	}
	event := events[0]

	if strings.Contains(event.Message, "user@example.com") {
		t.Errorf("Expected email redacted from message, got %q", event.Message)
	}
	if len(event.Exception) == 0 {
		t.Fatal("Expected an exception on the event")
	}
	for _, exc := range event.Exception {
		if strings.Contains(exc.Value, "user@example.com") {
			t.Errorf("Expected email redacted from exception value, got %q", exc.Value)
		}
		if !strings.Contains(exc.Value, "[redacted-email]") {
			t.Errorf("Expected redaction marker in exception value, got %q", exc.Value)
		}
	}
}

// specRef: logging-behaviour.md D2 — addresses embedded in surviving Extra
// string values are redacted before delivery.
func TestExtraStringValuesEmailRedacted(t *testing.T) {
	logger, transport := newTestLogger(t)

	logger.Error().
		Str("provider_error", `550 address "user@example.com" rejected`).
		Msg("send failed")

	events := transport.Events()
	if len(events) != 1 {
		t.Fatalf("Expected exactly one delivered event, got %d", len(events))
	}
	got, ok := events[0].Extra["provider_error"].(string)
	if !ok {
		t.Fatalf("Expected provider_error to survive as a string, got %v", events[0].Extra)
	}
	if strings.Contains(got, "user@example.com") {
		t.Errorf("Expected email redacted from Extra value, got %q", got)
	}
	if !strings.Contains(got, "[redacted-email]") {
		t.Errorf("Expected redaction marker in Extra value, got %q", got)
	}
}

// specRef: logging-behaviour.md D4 — request payloads and credential-bearing
// headers never leave the host.
func TestScrubClearsRequestData(t *testing.T) {
	event := sentry.NewEvent()
	event.Request = &sentry.Request{
		URL:         "https://api.example.net/api/v1/login",
		Data:        `{"email":"user@example.com","password":"hunter2"}`,
		Cookies:     "session=abc",
		QueryString: "next=/dashboard",
		Headers: map[string]string{
			"Authorization":   "Bearer tok",
			"Cookie":          "session=abc",
			"X-Forwarded-For": "203.0.113.7",
			"X-Real-Ip":       "203.0.113.7",
			"User-Agent":      "Mozilla/5.0 (X11; Linux x86_64)",
			"Content-Type":    "application/json",
		},
	}

	scrubbed := Scrub(event, nil)

	if scrubbed == nil {
		t.Fatal("Expected event to be delivered after scrubbing, got nil")
	}
	req := scrubbed.Request
	if req.Data != "" || req.Cookies != "" || req.QueryString != "" {
		t.Errorf("Expected request data/cookies/query cleared, got %+v", req)
	}
	for _, header := range []string{"Authorization", "Cookie", "X-Forwarded-For", "X-Real-Ip", "User-Agent"} {
		if _, ok := req.Headers[header]; ok {
			t.Errorf("Expected header %q to be removed", header)
		}
	}
	if _, ok := req.Headers["Content-Type"]; !ok {
		t.Error("Expected benign header Content-Type to survive")
	}
}

// specRef: logging-behaviour.md D4 — the global-hub client options carry the
// same scrubbing hooks as the log writer.
func TestInitOptionsDeliveryScrubbed(t *testing.T) {
	transport := &mockTransport{}
	opts := InitOptions(testConfig)
	opts.Transport = transport

	client, err := sentry.NewClient(opts)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	hub := sentry.NewHub(client, sentry.NewScope())

	event := sentry.NewEvent()
	event.Level = sentry.LevelError
	event.Message = "boom"
	event.Extra["email"] = "user@example.com"
	event.Extra["request_id"] = "control-2"
	hub.CaptureEvent(event)

	events := transport.Events()
	if len(events) != 1 {
		t.Fatalf("Expected exactly one delivered event, got %d", len(events))
	}
	if got, ok := events[0].Extra["request_id"]; !ok || got != "control-2" {
		t.Fatalf("Expected control field request_id=control-2, got %v", events[0].Extra)
	}
	if _, ok := events[0].Extra["email"]; ok {
		t.Error("Expected email scrubbed from hub-captured event")
	}
}

// specRef: logging-behaviour.md D5 — breadcrumbs are scrubbed before being
// attached to the shared scope.
func TestScrubBreadcrumb(t *testing.T) {
	crumb := &sentry.Breadcrumb{
		Message: "lookup for user@example.com",
		Data: map[string]any{
			"email":      "user@example.com",
			"request_id": "control-3",
		},
	}

	scrubbed := ScrubBreadcrumb(crumb, nil)

	if scrubbed == nil {
		t.Fatal("Expected breadcrumb to survive scrubbing")
	}
	if strings.Contains(scrubbed.Message, "user@example.com") {
		t.Errorf("Expected email redacted from breadcrumb message, got %q", scrubbed.Message)
	}
	if _, ok := scrubbed.Data["email"]; ok {
		t.Error("Expected email scrubbed from breadcrumb data")
	}
	if _, ok := scrubbed.Data["request_id"]; !ok {
		t.Error("Expected benign breadcrumb data to survive")
	}
}

// specRef: logging-behaviour.md C2, D5 — writer configuration constants.
func TestLogWriterConfigShape(t *testing.T) {
	cfg := LogWriterConfig(testConfig)

	if cfg.Options.WithBreadcrumbs {
		t.Error("Expected WithBreadcrumbs to be disabled for the zerolog writer")
	}
	wantLevels := map[zerolog.Level]bool{
		zerolog.ErrorLevel: true, zerolog.FatalLevel: true, zerolog.PanicLevel: true,
	}
	if len(cfg.Options.Levels) != len(wantLevels) {
		t.Fatalf("Expected exactly %d event levels, got %v", len(wantLevels), cfg.Options.Levels)
	}
	for _, level := range cfg.Options.Levels {
		if !wantLevels[level] {
			t.Errorf("Unexpected event level %v in writer config", level)
		}
	}
	if cfg.ClientOptions.BeforeSend == nil || cfg.ClientOptions.BeforeSendTransaction == nil || cfg.ClientOptions.BeforeBreadcrumb == nil {
		t.Error("Expected scrub hooks installed on the writer client options")
	}
}
