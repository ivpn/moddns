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

	// New rules append to the end of the existing display order. The base is the
	// current rule count; each created rule takes the next dense index.
	baseOrder := len(profile.Settings.CustomRules)

	for _, original := range values {
		normalized := normalizeRuleValue(profile.Settings, original)
		if normalized == "" {
			result.Skipped = append(result.Skipped, BulkCustomRuleSkipped{
				Value:   original,
				Reason:  BulkCustomRuleSkipReasonInvalidSyntax,
				Message: invalidSyntaxMessage,
			})
			continue
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
			Order:  baseOrder + len(toCreate),
		}

		toCreate = append(toCreate, customRule)
		existingValues[normalized] = struct{}{}
	}

	if len(profile.Settings.CustomRules)+len(toCreate) > model.MaxCustomRulesPerProfile {
		return nil, ErrMaxCustomRulesReached
	}

	if len(toCreate) > 0 {
		if err := p.ProfileRepository.CreateCustomRules(ctx, profileId, toCreate); err != nil {
			return nil, err
		}

		if err := p.Cache.AddCustomRules(ctx, profileId, toCreate); err != nil {
			return nil, err
		}

		result.Created = append(result.Created, toCreate...)
	}

	return result, nil
}

// normalizeRuleValue applies the canonical custom-rule normalization pipeline to a
// raw value: trim, strip trailing dot, ".x" -> "*.x", ASN canonicalization, and
// (unless the profile uses the "exact" subdomain rule) auto-prepend "*." to plain
// FQDNs so subdomains are included. It does NOT validate syntax — callers derive
// syntax via model.NewCustomRuleSyntax. Returns "" for empty input.
//
// Shared by CreateCustomRulesBulk and UpdateCustomRule so the create and edit
// paths never drift.
func normalizeRuleValue(settings *model.ProfileSettings, original string) string {
	trimmed := strings.TrimSpace(original)
	if trimmed == "" {
		return ""
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

	// When custom_rules_subdomains_rule is "include" (or empty/unset for backwards compat),
	// auto-prepend "*." to plain FQDN values so subdomains are included.
	// Skip values that already express subdomain/non-FQDN semantics:
	//   - wildcards (already contain "*")
	//   - dot-prefix (".facebook.com" was already normalized to "*.facebook.com" above)
	//   - IPs (v4/v6)
	//   - CIDRs ("1.2.3.0/24", "2001:db8::/32" — contain "/")
	//   - ASNs ("15169")
	if settings.Privacy.CustomRulesSubdomainsRule != model.CUSTOM_RULES_SUBDOMAINS_EXACT {
		if !strings.Contains(normalized, "*") && !strings.Contains(normalized, "/") &&
			net.ParseIP(normalized) == nil {
			if _, isASN := normalizeASN(normalized); !isASN {
				normalized = "*." + normalized
			}
		}
	}

	return normalized
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

// CustomRulePatch carries the partial-update fields for UpdateCustomRule. A nil
// pointer means "leave unchanged"; a non-nil pointer (including an empty string)
// is applied. This lets callers explicitly clear note/group.
type CustomRulePatch struct {
	Action *string
	Value  *string
	Note   *string
	Group  *string
	Order  *int
}

// UpdateCustomRule applies a partial update to a single rule in place, preserving
// its stable ObjectID. When the value changes it is re-normalized, its syntax is
// re-derived, and it is dup-checked against the profile's OTHER rules. Redis is
// re-synced only when a proxy-relevant field (action/value/syntax) changes; pure
// note/group/order edits do no cache work. Returns the updated rule.
func (p *ProfileService) UpdateCustomRule(ctx context.Context, accountId, profileId, customRuleId string, patch CustomRulePatch) (*model.CustomRule, error) {
	profile, err := p.validateProfileIdAffiliation(ctx, accountId, profileId)
	if err != nil {
		return nil, err
	}

	var existing *model.CustomRule
	for _, r := range profile.Settings.CustomRules {
		if r.ID.Hex() == customRuleId {
			existing = r
			break
		}
	}
	if existing == nil {
		return nil, dbErrors.ErrCustomRuleNotFound
	}

	updated := *existing
	proxyFieldsChanged := false

	if patch.Action != nil {
		action, err := model.NewCustomRuleAction(*patch.Action)
		if err != nil {
			return nil, err
		}
		if action != updated.Action {
			updated.Action = action
			proxyFieldsChanged = true
		}
	}

	if patch.Value != nil {
		normalized := normalizeRuleValue(profile.Settings, *patch.Value)
		if normalized == "" {
			return nil, model.ErrInvalidCustomRuleSyntax
		}
		syntax, err := model.NewCustomRuleSyntax(p.Validate, normalized)
		if err != nil {
			return nil, model.ErrInvalidCustomRuleSyntax
		}
		if normalized != updated.Value {
			// Duplicate check excludes the rule being edited.
			for _, r := range profile.Settings.CustomRules {
				if r.ID.Hex() != customRuleId && r.Value == normalized {
					return nil, ErrCustomRuleAlreadyExists
				}
			}
			updated.Value = normalized
			updated.Syntax = syntax
			proxyFieldsChanged = true
		}
	}

	if patch.Note != nil {
		updated.Note = *patch.Note
	}
	if patch.Group != nil {
		updated.Group = *patch.Group
	}
	if patch.Order != nil {
		updated.Order = *patch.Order
	}

	if err := p.ProfileRepository.UpdateCustomRule(ctx, profileId, &updated); err != nil {
		return nil, err
	}

	// Re-sync the proxy only when a field the proxy reads actually changed.
	// AddCustomRules HSets the rule hash (overwriting fields) and is idempotent
	// on the set membership, so a single-element slice re-syncs the edited rule.
	if proxyFieldsChanged {
		if err := p.Cache.AddCustomRules(ctx, profileId, []*model.CustomRule{&updated}); err != nil {
			return nil, err
		}
	}

	return &updated, nil
}

// ReorderCustomRules sets the display order of the profile's rules to match the
// position of each ID in orderedIds (index 0 first). Every ID must belong to the
// profile. Order is organizational only and is never synced to the proxy.
//
// Callers should send the complete ordered ID list for the profile; IDs omitted
// from orderedIds keep their stored order, which can collide with the renumbered
// ones, so partial lists are discouraged.
func (p *ProfileService) ReorderCustomRules(ctx context.Context, accountId, profileId string, orderedIds []string) error {
	profile, err := p.validateProfileIdAffiliation(ctx, accountId, profileId)
	if err != nil {
		return err
	}

	known := make(map[string]struct{}, len(profile.Settings.CustomRules))
	for _, r := range profile.Settings.CustomRules {
		known[r.ID.Hex()] = struct{}{}
	}

	idToOrder := make(map[string]int, len(orderedIds))
	for i, id := range orderedIds {
		if _, ok := known[id]; !ok {
			return dbErrors.ErrCustomRuleNotFound
		}
		if _, dup := idToOrder[id]; dup {
			continue
		}
		idToOrder[id] = i
	}

	return p.ProfileRepository.UpdateCustomRulesOrder(ctx, profileId, idToOrder)
}

// SetCustomRuleGroups upserts group-note entries. A nil note value deletes that
// group's note. The map is merged onto the profile's current group-note map and
// persisted wholesale. Group notes are metadata only and never reach the proxy.
// maxGroupNameLen bounds a decoded group name. Mirrors the frontend/edit limit.
const maxGroupNameLen = 64

// CustomRuleGroupOp is one JSON-Patch-style operation on the per-list group
// registry. The handler decodes the request's JSON-Pointer path/from into the
// plain Group/From names before calling the service. Action ("block"/"allow")
// scopes the op to one list, so the denylist and allowlist have independent groups.
//   - "add"/"replace": set Group's note to Note (creates the group).
//   - "remove":        delete Group (member rules → Ungrouped, note dropped).
//   - "move":          rename From → Group (reassign member rules, move the note).
type CustomRuleGroupOp struct {
	Operation string
	Action    string
	Group     string
	From      string
	Note      *string
}

// ApplyCustomRuleGroupOps applies a batch of per-list group-registry operations in
// order. Member-rule reassignments (remove/move) run as atomic, action-scoped bulk
// updates; the nested note map is folded in memory and persisted once at the end.
// Group labels are metadata only — no Redis writes.
func (p *ProfileService) ApplyCustomRuleGroupOps(ctx context.Context, accountId, profileId string, ops []CustomRuleGroupOp) error {
	profile, err := p.validateProfileIdAffiliation(ctx, accountId, profileId)
	if err != nil {
		return err
	}

	groups := profile.Settings.CustomRuleGroups.Clone()

	for _, op := range ops {
		if op.Action != model.ACTION_BLOCK && op.Action != model.ACTION_ALLOW {
			return model.ErrInvalidCustomRuleAction
		}

		switch op.Operation {
		case "add", "replace":
			if op.Group == "" || len(op.Group) > maxGroupNameLen {
				return model.ErrInvalidCustomRuleSyntax
			}
			comment := ""
			if op.Note != nil {
				comment = *op.Note
			}
			groups.Upsert(op.Action, op.Group, comment)

		case "remove":
			if op.Group == "" {
				return model.ErrInvalidCustomRuleSyntax
			}
			if err := p.ProfileRepository.ReassignCustomRuleGroup(ctx, profileId, op.Action, op.Group, ""); err != nil {
				return err
			}
			groups.Remove(op.Action, op.Group)

		case "move":
			if op.From == "" || op.Group == "" || len(op.Group) > maxGroupNameLen {
				return model.ErrInvalidCustomRuleSyntax
			}
			if op.From == op.Group {
				continue
			}
			if err := p.ProfileRepository.ReassignCustomRuleGroup(ctx, profileId, op.Action, op.From, op.Group); err != nil {
				return err
			}
			groups.Rename(op.Action, op.From, op.Group)

		default:
			return model.ErrInvalidCustomRuleSyntax
		}
	}

	return p.ProfileRepository.SetCustomRuleGroups(ctx, profileId, groups)
}
