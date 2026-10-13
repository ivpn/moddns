package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// specRef: api-endpoint-behaviour.md J1, J20, J40 - each family resolves exactly its own set.
func TestTimespanSets(t *testing.T) {
	for _, ts := range QueryLogsTimespans {
		_, err := NewTimespan(ts)
		require.NoError(t, err, ts)
	}
	for _, ts := range []string{LAST_3_HOURS, LAST_6_HOURS, LAST_3_MONTHS, LAST_YEAR} {
		_, err := NewTimespan(ts)
		require.Error(t, err, ts)
	}
	for _, ts := range QueryLogsTopTimespans {
		_, err := NewTopTimespan(ts)
		require.NoError(t, err, ts)
	}
	require.True(t, QueryLogsTopTimespans.Contains(QueryLogsTopDefaultTimespan))

	require.Len(t, statisticsTimespans, len(StatisticsTimespanSet))
	for _, ts := range StatisticsTimespanSet {
		_, err := NewStatisticsTimespan(ts)
		require.NoError(t, err, ts)
	}
	require.True(t, StatisticsTimespanSet.Contains(StatisticsDefaultTimespan))
	require.Equal(t, "LAST_1_HOUR, LAST_12_HOURS, LAST_1_DAY, LAST_7_DAYS, LAST_MONTH", QueryLogsTimespans.String())
}
