package filter

import (
	"strings"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/getsentry/sentry-go"
	"github.com/ivpn/dns/proxy/model"
	"github.com/ivpn/dns/proxy/requestcontext"
)

const (
	REASON_REBINDING = "rebinding_protection"

	// rebindingEnabledKey is the field read from the per-profile
	// settings:<id>:security:rebinding_protection Redis hash.
	rebindingEnabledKey = "enabled"
)

// filterRebinding blocks DNS answers where a public name resolves to a
// private/loopback/link-local IP (a DNS rebinding attempt). It runs in the IP
// phase at TierRebinding (150): below custom rules (T200), so a user custom Allow
// always overrides it via the Allow-wins aggregation.
//
// It is per-profile opt-in: the profile must have rebinding_protection enabled, and
// the global master switch must be on. Names matching an operator allow-suffix
// (e.g. .local) are never blocked.
func (f *IPFilter) filterRebinding(reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) (*model.StageResult, error) {
	defer sentry.Recover()

	result := &model.StageResult{Decision: model.DecisionNone, Tier: TierRebinding}

	if dctx == nil || dctx.Res == nil {
		return result, nil
	}

	// Global master switch.
	if f.RebindingConfig == nil || !f.RebindingConfig.Enabled {
		return result, nil
	}

	// Per-profile opt-in (default OFF). Absent/empty hash → not enabled.
	if reqCtx.RebindingProtectionSettings[rebindingEnabledKey] != "true" {
		return result, nil
	}

	// Operator allow-suffix list (split-horizon names like .local, .lan).
	if len(dctx.Req.Question) > 0 && isRebindingAllowedSuffix(dctx.Req.Question[0].Name, f.RebindingConfig.AllowSuffixes) {
		return result, nil
	}

	ips := extractIPsFromAnswer(dctx.Res.Answer)
	for _, ip := range ips {
		if isPrivateRebindingIP(ip, f.RebindingConfig) {
			result.Decision = model.DecisionBlock
			result.Reasons = append(result.Reasons, REASON_REBINDING)
			reqCtx.AddDomain(
				reqCtx.Logger.Debug().Str("reason", REASON_REBINDING).Str("private_ip", ip.String()),
				dctx.Req.Question[0].Name,
			).Msg("Blocked DNS rebinding (public name → private IP)")
			return result, nil
		}
	}

	return result, nil
}

// isRebindingAllowedSuffix reports whether name matches any operator allow-suffix.
// name is the DNS question name (lowercased and trailing dot trimmed before
// comparison). A suffix like ".local" matches "foo.local"; a bare "local" name
// also matches the ".local" suffix.
func isRebindingAllowedSuffix(name string, suffixes []string) bool {
	name = strings.ToLower(strings.TrimSuffix(name, "."))
	if name == "" {
		return false
	}
	for _, suffix := range suffixes {
		suffix = strings.ToLower(strings.TrimSpace(suffix))
		if suffix == "" {
			continue
		}
		if strings.HasSuffix(name, suffix) {
			return true
		}
		// Allow a bare label equal to the suffix without its leading dot
		// (e.g. name "local" with suffix ".local").
		if name == strings.TrimPrefix(suffix, ".") {
			return true
		}
	}
	return false
}
