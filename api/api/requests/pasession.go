package requests

// PASessionReq represents the request body for adding a pre-auth session.
type PASessionReq struct {
	ID        string `json:"id" validate:"required,uuid4"`
	PreauthID string `json:"preauth_id" validate:"required,uuid4"`
	Token     string `json:"token" validate:"required"`
}

// RotatePASessionReq represents the request body for rotating a pre-auth session ID.
type RotatePASessionReq struct {
	SessionID string `json:"sessionid" validate:"required,uuid4"`
}
