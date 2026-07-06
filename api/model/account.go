package model

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

const (
	AuthMethodPassword = "password"
	AuthMethodPasskey  = "passkey"
)

var ErrEmptyPassword = errors.New("password cannot be empty")

// Account represents a modDNS Account
type Account struct {
	ID                  primitive.ObjectID `json:"id" bson:"_id"`
	Email               string             `json:"email,omitempty" bson:"email"`
	EmailVerified       bool               `json:"email_verified,omitempty" bson:"email_verified"`
	Tokens              []Token            `json:"-" bson:"tokens"`
	Password            *string            `json:"-" bson:"password,omitempty"`
	Profiles            []string           `json:"profiles" bson:"profiles"`
	ErrorReportsConsent bool               `json:"error_reports_consent" bson:"error_reports_consent"`
	MFA                 MFASettings        `json:"mfa" bson:"mfa"`
	AuthMethods         []string           `json:"auth_methods" bson:"-"`
	DeletionCode        string             `json:"-" bson:"deletion_code,omitempty"`
	DeletionCodeExpires *time.Time         `json:"-" bson:"deletion_code_expires,omitempty"`
}

// MarshalJSON renders Profiles and AuthMethods as empty JSON arrays ([]) instead
// of null when nil, so the API always returns lists for these fields.
func (a Account) MarshalJSON() ([]byte, error) {
	type alias Account
	x := alias(a)
	if x.Profiles == nil {
		x.Profiles = []string{}
	}
	if x.AuthMethods == nil {
		x.AuthMethods = []string{}
	}
	return json.Marshal(x)
}

// New creates a new Account
func NewAccount(email, password, accountId, profileId string) (*Account, error) {
	tokens := make([]Token, 0)

	accIdPrimitive, err := primitive.ObjectIDFromHex(accountId)
	if err != nil {
		return nil, err
	}

	acc := &Account{
		ID:                  accIdPrimitive,
		Email:               email,
		Profiles:            []string{profileId},
		EmailVerified:       false,
		Tokens:              tokens,
		ErrorReportsConsent: false,
		MFA: MFASettings{
			TotpSettings{
				Enabled: false,
			},
		},
	}

	if password != "" { // password provided explicitly
		if err = acc.SetPassword(password); err != nil {
			return nil, err
		}
	}
	// Passkey method is added later upon successful WebAuthn credential registration

	return acc, nil
}

// WebAuthnID implements webauthn.User
func (a *Account) WebAuthnID() []byte {
	return []byte(a.ID.Hex())
}

// WebAuthnName implements webauthn.User
func (a *Account) WebAuthnName() string {
	return a.Email
}

// WebAuthnDisplayName implements webauthn.User
func (a *Account) WebAuthnDisplayName() string {
	return a.Email
}

// WebAuthnCredentials implements webauthn.User
func (a *Account) WebAuthnCredentials() []webauthn.Credential {
	// webauthn User interface requires this method, but we don't use it here
	// []webauthn.Credentials is taken from the User model (webauthn package)
	return nil
}

// SetPassword hashes provided plaintext password and updates account password & auth methods.
// It does not persist changes; caller must invoke repository update.
func (a *Account) SetPassword(newPassword string) error {
	if strings.TrimSpace(newPassword) == "" { // avoid setting empty password
		return ErrEmptyPassword
	}
	bytes, err := bcrypt.GenerateFromPassword([]byte(newPassword), 14)
	if err != nil {
		return err
	}
	hash := string(bytes)
	a.Password = &hash
	return nil
}

// AccountUpdate represents account settings update
// RFC6902 JSON Patch format is used
type AccountUpdate struct {
	Operation string `json:"operation" validate:"required,oneof=remove add replace move copy test"`
	Path      string `json:"path" validate:"required,oneof=/password /error_reports_consent /profiles /email"`
	Value     any    `json:"value" validate:"required"`
}
