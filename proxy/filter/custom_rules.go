package filter

import (
	"context"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/getsentry/sentry-go"
	"github.com/ivpn/dns/proxy/model"
	"github.com/ivpn/dns/proxy/requestcontext"
	"github.com/miekg/dns"
	"github.com/rs/zerolog/log"
)

const (
	ACTION_ALLOW        = "allow"
	ACTION_BLOCK        = "block"
	REASON_CUSTOM_RULES = "custom_rules"
	WILDCARD            = "*"
)

// matchDomain checks if the domain matches the pattern, handling wildcards
// and special domain formats used in custom rules.
//
// Rules:
//   - "ads.com"      => matches only ads.com
//   - "*.ads.com"    => matches ads.com and all its subdomains
//   - ".ads.com"     => treated as "*.ads.com" (same as above)
//   - "ads.*"        => matches ads.<any TLD>, but not the bare "ads" and not subdomains (e.g. sub.ads.com)
//   - "*ads*"        => matches any domain containing "ads" as a substring
func (f *DomainFilter) matchDomain(domain, pattern string) bool {
	domain = strings.ToLower(domain)
	pattern = strings.ToLower(pattern)

	if pattern == "" {
		return false
	}

	// Support ".example.com" syntax by treating it as "*.example.com".
	if strings.HasPrefix(pattern, ".") {
		pattern = "*" + pattern
	}

	// No wildcard => exact match only.
	if !strings.Contains(pattern, WILDCARD) {
		return domain == pattern
	}

	// Special handling for patterns like "*.example.com" (and the ".example.com"
	// equivalent above): they should match the root domain and any subdomain.
	if strings.HasPrefix(pattern, "*.") && !strings.Contains(pattern[2:], WILDCARD) {
		root := pattern[2:]
		if domain == root {
			return true
		}
		if strings.HasSuffix(domain, "."+root) {
			return true
		}
		return false
	}

	// Fast path for suffix wildcard like "example.*"; matches base plus any TLD
	// but not the bare base and not subdomains of the base.
	if strings.HasSuffix(pattern, ".*") {
		base := strings.TrimSuffix(pattern, ".*")
		if base == "" || strings.Contains(base, WILDCARD) {
			return false
		}
		return strings.HasPrefix(domain, base+".")
	}

	// Fast path for contains wildcard like "*example*" when the only wildcards
	// are the leading and trailing asterisks.
	if strings.HasPrefix(pattern, WILDCARD) && strings.HasSuffix(pattern, WILDCARD) && strings.Count(pattern, WILDCARD) == 2 {
		needle := pattern[1 : len(pattern)-1]
		if needle == "" {
			return false
		}
		return strings.Contains(domain, needle)
	}

	// For all other wildcard patterns, fall back to regex-based matching.
	var re *regexp.Regexp
	if cached, ok := f.patternCache.Load(pattern); ok {
		re = cached.(*regexp.Regexp)
	} else {
		regexPattern := "^" + strings.ReplaceAll(regexp.QuoteMeta(pattern), "\\*", ".*") + "$"
		compiled, err := regexp.Compile(regexPattern)
		if err != nil {
			log.Error().Err(err).Str("pattern", pattern).Msg("Error compiling pattern")
			return false
		}
		f.patternCache.Store(pattern, compiled)
		re = compiled
	}
	return re.MatchString(domain)
}

// filterCustomRules checks if the domain is allowed or blocked by custom rules; method is executed before the DNS request is sent.
func (f *DomainFilter) filterCustomRules(reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) (*model.StageResult, error) {
	defer sentry.Recover()
	customRuleHashes, err := f.Cache.GetCustomRulesHashes(context.Background(), reqCtx.ProfileId)
	if err != nil {
		return nil, err
	}

	question := dctx.Req.Question[0].Name
	fqdn, _ := strings.CutSuffix(question, ".")

	result := &model.StageResult{Decision: model.DecisionNone, Tier: TierCustomRules}
	allowMatched := false

	for _, customRuleHash := range customRuleHashes {
		hash, err := f.Cache.GetCustomRulesHash(context.Background(), customRuleHash)
		if err != nil {
			return nil, err
		}

		if f.matchDomain(fqdn, hash["value"]) {
			switch hash["action"] {
			case ACTION_BLOCK:
				reqCtx.Logger.Debug().
					Str("reason", REASON_CUSTOM_RULES).
					Str("pattern", hash["value"]).
					Str("protocol", string(dctx.Proto)).
					Str("qtype", dns.TypeToString[dctx.Req.Question[0].Qtype]).
					Str("domain", question).
					Msg("Domain blocked")
				result.Decision = model.DecisionBlock
				result.Reasons = append(result.Reasons, REASON_CUSTOM_RULES)
				return result, nil

			case ACTION_ALLOW:
				reqCtx.Logger.Debug().
					Str("reason", REASON_CUSTOM_RULES).
					Str("pattern", hash["value"]).
					Msgf("Allowing domain: %s", question)
				allowMatched = true
			}
		}
	}

	if allowMatched {
		result.Decision = model.DecisionAllow
		result.Reasons = append(result.Reasons, REASON_CUSTOM_RULES)
		return result, nil
	}

	return result, nil
}

