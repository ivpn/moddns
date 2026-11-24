package utils

import (
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
)

// AlphaNumericUserFriendly contains letters without those which looks similar to one another
const AlphaNumericUserFriendly = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789"

// AlphaNumericUserFriendlyUppercase is AlphaNumericUserFriendly without lowercase letters.
const AlphaNumericUserFriendlyUppercase = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

var (
	ErrInvalidLength = errors.New("random string: length must be positive")
	ErrEmptyCharset  = errors.New("random string: charset is empty")
)

// RandomString returns n characters drawn uniformly from charset with crypto/rand.
func RandomString(n int, charset string) (string, error) {
	if n <= 0 {
		return "", ErrInvalidLength
	}
	if charset == "" {
		return "", ErrEmptyCharset
	}

	sb := strings.Builder{}
	sb.Grow(n)
	maxIdx := big.NewInt(int64(len(charset)))
	for range n {
		idx, err := rand.Int(rand.Reader, maxIdx)
		if err != nil {
			return "", err
		}
		sb.WriteByte(charset[idx.Int64()])
	}
	return sb.String(), nil
}
