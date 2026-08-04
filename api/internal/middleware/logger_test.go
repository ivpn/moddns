package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// specRef: logging-behaviour.md A5
func TestRequestLoggerAddsRequestID(t *testing.T) {
	var buf strings.Builder
	origLogger := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = origLogger })

	app := fiber.New()
	app.Use(requestid.New())
	app.Use(RequestLogger())
	app.Get("/", func(c *fiber.Ctx) error {
		log.Ctx(c.UserContext()).Info().Msg("handler log")
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-ID", "req-test-1")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}

	out := buf.String()
	if !strings.Contains(out, `"request_id":"req-test-1"`) {
		t.Errorf("expected request_id on handler log line, got %q", out)
	}
	if !strings.Contains(out, "handler log") {
		t.Errorf("expected handler message in output, got %q", out)
	}
}

// specRef: logging-behaviour.md A5
func TestRequestLoggerAttachesNoAccountIdentifiers(t *testing.T) {
	var buf strings.Builder
	origLogger := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = origLogger })

	app := fiber.New()
	app.Use(requestid.New())
	app.Use(RequestLogger())
	app.Get("/", func(c *fiber.Ctx) error {
		log.Ctx(c.UserContext()).Info().Msg("handler log")
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/", nil)
	if _, err := app.Test(req); err != nil {
		t.Fatalf("request failed: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "account_id") {
		t.Errorf("request-scoped logger must not carry account_id, got %q", out)
	}
}
