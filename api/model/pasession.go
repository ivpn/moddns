package model

// PASession represents a pre-auth session stored in Redis during the ZLA signup flow.
// It is created by the preauth service and consumed during account registration.
type PASession struct {
	ID        string `json:"id"`
	Token     string `json:"token"`
	PreauthID string `json:"preauth_id"`
}
