package mongodb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// specRef: api-endpoint-behaviour.md J23 - the top-list aggregations carry a server-side time
// limit, so an abandoned request cannot keep a multi-second bucket unpack running.
func TestQueryLogTopAggregateOptions_MaxTime(t *testing.T) {
	opts := queryLogTopAggregateOptions()
	require.NotNil(t, opts.MaxTime)
	require.Equal(t, 15*time.Second, *opts.MaxTime)
}
