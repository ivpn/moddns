package filter

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/proxy/mocks"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Live store reads in the filter path must carry a deadline so a dead store
// cannot hold a query past StoreDeadline.
// specRef: proxy-filtering-behaviour.md #I8
func TestFilterBlocklists_MembershipLookupHasDeadline(t *testing.T) {
	var seen context.Context
	mockCache := mocks.NewCache(t)
	mockCache.EXPECT().GetBlocklistEntry(mock.Anything, "bl1", "example.com").
		Run(func(ctx context.Context, _ string, _ string) { seen = ctx }).
		Return(false, nil).Once()

	f := NewDomainFilter(nil, mockCache, nil)
	reqCtx := newTestReqCtx(t, "deadline-profile")
	reqCtx.Blocklists = []string{"bl1"}

	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)
	dctx := &proxy.DNSContext{Req: req, Addr: netip.MustParseAddrPort("192.0.2.1:53")}

	_, err := f.filterBlocklists(reqCtx, dctx)
	require.NoError(t, err)
	require.NotNil(t, seen)
	deadline, ok := seen.Deadline()
	require.True(t, ok, "membership lookup must run under a deadline")
	remaining := time.Until(deadline)
	require.Greater(t, remaining, time.Duration(0))
	require.LessOrEqual(t, remaining, StoreDeadline)
}
