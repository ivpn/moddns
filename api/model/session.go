package model

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

type Session struct {
	AccountID   string               `bson:"account_id" json:"-"`
	Token       string               `bson:"token" json:"token"`
	Data        []byte               `bson:"data" json:"-"`
	SessionData webauthn.SessionData `bson:"-" json:"-"`
	Purpose     string               `bson:"purpose,omitempty" json:"-"`
	SubID       string               `bson:"sub_id,omitempty" json:"-"`
	// ExpiresAt is used as TTL index in mongoDB
	LastModified time.Time `bson:"last_modified" json:"-"`
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
	var data webauthn.SessionData
	if err := json.Unmarshal(s.Data, &data); err != nil {
		return err
	}

	s.SessionData = data
	return nil
}
