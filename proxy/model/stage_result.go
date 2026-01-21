package model

// Decision represents the outcome of a single filtering stage.
// It is used for deterministic aggregation across concurrently executed stages.
type Decision string

const (
	DecisionNone  Decision = "none"
	DecisionAllow Decision = "allow"
	DecisionBlock Decision = "block"
)

// StageResult is a richer intermediate result produced by a single filtering stage.
// Tier determines precedence: higher tier wins. Allow always wins over Block within
// the same tier.
type StageResult struct {
	Decision Decision `json:"decision"`
	Tier     int      `json:"tier"`
	Reasons  []string `json:"reasons"`
}
