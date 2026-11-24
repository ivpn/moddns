package totpcrypt

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	accountA = "507f1f77bcf86cd799439011"
	accountB = "507f1f77bcf86cd799439012"
	secret   = "JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP"
)

func testKeeper(t *testing.T, fill byte) *Keeper {
	t.Helper()
	k, err := New(bytes.Repeat([]byte{fill}, KeySize))
	require.NoError(t, err)
	return k
}

// specRef: api-endpoint-behaviour.md E4
func TestSealOpenSecretRoundTrip(t *testing.T) {
	k := testKeeper(t, 1)
	sealed, err := k.SealSecret(accountA, secret)
	require.NoError(t, err)
	require.NotContains(t, sealed, secret)
	require.True(t, IsSealed(sealed))

	got, err := k.OpenSecret(accountA, sealed)
	require.NoError(t, err)
	require.Equal(t, secret, got)
}

func TestSealUsesFreshNonce(t *testing.T) {
	k := testKeeper(t, 1)
	a, err := k.SealSecret(accountA, secret)
	require.NoError(t, err)
	b, err := k.SealSecret(accountA, secret)
	require.NoError(t, err)
	require.NotEqual(t, a, b)
}

// specRef: api-endpoint-behaviour.md E4
func TestOpenSecretBoundToAccount(t *testing.T) {
	k := testKeeper(t, 1)
	sealed, err := k.SealSecret(accountA, secret)
	require.NoError(t, err)
	_, err = k.OpenSecret(accountB, sealed)
	require.ErrorIs(t, err, ErrDecrypt)
}

func TestPendingAndStoredSecretsAreNotInterchangeable(t *testing.T) {
	k := testKeeper(t, 1)
	pending, err := k.SealPendingSecret(accountA, secret)
	require.NoError(t, err)
	_, err = k.OpenSecret(accountA, pending)
	require.ErrorIs(t, err, ErrDecrypt)

	got, err := k.OpenPendingSecret(accountA, pending)
	require.NoError(t, err)
	require.Equal(t, secret, got)
}

func TestOpenSecretRejectsTampering(t *testing.T) {
	k := testKeeper(t, 1)
	sealed, err := k.SealSecret(accountA, secret)
	require.NoError(t, err)
	i := strings.LastIndexByte(sealed, '.')
	raw, err := base64.RawURLEncoding.DecodeString(sealed[i+1:])
	require.NoError(t, err)
	raw[len(raw)-1] ^= 0x01
	_, err = k.OpenSecret(accountA, sealed[:i+1]+base64.RawURLEncoding.EncodeToString(raw))
	require.ErrorIs(t, err, ErrDecrypt)
}

// specRef: api-endpoint-behaviour.md E10
func TestOpenSecretUnderOtherKeyFailsClosed(t *testing.T) {
	sealed, err := testKeeper(t, 1).SealSecret(accountA, secret)
	require.NoError(t, err)
	_, err = testKeeper(t, 2).OpenSecret(accountA, sealed)
	require.ErrorIs(t, err, ErrUnknownKey)
}

func TestOpenSecretRejectsMalformedInput(t *testing.T) {
	k := testKeeper(t, 1)
	for _, in := range []string{"", secret, "v1.", "v1.abcd", "v2.abcd.AAAA", "v1." + k.kid + ".!!!", "v1." + k.kid + ".AAAA"} {
		_, err := k.OpenSecret(accountA, in)
		require.Errorf(t, err, "input %q", in)
	}
}

func TestIsSealedNeverMatchesBase32Secret(t *testing.T) {
	require.False(t, IsSealed(secret))
	require.False(t, IsSealed(""))
}

// specRef: api-endpoint-behaviour.md E5
func TestHashBackupCode(t *testing.T) {
	k := testKeeper(t, 1)
	code := "AbCdEfGhJkMnPqRs"

	h := k.HashBackupCode(accountA, code)
	require.Equal(t, h, k.HashBackupCode(accountA, code), "deterministic")
	require.NotContains(t, h, code)
	require.True(t, strings.HasPrefix(h, "v1."+k.kid+"."))

	require.NotEqual(t, h, k.HashBackupCode(accountB, code), "bound to the account")
	require.NotEqual(t, h, k.HashBackupCode(accountA, "AbCdEfGhJkMnPqRt"))
	require.NotEqual(t, h, testKeeper(t, 2).HashBackupCode(accountA, code), "keyed")
}

// specRef: api-endpoint-behaviour.md E10
func TestParseKey(t *testing.T) {
	good := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, KeySize))
	key, err := ParseKey(good)
	require.NoError(t, err)
	require.Len(t, key, KeySize)

	for _, bad := range []string{"", "not base64!", base64.StdEncoding.EncodeToString(make([]byte, 16))} {
		_, err := ParseKey(bad)
		require.ErrorIsf(t, err, ErrKeySize, "input %q", bad)
	}
}

func TestNewRejectsWrongKeySize(t *testing.T) {
	_, err := New(make([]byte, 16))
	require.ErrorIs(t, err, ErrKeySize)
}
