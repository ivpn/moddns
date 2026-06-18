package metrics

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func doGet(t *testing.T, h http.Handler, path string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body, _ := io.ReadAll(rec.Result().Body)
	return rec.Code, string(body)
}

func TestHealthLive_AlwaysOK(t *testing.T) {
	h := handler(func(context.Context) error { return errors.New("deps down") })
	if code, _ := doGet(t, h, "/health/live"); code != http.StatusOK {
		t.Errorf("/health/live = %d, want 200", code)
	}
}

func TestHealthReady(t *testing.T) {
	t.Run("ready", func(t *testing.T) {
		h := handler(func(context.Context) error { return nil })
		if code, _ := doGet(t, h, "/health/ready"); code != http.StatusOK {
			t.Errorf("/health/ready = %d, want 200", code)
		}
	})
	t.Run("not ready", func(t *testing.T) {
		h := handler(func(context.Context) error { return errors.New("mongo down") })
		if code, _ := doGet(t, h, "/health/ready"); code != http.StatusServiceUnavailable {
			t.Errorf("/health/ready = %d, want 503", code)
		}
	})
	t.Run("nil ready func", func(t *testing.T) {
		h := handler(nil)
		if code, _ := doGet(t, h, "/health/ready"); code != http.StatusOK {
			t.Errorf("/health/ready = %d, want 200", code)
		}
	})
}

func TestMetricsEndpoint(t *testing.T) {
	// Register blocklist collectors on the default registry so promhttp exposes them.
	NewPromUpdates(prometheus.DefaultRegisterer).RecordUpdate("blp_test", StatusSuccess)

	h := handler(nil)
	code, body := doGet(t, h, "/metrics")
	if code != http.StatusOK {
		t.Fatalf("/metrics = %d, want 200", code)
	}
	if !strings.Contains(body, "blocklists_update_total") {
		t.Errorf("/metrics body missing blocklists_update_total; got:\n%s", body)
	}
}
