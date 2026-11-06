package webauthn

import (
	"encoding/base64"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	gowebauthn "github.com/go-webauthn/webauthn/webauthn"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/ivpn/dns/api/model"
)

// User is the account a ceremony runs for, with the credentials it already holds.
type User struct {
	AccountID primitive.ObjectID
	// Handle is the WebAuthn user handle (user.id), fixed at registration.
	Handle      []byte
	Name        string
	DisplayName string
	Credentials []model.Credential
}

// libUser adapts User to the library's user interface.
type libUser struct {
	user        User
	credentials []gowebauthn.Credential
}

func newLibUser(u User) *libUser {
	creds := make([]gowebauthn.Credential, len(u.Credentials))
	for i := range u.Credentials {
		creds[i] = toLibCredential(&u.Credentials[i])
	}
	return &libUser{user: u, credentials: creds}
}

func (u *libUser) WebAuthnID() []byte                           { return u.user.Handle }
func (u *libUser) WebAuthnName() string                         { return u.user.Name }
func (u *libUser) WebAuthnDisplayName() string                  { return u.user.DisplayName }
func (u *libUser) WebAuthnCredentials() []gowebauthn.Credential { return u.credentials }

func toLibCredential(c *model.Credential) gowebauthn.Credential {
	transports := make([]protocol.AuthenticatorTransport, len(c.Transport))
	for i, t := range c.Transport {
		transports[i] = protocol.AuthenticatorTransport(t)
	}

	return gowebauthn.Credential{
		ID:              c.CredentialID,
		PublicKey:       c.PublicKey,
		AttestationType: c.AttestationType,
		Transport:       transports,
		Flags: gowebauthn.CredentialFlags{
			UserPresent:    c.Flags.UserPresent,
			UserVerified:   c.Flags.UserVerified,
			BackupEligible: c.Flags.BackupEligible,
			BackupState:    c.Flags.BackupState,
		},
		Authenticator: gowebauthn.Authenticator{
			AAGUID:       c.Authenticator.AAGUID,
			SignCount:    c.Authenticator.SignCount,
			CloneWarning: c.Authenticator.CloneWarning,
			Attachment:   protocol.AuthenticatorAttachment(c.Authenticator.Attachment),
		},
	}
}

// toRecord builds the stored record of a credential owned by accountID.
func toRecord(cred *gowebauthn.Credential, accountID primitive.ObjectID) *model.Credential {
	transports := make([]string, len(cred.Transport))
	for i, t := range cred.Transport {
		transports[i] = string(t)
	}

	now := time.Now()
	return &model.Credential{
		ID:                 primitive.NewObjectID(),
		CredentialID:       cred.ID,
		CredentialIDString: base64.StdEncoding.EncodeToString(cred.ID),
		PublicKey:          cred.PublicKey,
		AttestationType:    cred.AttestationType,
		Transport:          transports,
		Flags: model.CredentialFlags{
			UserPresent:    cred.Flags.UserPresent,
			UserVerified:   cred.Flags.UserVerified,
			BackupEligible: cred.Flags.BackupEligible,
			BackupState:    cred.Flags.BackupState,
		},
		Authenticator: model.Authenticator{
			AAGUID:       cred.Authenticator.AAGUID,
			SignCount:    cred.Authenticator.SignCount,
			CloneWarning: cred.Authenticator.CloneWarning,
			Attachment:   string(cred.Authenticator.Attachment),
		},
		AccountID: accountID,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
