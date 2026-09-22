package filter

import (
	"context"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/proxy/requestcontext"
)

type Filter interface {
	// Execute runs one filter phase; ctx is the request context and bounds
	// the phase's live store reads.
	Execute(ctx context.Context, reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) error
}

const (
	FilterTypeDomain = "domain"
	FilterTypeIP     = "ip"
)
