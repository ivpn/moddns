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

// Live store reads in the filter path must carry a deadline derived from the
// request context, so a dead store cannot hold a query past StoreDeadline and a
// client that hangs up cancels the work.
// specRef: proxy-filtering-behaviour.md #I8
func TestDomainFilterExecute_MembershipLookupHasRequestDeadline(t *testing.T) {
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

	// A request context that expires sooner than StoreDeadline must win.
	parentBudget := StoreDeadline / 4
	parent, cancel := context.WithTimeout(context.Background(), parentBudget)
	defer cancel()

	require.NoError(t, f.Execute(parent, reqCtx, dctx))
	require.NotNil(t, seen)
	deadline, ok := seen.Deadline()
	require.True(t, ok, "membership lookup must run under a deadline")
	remaining := time.Until(deadline)
	require.Greater(t, remaining, time.Duration(0))
	require.LessOrEqual(t, remaining, parentBudget, "deadline must derive from the request context")

	// Without a parent deadline the phase budget applies.
	seen = nil
	mockCache.EXPECT().GetBlocklistEntry(mock.Anything, "bl1", "example.com").
		Run(func(ctx context.Context, _ string, _ string) { seen = ctx }).
		Return(false, nil).Once()
	reqCtx.PartialFilteringResults = nil
	require.NoError(t, f.Execute(context.Background(), reqCtx, dctx))
	deadline, ok = seen.Deadline()
	require.True(t, ok)
	require.LessOrEqual(t, time.Until(deadline), StoreDeadline)
}
