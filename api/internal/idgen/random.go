package idgen

import (
	"crypto/rand"

	"github.com/ivpn/dns/libs/profileid"
)

// Bytes at or above the largest multiple of len(profileid.Alphabet) below 256
// are discarded so every character is equally likely.
const randomIDByteLimit = 256 - 256%len(profileid.Alphabet)

// RandomGenerator generates profile IDs of profileid.Length uniformly random
// characters from profileid.Alphabet using crypto/rand.
type RandomGenerator struct{}

func NewRandomGenerator() *RandomGenerator {
	return &RandomGenerator{}
}

func (g *RandomGenerator) Generate() (string, error) {
	id := make([]byte, 0, profileid.Length)
	buf := make([]byte, profileid.Length*2)
	for len(id) < profileid.Length {
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		for _, b := range buf {
			if int(b) >= randomIDByteLimit {
				continue
			}
			id = append(id, profileid.Alphabet[int(b)%len(profileid.Alphabet)])
			if len(id) == profileid.Length {
				break
			}
		}
	}
	return string(id), nil
}
