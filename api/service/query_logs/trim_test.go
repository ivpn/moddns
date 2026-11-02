package querylogs_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ivpn/dns/api/mocks"
	"github.com/ivpn/dns/api/model"
	querylogs "github.com/ivpn/dns/api/service/query_logs"
)

// specRef: api-endpoint-behaviour.md J54 — the trim measures the retention from now.
func TestTrimToRetention(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	repo := mocks.NewQueryLogsRepository(t)
	svc := querylogs.NewQueryLogsService(repo, querylogs.WithClock(func() time.Time { return now }))
	repo.On("TrimQueryLogs", mock.Anything, "p1", model.RetentionOneDay, now).Return(int64(4), nil).Once()

	deleted, err := svc.TrimToRetention(context.Background(), "p1", model.RetentionOneDay)
	require.NoError(t, err)
	require.EqualValues(t, 4, deleted)
}
