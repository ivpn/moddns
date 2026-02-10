package profile

import (
	"context"
	"net"
	"strconv"
	"strings"

	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/model"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type BulkCustomRuleSkipReason string

const (
	BulkCustomRuleSkipReasonInvalidSyntax     BulkCustomRuleSkipReason = "invalid_syntax"
	BulkCustomRuleSkipReasonDuplicateExisting BulkCustomRuleSkipReason = "duplicate_existing"
	BulkCustomRuleSkipReasonDuplicatePayload  BulkCustomRuleSkipReason = "duplicate_payload"
)

type BulkCustomRuleSkipped struct {
	Value   string
	Reason  BulkCustomRuleSkipReason
	Message string
}

type BulkCustomRuleResult struct {
	Action         model.CustomRuleAction
	TotalRequested int
	Created        []*model.CustomRule
	Skipped        []BulkCustomRuleSkipped
}

const (
	duplicateExistingMessage = "Rule already exists on this profile."
	duplicatePayloadMessage  = "Value appears more than once in this request."
	invalidSyntaxMessage     = "Value must be a valid domain, wildcard, IP address, or ASN."
)

// CreateCustomRule creates a new custom rule entry for a profile.
func (p *ProfileService) CreateCustomRule(ctx context.Context, accountId, profileId, action string, value string) error {
	result, err := p.CreateCustomRulesBulk(ctx, accountId, profileId, action, []string{value})
	if err != nil {
		return err
	}

	if len(result.Created) == 0 && len(result.Skipped) > 0 {
		switch result.Skipped[0].Reason {
		case BulkCustomRuleSkipReasonDuplicateExisting:
			return ErrCustomRuleAlreadyExists
		case BulkCustomRuleSkipReasonInvalidSyntax:
			return model.ErrInvalidCustomRuleSyntax
		}
	}

	return nil
}

// CreateCustomRulesBulk attempts to create multiple custom rules at once while returning
// detailed information about skipped entries.
func (p *ProfileService) CreateCustomRulesBulk(ctx context.Context, accountId, profileId, action string, values []string) (*BulkCustomRuleResult, error) {
	profile, err := p.validateProfileIdAffiliation(ctx, accountId, profileId)
	if err != nil {
		return nil, err
	}

	actionCustomRule, err := model.NewCustomRuleAction(action)
	if err != nil {
		return nil, err
	}

	result := &BulkCustomRuleResult{
		Action:         actionCustomRule,
		TotalRequested: len(values),
		Created:        make([]*model.CustomRule, 0),
		Skipped:        make([]BulkCustomRuleSkipped, 0),
	}

	if len(values) == 0 {
		return result, nil
	}

	existingValues := make(map[string]struct{}, len(profile.Settings.CustomRules))
	for _, rule := range profile.Settings.CustomRules {
		existingValues[rule.Value] = struct{}{}
	}

	payloadSeen := make(map[string]struct{}, len(values))
	toCreate := make([]*model.CustomRule, 0)

	for _, original := range values {
		trimmed := strings.TrimSpace(original)
		if trimmed == "" {
			result.Skipped = append(result.Skipped, BulkCustomRuleSkipped{
				Value:   original,
				Reason:  BulkCustomRuleSkipReasonInvalidSyntax,
				Message: invalidSyntaxMessage,
			})
			continue
		}

		normalized, _ := strings.CutSuffix(trimmed, ".")
		// Support ".example.com" syntax by normalizing to "*.example.com" for validation/storage
		if strings.HasPrefix(normalized, ".") {
			normalized = "*" + normalized
		}

		// Normalize ASN rules: allow both "AS15169" and "15169" inputs, store canonical digits only.
		if asnNormalized, ok := normalizeASN(normalized); ok {
			normalized = asnNormalized
		}

		// When custom_rules_subdomains is "include" (or empty/unset for backwards compat),
		// auto-prepend "*." to plain FQDN values so subdomains are included.
		if profile.Settings.Privacy.CustomRulesSubdomains != model.CUSTOM_RULES_SUBDOMAINS_EXACT {
			if !strings.Contains(normalized, "*") && net.ParseIP(normalized) == nil {
				if _, isASN := normalizeASN(normalized); !isASN {
					normalized = "*." + normalized
				}
			}
		}

		if _, exists := payloadSeen[normalized]; exists {
			result.Skipped = append(result.Skipped, BulkCustomRuleSkipped{
				Value:   normalized,
				Reason:  BulkCustomRuleSkipReasonDuplicatePayload,
				Message: duplicatePayloadMessage,
			})
			continue
		}

		payloadSeen[normalized] = struct{}{}

		syntax, err := model.NewCustomRuleSyntax(p.Validate, normalized)
		if err != nil {
			result.Skipped = append(result.Skipped, BulkCustomRuleSkipped{
				Value:   normalized,
				Reason:  BulkCustomRuleSkipReasonInvalidSyntax,
				Message: invalidSyntaxMessage,
			})
			continue
		}

		if _, exists := existingValues[normalized]; exists {
			result.Skipped = append(result.Skipped, BulkCustomRuleSkipped{
				Value:   normalized,
				Reason:  BulkCustomRuleSkipReasonDuplicateExisting,
				Message: duplicateExistingMessage,
			})
			continue
		}

		customRule := &model.CustomRule{
			ID:     primitive.NewObjectID(),
			Action: actionCustomRule,
			Value:  normalized,
			Syntax: syntax,
		}

		toCreate = append(toCreate, customRule)
		existingValues[normalized] = struct{}{}
	}

	if len(toCreate) > 0 {
		if err := p.ProfileRepository.CreateCustomRules(ctx, profileId, toCreate); err != nil {
			return nil, err
		}

		for _, rule := range toCreate {
			if err := p.Cache.AddCustomRule(ctx, profileId, rule); err != nil {
				return nil, err
			}
		}

		result.Created = append(result.Created, toCreate...)
	}

	return result, nil
}

func normalizeASN(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", false
	}

	upper := strings.ToUpper(trimmed)
	if strings.HasPrefix(upper, "AS") {
		trimmed = strings.TrimSpace(trimmed[2:])
	}
	if trimmed == "" {
		return "", false
	}

	parsed, err := strconv.ParseUint(trimmed, 10, 32)
	if err != nil || parsed == 0 {
		return "", false
	}
	return strconv.FormatUint(parsed, 10), true
}

// DeleteCustomRule removes a selected custom rule for the given profile.
func (p *ProfileService) DeleteCustomRule(ctx context.Context, accountId, profileId, customRuleId string) error {
	profile, err := p.validateProfileIdAffiliation(ctx, accountId, profileId)
	if err != nil {
		return err
	}
	var found bool
	for _, customRule := range profile.Settings.CustomRules {
		if customRule.ID.Hex() == customRuleId {
			found = true
			break
		}
	}
	if !found {
		return dbErrors.ErrCustomRuleNotFound
	}

	if err = p.ProfileRepository.RemoveCustomRules(ctx, profileId, []string{customRuleId}); err != nil {
		return err
	}

	// remove custom rule from cache
	if err = p.Cache.RemoveCustomRule(ctx, profileId, customRuleId); err != nil {
		return err
	}
	return nil
}