// filterCustomRules checks if the IP address is allowed or blocked by custom rules; method is executed after the DNS request is sent.
func (f *IPFilter) filterCustomRules(reqCtx *requestcontext.RequestContext, dctx *proxy.DNSContext) (*model.StageResult, error) {
	defer sentry.Recover()

	customRuleHashes, err := f.Cache.GetCustomRulesHashes(context.Background(), reqCtx.ProfileId)
	if err != nil {
		return nil, err
	}

	result := &model.StageResult{Decision: model.DecisionNone, Tier: TierCustomRules}
	allowMatched := false
	blockMatched := false

	if dctx == nil || dctx.Res == nil {
		return result, nil
	}

	ips := extractIPsFromAnswer(dctx.Res.Answer)
	for _, customRuleHash := range customRuleHashes {
		hash, err := f.Cache.GetCustomRulesHash(context.Background(), customRuleHash)
		if err != nil {
			return nil, err
		}
		syntax, ok := hash["syntax"]
		if !ok || syntax == "" {
			log.Debug().Str("hash", customRuleHash).Msg("Old custom rule detected, syntax is empty")
			continue
		}

		switch {
		case strings.Contains(syntax, "ip"):
			for _, ip := range ips {
				allow, block := f.matchIPRule(ip, hash)
				allowMatched = allowMatched || allow
				blockMatched = blockMatched || block
			}
		case syntax == "asn":
			if f.ASNLookup == nil {
				continue
			}
			ruleASN, ok := parseCustomRuleASN(hash["value"])
			if !ok {
				log.Debug().Str("hash", customRuleHash).Str("value", hash["value"]).Msg("Invalid ASN custom rule value")
				continue
			}
			for _, ip := range ips {
				allow, block := f.matchASNRule(ip, ruleASN, hash["action"])
				allowMatched = allowMatched || allow
				blockMatched = blockMatched || block
			}
		default:
			continue
		}

	}

	if blockMatched {
		result.Decision = model.DecisionBlock
		result.Reasons = append(result.Reasons, REASON_CUSTOM_RULES)
		return result, nil
	}
	if allowMatched {
		result.Decision = model.DecisionAllow
		result.Reasons = append(result.Reasons, REASON_CUSTOM_RULES)
		return result, nil
	}

	return result, nil
}

func parseCustomRuleASN(value string) (uint, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, false
	}

	upper := strings.ToUpper(trimmed)
	if strings.HasPrefix(upper, "AS") {
		trimmed = strings.TrimSpace(trimmed[2:])
	}
	if trimmed == "" {
		return 0, false
	}

	parsed, err := strconv.ParseUint(trimmed, 10, 32)
	if err != nil || parsed == 0 {
		return 0, false
	}
	return uint(parsed), true
}

func (f *IPFilter) matchASNRule(ip net.IP, ruleASN uint, action string) (allow bool, block bool) {
	if ip == nil {
		return false, false
	}

	asn, err := f.ASNLookup.ASN(ip)
	if err != nil || asn == 0 {
		return false, false
	}
	if asn != ruleASN {
		return false, false
	}

	switch action {
	case ACTION_BLOCK:
		log.Debug().Str("reason", REASON_CUSTOM_RULES).Uint("asn", asn).Msg("Blocked ASN")
		return false, true
	case ACTION_ALLOW:
		log.Debug().Str("reason", REASON_CUSTOM_RULES).Uint("asn", asn).Msg("Allowing ASN")
		return true, false
	default:
		return false, false
	}
}

func (f *IPFilter) matchIPRule(ip net.IP, hash map[string]string) (allow bool, block bool) {
	if ip == nil {
		return false, false
	}
	if !ip.Equal(net.ParseIP(hash["value"])) {
		return false, false
	}

	switch hash["action"] {
	case ACTION_BLOCK:
		log.Debug().
			Str("reason", REASON_CUSTOM_RULES).
			Str("pattern", hash["value"]).
			Msgf("Blocked IP: %s", ip.String())
		return false, true
	case ACTION_ALLOW:
		log.Debug().
			Str("reason", REASON_CUSTOM_RULES).
			Str("pattern", hash["value"]).
			Msgf("Allowing IP: %s", ip.String())
		return true, false
	default:
		return false, false
	}
}
