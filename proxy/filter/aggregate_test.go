package filter

import (
	"testing"

	"github.com/ivpn/dns/proxy/model"
	"github.com/stretchr/testify/assert"
)

func TestGetFinalFilteringResult(t *testing.T) {
	tests := []struct {
		name          string
		stageResults  []model.StageResult
		expectStatus  model.Status
		expectReasons []string
	}{
		{
			name:          "No decisions => processed",
			stageResults:  []model.StageResult{{Decision: model.DecisionNone, Tier: TierBlocklists}},
			expectStatus:  model.StatusProcessed,
			expectReasons: nil,
		},
		{
			name:          "Block => blocked (reasons deduped+sorted)",
			stageResults:  []model.StageResult{{Decision: model.DecisionBlock, Tier: TierBlocklists, Reasons: []string{"b", "a", "b", ""}}},
			expectStatus:  model.StatusBlocked,
			expectReasons: []string{"a", "b"},
		},
		{
			name:          "Allow => processed",
			stageResults:  []model.StageResult{{Decision: model.DecisionAllow, Tier: TierCustomRules, Reasons: []string{"custom_rules"}}},
			expectStatus:  model.StatusProcessed,
			expectReasons: []string{"custom_rules"},
		},
		{
			name: "Allow beats block regardless of tier",
			stageResults: []model.StageResult{
				{Decision: model.DecisionBlock, Tier: TierCustomRules, Reasons: []string{"block"}},
				{Decision: model.DecisionAllow, Tier: TierDefaultRule, Reasons: []string{"allow"}},
			},
			expectStatus:  model.StatusProcessed,
			expectReasons: []string{"allow"},
		},
		{
			name: "Highest allow tier wins + merges same-tier reasons",
			stageResults: []model.StageResult{
				{Decision: model.DecisionAllow, Tier: 10, Reasons: []string{"low"}},
				{Decision: model.DecisionAllow, Tier: 20, Reasons: []string{"b"}},
				{Decision: model.DecisionAllow, Tier: 20, Reasons: []string{"a"}},
			},
			expectStatus:  model.StatusProcessed,
			expectReasons: []string{"a", "b"},
		},
		{
			name: "Highest block tier wins + merges same-tier reasons",
			stageResults: []model.StageResult{
				{Decision: model.DecisionBlock, Tier: 10, Reasons: []string{"low"}},
				{Decision: model.DecisionBlock, Tier: 20, Reasons: []string{"b"}},
				{Decision: model.DecisionBlock, Tier: 20, Reasons: []string{"a"}},
			},
			expectStatus:  model.StatusBlocked,
			expectReasons: []string{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getFinalFilteringResult(tt.stageResults)
			assert.Equal(t, tt.expectStatus, got.Status)
			assert.Equal(t, tt.expectReasons, got.Reasons)
		})
	}
}

func TestGetFinalFilteringResult_IsOrderIndependent(t *testing.T) {
	stageResultsA := []model.StageResult{
		{Decision: model.DecisionBlock, Tier: TierBlocklists, Reasons: []string{"blocklists"}},
		{Decision: model.DecisionAllow, Tier: TierCustomRules, Reasons: []string{"custom_rules"}},
		{Decision: model.DecisionBlock, Tier: TierDefaultRule, Reasons: []string{"default_rule"}},
	}
	stageResultsB := []model.StageResult{
		stageResultsA[2],
		stageResultsA[0],
		stageResultsA[1],
	}

	gotA := getFinalFilteringResult(stageResultsA)
	gotB := getFinalFilteringResult(stageResultsB)
	assert.Equal(t, gotA, gotB)
}
