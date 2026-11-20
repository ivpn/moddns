package repository

import (
	"context"

	"github.com/ivpn/dns/api/model"
)

// SessionRepository represents a Session repository
type SessionRepository interface {
	GetSession(ctx context.Context, token string) (model.Session, bool, error)
	SaveSession(ctx context.Context, sessionData model.SessionPayload, token string, userID string) error
	DeleteSession(ctx context.Context, token string) error
	DeleteSessionsByAccountID(ctx context.Context, accID string) error
	DeleteSessionsByAccountIDExceptCurrent(ctx context.Context, accID, currentToken string) error
	CountSessionsByAccountID(ctx context.Context, accID string) (int64, error)
}
