// Package totpcrypt protects stored TOTP material with a key held outside the
// database: the TOTP secret is encrypted (it must be recoverable to verify
// codes), backup codes are keyed hashes (they only need to be matched).
package totpcrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// KeySize is the length of the master key in bytes.
const KeySize = 32

const version = "v1"

var (
	ErrKeySize    = fmt.Errorf("totp storage key must be %d bytes, standard base64", KeySize)
	ErrMalformed  = errors.New("totp storage: malformed sealed value")
	ErrUnknownKey = errors.New("totp storage: value sealed under another key")
	ErrDecrypt    = errors.New("totp storage: decryption failed")
)

// Domain-separation labels: a value sealed for one purpose never opens for another.
const (
	purposeSecret  = "moddns/totp-secret" //nolint:gosec // G101 - label, not a credential
	purposePending = "moddns/totp-pending"
	purposeBackup  = "moddns/totp-backup-code"
)

// Keeper seals TOTP secrets and hashes backup codes. Safe for concurrent use.
type Keeper struct {
	aead   cipher.AEAD
	macKey []byte
	kid    string
}

// ParseKey decodes a standard-base64 master key and checks its length.
func ParseKey(b64 string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil || len(key) != KeySize {
		return nil, ErrKeySize
	}
	return key, nil
}

// New derives independent encryption, MAC and key-ID material from the master
// key with HKDF-SHA256 (RFC 5869).
func New(masterKey []byte) (*Keeper, error) {
	if len(masterKey) != KeySize {
		return nil, ErrKeySize
	}
	encKey, err := hkdf.Key(sha256.New, masterKey, nil, "moddns totp v1 encryption", 32)
	if err != nil {
		return nil, err
	}
	macKey, err := hkdf.Key(sha256.New, masterKey, nil, "moddns totp v1 backup-code mac", 32)
	if err != nil {
		return nil, err
	}
	kidBytes, err := hkdf.Key(sha256.New, masterKey, nil, "moddns totp v1 key id", 4)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Keeper{aead: aead, macKey: macKey, kid: hex.EncodeToString(kidBytes)}, nil
}

// IsSealed reports whether v has the sealed-value shape. A base32 TOTP secret
// never contains '.', so legacy plaintext is never mistaken for a sealed value.
func IsSealed(v string) bool {
	return strings.HasPrefix(v, version+".")
}

// SealSecret encrypts a TOTP secret for storage on the given account.
func (k *Keeper) SealSecret(accountID, secret string) (string, error) {
	return k.seal(purposeSecret, accountID, secret)
}

// OpenSecret decrypts a value produced by SealSecret for the same account.
func (k *Keeper) OpenSecret(accountID, sealed string) (string, error) {
	return k.open(purposeSecret, accountID, sealed)
}

// SealPendingSecret encrypts a TOTP secret awaiting enrolment confirmation.
func (k *Keeper) SealPendingSecret(accountID, secret string) (string, error) {
	return k.seal(purposePending, accountID, secret)
}

// OpenPendingSecret decrypts a value produced by SealPendingSecret.
func (k *Keeper) OpenPendingSecret(accountID, sealed string) (string, error) {
	return k.open(purposePending, accountID, sealed)
}

// HashBackupCode returns the stored form of a backup code: HMAC-SHA256 under
// the derived MAC key, bound to the account.
func (k *Keeper) HashBackupCode(accountID, code string) string {
	mac := hmac.New(sha256.New, k.macKey)
	mac.Write(label(purposeBackup, accountID))
	mac.Write([]byte(code))
	return k.format(mac.Sum(nil))
}

func (k *Keeper) seal(purpose, accountID, plaintext string) (string, error) {
	nonce := make([]byte, k.aead.NonceSize(), k.aead.NonceSize()+len(plaintext)+k.aead.Overhead())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := k.aead.Seal(nonce, nonce, []byte(plaintext), label(purpose, accountID))
	return k.format(out), nil
}

func (k *Keeper) open(purpose, accountID, sealed string) (string, error) {
	parts := strings.Split(sealed, ".")
	if len(parts) != 3 || parts[0] != version {
		return "", ErrMalformed
	}
	if parts[1] != k.kid {
		return "", ErrUnknownKey
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(raw) < k.aead.NonceSize()+k.aead.Overhead() {
		return "", ErrMalformed
	}
	nonce, ct := raw[:k.aead.NonceSize()], raw[k.aead.NonceSize():]
	pt, err := k.aead.Open(nil, nonce, ct, label(purpose, accountID))
	if err != nil {
		return "", ErrDecrypt
	}
	return string(pt), nil
}

func (k *Keeper) format(b []byte) string {
	return version + "." + k.kid + "." + base64.RawURLEncoding.EncodeToString(b)
}

func label(purpose, accountID string) []byte {
	return []byte(purpose + "\x00" + accountID + "\x00")
}
