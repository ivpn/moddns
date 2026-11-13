package idgen

import "crypto/rand"

const (
	// RandomIDLength is the length of every generated profile ID.
	RandomIDLength = 10
	// RandomIDAlphabet is lowercase only: DNS names compare case-insensitively
	// (RFC 4343) and the ID travels as a DNS label in DoT/DoQ.
	RandomIDAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	// Bytes at or above the largest multiple of len(RandomIDAlphabet) below 256
	// are discarded so every character is equally likely.
	randomIDByteLimit = 256 - 256%len(RandomIDAlphabet)
)

// RandomGenerator generates RandomIDLength-character IDs of uniformly random
// characters from RandomIDAlphabet using crypto/rand.
type RandomGenerator struct{}

func NewRandomGenerator() *RandomGenerator {
	return &RandomGenerator{}
}

func (g *RandomGenerator) Generate() (string, error) {
	id := make([]byte, 0, RandomIDLength)
	buf := make([]byte, RandomIDLength*2)
	for len(id) < RandomIDLength {
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		for _, b := range buf {
			if int(b) >= randomIDByteLimit {
				continue
			}
			id = append(id, RandomIDAlphabet[int(b)%len(RandomIDAlphabet)])
			if len(id) == RandomIDLength {
				break
			}
		}
	}
	return string(id), nil
}
