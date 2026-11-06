package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Credential represents a WebAuthn credential stored in the database
type Credential struct {
	ID                 primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	CredentialID       []byte             `json:"-" bson:"credential_id"`
	CredentialIDString string             `json:"-" bson:"credential_id_string"`
	PublicKey          []byte             `json:"-" bson:"public_key"`
	AttestationType    string             `json:"-" bson:"attestation_type"`
	Transport          []string           `json:"-" bson:"transport"`
	Flags              CredentialFlags    `json:"-" bson:"flags"`
	Authenticator      Authenticator      `json:"-" bson:"authenticator"`
	AccountID          primitive.ObjectID `json:"-" bson:"account_id"`
	CreatedAt          time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt          time.Time          `json:"-" bson:"updated_at"`
	Name               string             `json:"-" bson:"name,omitempty"`
}

// CredentialFlags represents the flags of a WebAuthn credential
type CredentialFlags struct {
	UserPresent    bool `json:"user_present" bson:"user_present"`
	UserVerified   bool `json:"user_verified" bson:"user_verified"`
	BackupEligible bool `json:"backup_eligible" bson:"backup_eligible"`
	BackupState    bool `json:"backup_state" bson:"backup_state"`
}

// Authenticator represents the authenticator that created the credential
type Authenticator struct {
	AAGUID       []byte `json:"aaguid" bson:"aaguid"`
	SignCount    uint32 `json:"sign_count" bson:"sign_count"`
	CloneWarning bool   `json:"clone_warning" bson:"clone_warning"`
	Attachment   string `json:"attachment" bson:"attachment"`
}
