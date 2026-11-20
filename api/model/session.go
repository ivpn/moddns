package model

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"time"
)

type Session struct {
	AccountID   string         `bson:"account_id" json:"-"`
	Token       string         `bson:"token" json:"token"`
	Data        []byte         `bson:"data" json:"-"`
	SessionData SessionPayload `bson:"-" json:"-"`
	// ExpiresAt is used as TTL index in mongoDB
	LastModified time.Time `bson:"last_modified" json:"-"`
}

// SessionPayload is the JSON stored in a login session's data. Every instance reads
// stored sessions during a rolling deploy, so the field names must not change.
type SessionPayload struct {
	// Challenge is empty for login sessions; a stored document carrying one is
	// WebAuthn ceremony state, never a login session.
	Challenge string    `json:"challenge"`
	UserID    []byte    `json:"user_id"`
	Expires   time.Time `json:"expires"`
}

func GenSessionToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

func (s *Session) UnmarshalSessionData() error {
	var data SessionPayload
	if err := json.Unmarshal(s.Data, &data); err != nil {
		return err
	}

	s.SessionData = data
	return nil
}
