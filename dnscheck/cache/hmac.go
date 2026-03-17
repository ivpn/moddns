package cache

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// HMACKey computes an HMAC-SHA256 of subdomain using secret,
// returning a hex-encoded string suitable as a cache key.
func HMACKey(secret, subdomain string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(subdomain))
	return hex.EncodeToString(mac.Sum(nil))
}
