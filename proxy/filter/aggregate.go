package filter

import (
	"slices"

	"github.com/ivpn/dns/proxy/model"
)

const (
	TierDefaultRule = 0
	TierBlocklists  = 100
	TierServices    = 100
	TierCustomRules = 200
)

func getFinalFilteringResult(stageResults []model.StageResult) model.FilterResult {
	bestAllowTier := -1
	bestBlockTier := -1
	allowReasons := make([]string, 0)
	blockReasons := make([]string, 0)

	for _, res := range stageResults {
		switch res.Decision {
		case model.DecisionAllow:
			if res.Tier > bestAllowTier {
				bestAllowTier = res.Tier
				allowReasons = append(allowReasons[:0], res.Reasons...)
			} else if res.Tier == bestAllowTier {
				allowReasons = append(allowReasons, res.Reasons...)
			}
		case model.DecisionBlock:
			if res.Tier > bestBlockTier {
				bestBlockTier = res.Tier
				blockReasons = append(blockReasons[:0], res.Reasons...)
			} else if res.Tier == bestBlockTier {
				blockReasons = append(blockReasons, res.Reasons...)
			}
		}
	}

	if bestAllowTier >= 0 {
		final := model.FilterResult{Status: model.StatusProcessed, Reasons: dedupeAndSort(allowReasons)}
		return final
	}
	if bestBlockTier >= 0 {
		final := model.FilterResult{Status: model.StatusBlocked, Reasons: dedupeAndSort(blockReasons)}
		return final
	}

	return model.FilterResult{Status: model.StatusProcessed}
}

func dedupeAndSort(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	// Copy to avoid surprising aliasing.
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, r := range in {
		if r == "" {
			continue
		}
		if _, ok := seen[r]; ok {
			continue
		}
		seen[r] = struct{}{}
		out = append(out, r)
	}
	if len(out) == 0 {
		return nil
	}
	slices.Sort(out)
	return out
}
