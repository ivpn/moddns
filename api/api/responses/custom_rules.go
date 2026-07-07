package responses

import "encoding/json"

// CreateProfileCustomRulesBatchResponse represents the payload returned after attempting to create
// a batch of custom rules for a profile.
type CreateProfileCustomRulesBatchResponse struct {
	Action         string                   `json:"action"`
	TotalRequested int                      `json:"total_requested"`
	Created        []CustomRuleBatchCreated `json:"created"`
	Skipped        []CustomRuleBatchSkipped `json:"skipped"`
}

// MarshalJSON renders Created and Skipped as empty JSON arrays ([]) instead of
// null when nil, so the API always returns lists for these fields.
func (r CreateProfileCustomRulesBatchResponse) MarshalJSON() ([]byte, error) {
	type alias CreateProfileCustomRulesBatchResponse
	a := alias(r)
	if a.Created == nil {
		a.Created = []CustomRuleBatchCreated{}
	}
	if a.Skipped == nil {
		a.Skipped = []CustomRuleBatchSkipped{}
	}
	return json.Marshal(a)
}

// CustomRuleBatchCreated holds information about a successfully created rule within a batch request.
type CustomRuleBatchCreated struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

// CustomRuleBatchSkipped contains metadata about a rule that was not created within a batch request.
type CustomRuleBatchSkipped struct {
	Value   string `json:"value"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}
