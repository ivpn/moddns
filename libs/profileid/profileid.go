// Package profileid holds the profile ID format shared by the API, which
// issues profile IDs, and the proxy, which accepts them.
package profileid

const (
	// Length is the length of every profile ID the API issues.
	Length = 10
	// Alphabet is the character set of issued profile IDs. Lowercase only:
	// DNS names compare case-insensitively (RFC 4343) and the ID travels as
	// a DNS label in DoT/DoQ.
	Alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
)
