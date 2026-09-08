package filter

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/ivpn/dns/proxy/model"
	"github.com/ivpn/dns/proxy/requestcontext"
)

// Stage names double as the `stage` label of
// proxy_dns_filter_stage_errors_total, so they are stable identifiers.
const (
	StageBlocklists     = "blocklists"
	StageCustomRules    = "custom_rules"
	StageServiceDomains = "service_domains"
	StageDefaultRule    = "default_rule"
	StageServices       = "services"
	StageRebinding      = "rebinding"
	StageCNAME          = "cname"
)

// StoreDeadline bounds the live settings-store reads of one pipeline step
// (admission batch, domain phase, IP phase), each derived from the request
// context so a client hang-up cancels the work. Three steps keep the worst
// case under the 5s most stub resolvers wait before giving up.
const StoreDeadline = time.Second

// storeContext returns the context for one phase's live store reads.
func storeContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, StoreDeadline)
}

// StageErrorRecorder receives one call per failed filter stage. A nil recorder
// disables metrics without disabling the failure handling.
type StageErrorRecorder interface {
	RecordFilterStageError(phase, stage string)
}

type stageFunc func(ctx context.Context, reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) (*model.StageResult, error)

type stage struct {
	name string
	run  stageFunc
}

// runStages executes every stage concurrently and appends the successful
// results to reqCtx.PartialFilteringResults. A stage that returns an error
// contributes no result; every such failure is recorded and the joined error is
// returned, which the caller must treat as StatusUnavailable rather than
// aggregate the partial results (spec: proxy-filtering-behaviour.md Section I).
func runStages(ctx context.Context, phase string, stages []stage, metrics StageErrorRecorder, reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) error {
	ctx, cancel := storeContext(ctx)
	defer cancel()

	type outcome struct {
		res *model.StageResult
		err error
	}
	outcomes := make([]outcome, len(stages))

	var wg sync.WaitGroup
	wg.Add(len(stages))
	for i, st := range stages {
		go func(i int, st stage) {
			defer wg.Done()
			res, err := st.run(ctx, reqCtx, dctx)
			outcomes[i] = outcome{res: res, err: err}
		}(i, st)
	}
	wg.Wait()

	var errs []error
	for i, st := range stages {
		out := outcomes[i]
		if out.err != nil {
			if metrics != nil {
				metrics.RecordFilterStageError(phase, st.name)
			}
			reqCtx.Logger.Debug().Err(out.err).Str("filter_type", phase).Str("stage", st.name).Msg("Filter stage failed")
			errs = append(errs, fmt.Errorf("%s/%s: %w", phase, st.name, out.err))
			continue
		}
		if out.res != nil {
			reqCtx.PartialFilteringResults = append(reqCtx.PartialFilteringResults, *out.res)
		}
	}
	return errors.Join(errs...)
}
