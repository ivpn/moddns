package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestPromUpdates_RecordsAllSeries(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewPromUpdates(reg)

	const source = "blp_test"

	m.RecordUpdate(source, StatusSuccess)
	m.RecordUpdate(source, StatusFailure)
	m.RecordUpdate(source, StatusFailure)
	m.SetDomainsExtracted(source, 1234)
	m.SetLastSuccess(source, time.Unix(1700000000, 0))
	m.RecordDownloadBytes(source, 4096)
	m.RecordValidationRejected(source, ReasonShrink)

	if got := testutil.ToFloat64(m.updates.WithLabelValues(source, StatusSuccess)); got != 1 {
		t.Errorf("update_total{success} = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.updates.WithLabelValues(source, StatusFailure)); got != 2 {
		t.Errorf("update_total{failure} = %v, want 2", got)
	}
	if got := testutil.ToFloat64(m.domainsExtracted.WithLabelValues(source)); got != 1234 {
		t.Errorf("domains_extracted = %v, want 1234", got)
	}
	if got := testutil.ToFloat64(m.lastSuccess.WithLabelValues(source)); got != 1700000000 {
		t.Errorf("last_success = %v, want 1700000000", got)
	}
	if got := testutil.ToFloat64(m.downloadBytes.WithLabelValues(source)); got != 4096 {
		t.Errorf("download_bytes = %v, want 4096", got)
	}
	if got := testutil.ToFloat64(m.validationRejects.WithLabelValues(source, ReasonShrink)); got != 1 {
		t.Errorf("validation_rejected{shrink} = %v, want 1", got)
	}
}

func TestPromUpdates_DurationObserved(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewPromUpdates(reg)

	m.RecordDuration("blp_test", 250*time.Millisecond)

	// One observation should have been recorded in the histogram.
	if got := testutil.CollectAndCount(m.updateDuration); got != 1 {
		t.Errorf("update_duration series count = %d, want 1", got)
	}
}

func TestNoopUpdates_DoesNotPanic(t *testing.T) {
	var m Updates = NoopUpdates{}
	m.RecordUpdate("s", StatusSuccess)
	m.RecordDuration("s", time.Second)
	m.SetDomainsExtracted("s", 1)
	m.SetLastSuccess("s", time.Now())
	m.RecordDownloadBytes("s", 1)
	m.RecordValidationRejected("s", ReasonEmpty)
}
