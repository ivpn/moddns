package utils

import (
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRandomStringLength(t *testing.T) {
	result, err := RandomString(10, AlphaNumericUserFriendly)
	require.NoError(t, err)
	require.Len(t, result, 10)
}

func TestRandomStringCharacters(t *testing.T) {
	result, err := RandomString(64, AlphaNumericUserFriendly)
	require.NoError(t, err)
	for _, char := range result {
		require.Truef(t, strings.ContainsRune(AlphaNumericUserFriendly, char), "unexpected character %c", char)
	}
}

func TestRandomStringDifferentResults(t *testing.T) {
	a, err := RandomString(16, AlphaNumericUserFriendly)
	require.NoError(t, err)
	b, err := RandomString(16, AlphaNumericUserFriendly)
	require.NoError(t, err)
	require.NotEqual(t, a, b)
}

func TestRandomStringRejectsInvalidInput(t *testing.T) {
	_, err := RandomString(0, AlphaNumericUserFriendly)
	require.ErrorIs(t, err, ErrInvalidLength)
	_, err = RandomString(8, "")
	require.ErrorIs(t, err, ErrEmptyCharset)
}

// Every character of the alphabet must be drawn with equal probability; a
// modulo-biased or truncated index skews the counts far beyond this bound.
func TestRandomStringUniformDistribution(t *testing.T) {
	const perChar = 2000
	n := len(AlphaNumericUserFriendly) * perChar
	result, err := RandomString(n, AlphaNumericUserFriendly)
	require.NoError(t, err)

	counts := map[rune]int{}
	for _, c := range result {
		counts[c]++
	}
	require.Len(t, counts, len(AlphaNumericUserFriendly))
	for c, got := range counts {
		// ~7 standard deviations for a binomial with p = 1/56.
		require.InDeltaf(t, perChar, got, 310, "character %c drawn %d times", c, got)
	}
}

// Backup codes are generated from concurrent requests; run with -race.
func TestRandomStringConcurrentUse(t *testing.T) {
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				_, err := RandomString(16, AlphaNumericUserFriendly)
				require.NoError(t, err)
			}
		}()
	}
	wg.Wait()
}
