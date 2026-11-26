package account_test

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"

	"github.com/ivpn/dns/api/api/requests"
	"github.com/ivpn/dns/api/cache"
	"github.com/ivpn/dns/api/config"
	dbErrors "github.com/ivpn/dns/api/db/errors"
	"github.com/ivpn/dns/api/db/repository"
	"github.com/ivpn/dns/api/internal/auth"
	webhookClient "github.com/ivpn/dns/api/internal/client"
	"github.com/ivpn/dns/api/internal/reauth"
	"github.com/ivpn/dns/api/internal/totpcrypt"
	"github.com/ivpn/dns/api/internal/utils"
	"github.com/ivpn/dns/api/model"
	"github.com/ivpn/dns/api/service/account"
)

const legacySecret = "JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP"

// totpRepo is an in-memory account store with the same atomicity as the
// MongoDB implementation of the TOTP methods.
type totpRepo struct {
	repository.AccountRepository
	mu        sync.Mutex
	accounts  map[string]model.Account
	enableErr error
	writeErr  error // returned by DisableTOTP and ReplaceBackupCodes
}

func (r *totpRepo) GetAccountById(_ context.Context, id string) (*model.Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	acc, ok := r.accounts[id]
	if !ok {
		return nil, dbErrors.ErrAccountNotFound
	}
	t := acc.MFA.TOTP
	t.BackupCodeHashes = slices.Clone(t.BackupCodeHashes)
	t.BackupCodes = slices.Clone(t.BackupCodes)
	acc.MFA.TOTP = t
	return &acc, nil
}

func (r *totpRepo) EnableTOTP(_ context.Context, id, sealed string, hashes []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.enableErr != nil {
		return r.enableErr
	}
	acc := r.accounts[id]
	if acc.MFA.TOTP.Enabled {
		return dbErrors.ErrTOTPStateChanged
	}
	acc.MFA.TOTP = model.TotpSettings{Enabled: true, SecretSealed: sealed, BackupCodeHashes: slices.Clone(hashes)}
	r.accounts[id] = acc
	return nil
}

func (r *totpRepo) DisableTOTP(_ context.Context, id string, proof model.TOTPProof) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.writeErr != nil {
		return r.writeErr
	}
	acc := r.accounts[id]
	if !matchesProof(acc.MFA.TOTP, proof) {
		return dbErrors.ErrTOTPStateChanged
	}
	acc.MFA.TOTP = model.TotpSettings{Enabled: false}
	r.accounts[id] = acc
	return nil
}

func (r *totpRepo) GetAccountByEmail(context.Context, string) (*model.Account, error) {
	return nil, dbErrors.ErrAccountNotFound
}

// UpdateAccount stores everything but the 2FA state, like the MongoDB
// implementation (E11).
func (r *totpRepo) UpdateAccount(_ context.Context, acc *model.Account) (*model.Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	stored := *acc
	stored.MFA = r.accounts[acc.ID.Hex()].MFA
	r.accounts[acc.ID.Hex()] = stored
	return acc, nil
}

func (r *totpRepo) ReplaceBackupCodes(_ context.Context, id string, proof model.TOTPProof, hashes []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.writeErr != nil {
		return r.writeErr
	}
	acc := r.accounts[id]
	if !matchesProof(acc.MFA.TOTP, proof) {
		return dbErrors.ErrTOTPStateChanged
	}
	acc.MFA.TOTP = model.TotpSettings{Enabled: true, SecretSealed: acc.MFA.TOTP.SecretSealed, BackupCodeHashes: slices.Clone(hashes)}
	r.accounts[id] = acc
	return nil
}

func matchesProof(t model.TotpSettings, proof model.TOTPProof) bool {
	if !t.Enabled || proof.SecretSealed == "" || t.SecretSealed != proof.SecretSealed {
		return false
	}
	return proof.BackupCodeHash == "" || slices.Contains(t.BackupCodeHashes, proof.BackupCodeHash)
}

func (r *totpRepo) ConsumeBackupCode(_ context.Context, id, hash string) (bool, error) {
	return r.consume(id, hash, false), nil
}

func (r *totpRepo) ConsumeLegacyBackupCode(_ context.Context, id, code string) (bool, error) {
	return r.consume(id, code, true), nil
}

func (r *totpRepo) consume(id, value string, legacy bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	acc := r.accounts[id]
	unused := &acc.MFA.TOTP.BackupCodeHashes
	if legacy {
		unused = &acc.MFA.TOTP.BackupCodes
	}
	i := slices.Index(*unused, value)
	if !acc.MFA.TOTP.Enabled || i < 0 {
		return false
	}
	*unused = slices.Delete(slices.Clone(*unused), i, i+1)
	r.accounts[id] = acc
	return true
}

func (r *totpRepo) stored(id string) model.TotpSettings {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.accounts[id].MFA.TOTP
}

type totpCache struct {
	cache.Cache
	mu      sync.Mutex
	data    map[string]string
	ctr     map[string]int64
	incrErr error
}

func (c *totpCache) SetTOTPSecret(_ context.Context, id, secret string, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data["totp:"+id] = secret
	return nil
}

func (c *totpCache) GetTOTPSecret(_ context.Context, id string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.data["totp:"+id]
	if !ok {
		return "", redis.Nil
	}
	return v, nil
}

func (c *totpCache) DeleteTOTPSecret(_ context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, "totp:"+id)
	return nil
}

func (c *totpCache) Get(_ context.Context, key string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return strconv.FormatInt(c.ctr[key], 10), nil
}

func (c *totpCache) Del(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.ctr, key)
	return nil
}

func (c *totpCache) Incr(_ context.Context, key string, _ time.Duration) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.incrErr != nil {
		return 0, c.incrErr
	}
	c.ctr[key]++
	return c.ctr[key], nil
}

type totpFixture struct {
	svc    *account.AccountService
	repo   *totpRepo
	cache  *totpCache
	keeper *totpcrypt.Keeper
	id     string
}

func newTOTPFixture(t *testing.T, storageKey []byte) *totpFixture {
	t.Helper()
	id := primitive.NewObjectID()
	acc := model.Account{ID: id, Email: "totp@example.com", Password: &testPasswordHash}
	repo := &totpRepo{accounts: map[string]model.Account{id.Hex(): acc}}
	c := &totpCache{data: map[string]string{}, ctr: map[string]int64{}}
	cfg := config.ServiceConfig{
		OTPExpirationTime:   5 * time.Minute,
		IdLimiterMax:        5,
		IdLimiterExpiration: time.Minute,
		TOTPStorageKey:      storageKey,
	}
	svc := account.NewAccountService(cfg, repo, nil, nil, nil, nil, c, nil, nil, nil, webhookClient.Http{})
	f := &totpFixture{svc: svc, repo: repo, cache: c, id: id.Hex()}
	if len(storageKey) == totpcrypt.KeySize {
		k, err := totpcrypt.New(storageKey)
		require.NoError(t, err)
		f.keeper = k
	}
	return f
}

const testPassword = "Curr3nt-Passw0rd!"

// testPasswordHash is testPassword at the lowest bcrypt cost, to keep the
// fixtures fast; verification reads the cost from the hash.
var testPasswordHash = func() string {
	h, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.MinCost)
	if err != nil {
		panic(err)
	}
	return string(h)
}()

// regenerate is a regenerate request verified with the current password.
func (f *totpFixture) regenerate(otp string) requests.TotpRegenerateReq {
	password := testPassword
	return requests.TotpRegenerateReq{OTP: otp, CurrentPassword: &password}
}

// enableReq is a set-up request verified with the current password.
func (f *totpFixture) enableReq() requests.TotpEnableReq {
	password := testPassword
	return requests.TotpEnableReq{CurrentPassword: &password}
}

// disable is a disable request verified with the current password.
func (f *totpFixture) disable(otp string) requests.TotpDisableReq {
	password := testPassword
	return requests.TotpDisableReq{OTP: otp, CurrentPassword: &password}
}

func testStorageKey(fill byte) []byte { return bytes.Repeat([]byte{fill}, totpcrypt.KeySize) }

// enable runs the enrolment flow and returns the secret and backup codes.
func (f *totpFixture) enable(t *testing.T) (string, []string) {
	t.Helper()
	ctx := context.Background()
	newTOTP, err := f.svc.TotpEnable(ctx, f.id, f.enableReq())
	require.NoError(t, err)
	code, err := totp.GenerateCode(newTOTP.Secret, time.Now())
	require.NoError(t, err)
	backup, err := f.svc.TotpConfirm(ctx, f.id, code)
	require.NoError(t, err)
	return newTOTP.Secret, backup.BackupCodes
}

func (f *totpFixture) setLegacy(totpSettings model.TotpSettings) {
	f.repo.mu.Lock()
	defer f.repo.mu.Unlock()
	acc := f.repo.accounts[f.id]
	acc.MFA.TOTP = totpSettings
	f.repo.accounts[f.id] = acc
}

// specRef: api-endpoint-behaviour.md E4
func TestTotpEnable_PendingSecretIsSealed(t *testing.T) {
	f := newTOTPFixture(t, testStorageKey(1))
	newTOTP, err := f.svc.TotpEnable(context.Background(), f.id, f.enableReq())
	require.NoError(t, err)
	require.NotEmpty(t, newTOTP.Secret)

	cached := f.cache.data["totp:"+f.id]
	require.True(t, totpcrypt.IsSealed(cached))
	require.NotContains(t, cached, newTOTP.Secret)
	opened, err := f.keeper.OpenPendingSecret(f.id, cached)
	require.NoError(t, err)
	require.Equal(t, newTOTP.Secret, opened)
}

// specRef: api-endpoint-behaviour.md E2, E4, E5
func TestTotpConfirm_StoresSealedSecretAndHashedCodes(t *testing.T) {
	f := newTOTPFixture(t, testStorageKey(1))
	secret, codes := f.enable(t)

	require.Len(t, codes, 8)
	for _, c := range codes {
		require.Len(t, c, 25)
		for _, ch := range c {
			require.True(t, strings.ContainsRune(utils.AlphaNumericUserFriendlyUppercase, ch))
		}
	}

	require.NotContains(t, f.cache.data, "totp:"+f.id, "pending secret is single-use")

	stored := f.repo.stored(f.id)
	require.True(t, stored.Enabled)
	opened, err := f.keeper.OpenSecret(f.id, stored.SecretSealed)
	require.NoError(t, err)
	require.Equal(t, secret, opened)

	want := make([]string, 0, len(codes))
	for _, c := range codes {
		want = append(want, f.keeper.HashBackupCode(f.id, c))
	}
	require.Equal(t, want, stored.BackupCodeHashes)
	require.Empty(t, stored.Secret)
	require.Empty(t, stored.BackupCodes)
	require.False(t, stored.BackupCodesLegacy)
}

// The pending secret may also be stored unsealed.
// specRef: api-endpoint-behaviour.md E8
func TestTotpConfirm_AcceptsPlaintextPendingSecret(t *testing.T) {
	f := newTOTPFixture(t, testStorageKey(1))
	f.cache.data["totp:"+f.id] = legacySecret
	code, err := totp.GenerateCode(legacySecret, time.Now())
	require.NoError(t, err)

	_, err = f.svc.TotpConfirm(context.Background(), f.id, code)
	require.NoError(t, err)

	opened, err := f.keeper.OpenSecret(f.id, f.repo.stored(f.id).SecretSealed)
	require.NoError(t, err)
	require.Equal(t, legacySecret, opened)
}

// Set-up takes the code as authenticator apps display it, like login does.
// specRef: api-endpoint-behaviour.md E2
func TestTotpConfirm_SeparatedCode(t *testing.T) {
	for _, sep := range []string{" ", "-"} {
		t.Run(strconv.Quote(sep), func(t *testing.T) {
			f := newTOTPFixture(t, testStorageKey(1))
			newTOTP, err := f.svc.TotpEnable(context.Background(), f.id, f.enableReq())
			require.NoError(t, err)
			code, err := totp.GenerateCode(newTOTP.Secret, time.Now())
			require.NoError(t, err)

			_, err = f.svc.TotpConfirm(context.Background(), f.id, code[:3]+sep+code[3:])
			require.NoError(t, err)
			require.True(t, f.repo.stored(f.id).Enabled)
		})
	}
}

// specRef: api-endpoint-behaviour.md E2
func TestTotpConfirm_ConcurrentEnableReportsAlreadyConfigured(t *testing.T) {
	f := newTOTPFixture(t, testStorageKey(1))
	newTOTP, err := f.svc.TotpEnable(context.Background(), f.id, f.enableReq())
	require.NoError(t, err)
	f.repo.enableErr = dbErrors.ErrTOTPStateChanged
	code, err := totp.GenerateCode(newTOTP.Secret, time.Now())
	require.NoError(t, err)

	backup, err := f.svc.TotpConfirm(context.Background(), f.id, code)
	require.ErrorIs(t, err, account.ErrTOTPAlreadyConfigured)
	require.Nil(t, backup, "no backup codes when nothing was stored")
}

// specRef: api-endpoint-behaviour.md E4
func TestVerifyTotp_SealedSecret(t *testing.T) {
	f := newTOTPFixture(t, testStorageKey(1))
	secret, _ := f.enable(t)
	code, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)

	_, err = f.svc.VerifyTotp(context.Background(), f.id, code)
	require.NoError(t, err)

	_, err = f.svc.VerifyTotp(context.Background(), f.id, "000000")
	require.ErrorIs(t, err, account.ErrInvalidTOTPCode)
}

// Authenticator apps display codes as "123 456"; a pasted separator is ignored.
// specRef: api-endpoint-behaviour.md E6
func TestVerifyTotp_SeparatedTOTPCode(t *testing.T) {
	f := newTOTPFixture(t, testStorageKey(1))
	secret, _ := f.enable(t)
	code, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)

	_, err = f.svc.VerifyTotp(context.Background(), f.id, " "+code[:3]+" "+code[3:]+" ")
	require.NoError(t, err)
	// The previous time step is still valid (RFC 6238 §5.2 skew of one step).
	prev, err := totp.GenerateCode(secret, time.Now().Add(-30*time.Second))
	require.NoError(t, err)
	_, err = f.svc.VerifyTotp(context.Background(), f.id, prev[:3]+"-"+prev[3:])
	require.NoError(t, err)

	_, err = f.svc.VerifyTotp(context.Background(), f.id, strings.Repeat("-", 1<<20)+code)
	require.ErrorIs(t, err, account.ErrInvalidTOTPCode)
}

// specRef: api-endpoint-behaviour.md E6
func TestVerifyTotp_BackupCodeIsSingleUse(t *testing.T) {
	f := newTOTPFixture(t, testStorageKey(1))
	_, codes := f.enable(t)
	ctx := context.Background()

	_, err := f.svc.VerifyTotp(ctx, f.id, codes[0])
	require.NoError(t, err)

	// A used code is gone: it is refused like any wrong code.
	_, err = f.svc.VerifyTotp(ctx, f.id, codes[0])
	require.ErrorIs(t, err, account.ErrInvalidTOTPCode)

	_, err = f.svc.VerifyTotp(ctx, f.id, "ZZZZZZZZZZZZZZZZZZZZ")
	require.ErrorIs(t, err, account.ErrInvalidTOTPCode)

	stored := f.repo.stored(f.id)
	require.Len(t, stored.BackupCodeHashes, 7)
	require.NotContains(t, stored.BackupCodeHashes, f.keeper.HashBackupCode(f.id, codes[0]))
}

// Backup codes are displayed in groups of five; the separators are ignored.
// specRef: api-endpoint-behaviour.md E6
func TestVerifyTotp_GroupedBackupCode(t *testing.T) {
	f := newTOTPFixture(t, testStorageKey(1))
	_, codes := f.enable(t)
	ctx := context.Background()

	group := func(code, sep string) string {
		var parts []string
		for i := 0; i < len(code); i += 5 {
			parts = append(parts, code[i:min(i+5, len(code))])
		}
		return strings.Join(parts, sep)
	}

	_, err := f.svc.VerifyTotp(ctx, f.id, group(codes[0], "-"))
	require.NoError(t, err)
	_, err = f.svc.VerifyTotp(ctx, f.id, " "+group(codes[1], " ")+" ")
	require.NoError(t, err)

	// A grouped code is the same code: it is single-use.
	_, err = f.svc.VerifyTotp(ctx, f.id, codes[0])
	require.ErrorIs(t, err, account.ErrInvalidTOTPCode)
	_, err = f.svc.VerifyTotp(ctx, f.id, strings.ToLower(codes[0]))
	require.ErrorIs(t, err, account.ErrInvalidTOTPCode)

	// Codes are uppercase; case is not part of the code.
	_, err = f.svc.VerifyTotp(ctx, f.id, strings.ToLower(group(codes[3], "-")))
	require.NoError(t, err)

	// Grouping does not stretch a code past the stored length.
	_, err = f.svc.VerifyTotp(ctx, f.id, group(codes[2]+"X", "-"))
	require.ErrorIs(t, err, account.ErrInvalidTOTPCode)
	require.Len(t, f.repo.stored(f.id).BackupCodeHashes, 5)
}

// specRef: api-endpoint-behaviour.md E6
func TestVerifyTotp_ConcurrentBackupCodeUseSucceedsOnce(t *testing.T) {
	f := newTOTPFixture(t, testStorageKey(1))
	_, codes := f.enable(t)

	var wins atomic.Int32
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := f.svc.VerifyTotp(context.Background(), f.id, codes[1]); err == nil {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(1), wins.Load())
}

// specRef: api-endpoint-behaviour.md E8
func TestVerifyTotp_LegacyPlaintextRecord(t *testing.T) {
	f := newTOTPFixture(t, testStorageKey(1))
	f.setLegacy(model.TotpSettings{Enabled: true, Secret: legacySecret, BackupCodes: []string{"LegacyCodeAAAAA1", "LegacyCodeBBBBB2"}})
	ctx := context.Background()

	code, err := totp.GenerateCode(legacySecret, time.Now())
	require.NoError(t, err)
	_, err = f.svc.VerifyTotp(ctx, f.id, code)
	require.NoError(t, err)

	// Legacy codes are mixed case; only the exact case matches.
	_, err = f.svc.VerifyTotp(ctx, f.id, "LEGACYCODEAAAAA1")
	require.ErrorIs(t, err, account.ErrInvalidTOTPCode)

	_, err = f.svc.VerifyTotp(ctx, f.id, "LegacyCodeAAAAA1")
	require.NoError(t, err)
	_, err = f.svc.VerifyTotp(ctx, f.id, "LegacyCodeAAAAA1")
	require.ErrorIs(t, err, account.ErrInvalidTOTPCode)

	require.Equal(t, []string{"LegacyCodeBBBBB2"}, f.repo.stored(f.id).BackupCodes)
}

// Backup codes of 8 characters are accepted.
// specRef: api-endpoint-behaviour.md E8
func TestVerifyTotp_ShortLegacyBackupCode(t *testing.T) {
	f := newTOTPFixture(t, testStorageKey(1))
	f.setLegacy(model.TotpSettings{Enabled: true, Secret: legacySecret, BackupCodes: []string{"Abc2Def3"}})
	_, err := f.svc.VerifyTotp(context.Background(), f.id, "Abc2Def3")
	require.NoError(t, err)

	// The same length stored as a digest.
	f.setLegacy(model.TotpSettings{Enabled: true, Secret: legacySecret, BackupCodesLegacy: true, BackupCodeHashes: []string{f.keeper.HashBackupCode(f.id, "Xyz4Uvw5")}})
	_, err = f.svc.VerifyTotp(context.Background(), f.id, "Xyz4Uvw5")
	require.NoError(t, err)
}

// Migrated codes of earlier formats are mixed case and match only as typed.
// specRef: api-endpoint-behaviour.md E6, E8
func TestVerifyTotp_MigratedLegacyCodeIsCaseSensitive(t *testing.T) {
	f := newTOTPFixture(t, testStorageKey(1))
	f.setLegacy(model.TotpSettings{
		Enabled:           true,
		Secret:            legacySecret,
		BackupCodesLegacy: true,
		BackupCodeHashes: []string{
			f.keeper.HashBackupCode(f.id, "AbCdEfGh2345JkMn"),
			f.keeper.HashBackupCode(f.id, "AB23CD45"),
		},
	})
	ctx := context.Background()

	for _, wrongCase := range []string{"ABCDEFGH2345JKMN", "abcdefgh2345jkmn", "ab23cd45"} {
		_, err := f.svc.VerifyTotp(ctx, f.id, wrongCase)
		require.ErrorIs(t, err, account.ErrInvalidTOTPCode, wrongCase)
	}
	require.Len(t, f.repo.stored(f.id).BackupCodeHashes, 2)

	_, err := f.svc.VerifyTotp(ctx, f.id, "AbCdE-fGh23-45JkMn")
	require.NoError(t, err)
	_, err = f.svc.VerifyTotp(ctx, f.id, "AB23CD45")
	require.NoError(t, err)
}

// Password reset and turning 2FA off take a backup code of an earlier format,
// as stored before or after the storage migration, matched only as typed.
// specRef: api-endpoint-behaviour.md C2, E3, E8
func TestLegacyBackupCodeInResetAndDisable(t *testing.T) {
	const code = "LegacyCodeAAAAA1"
	records := map[string]func(t *testing.T, f *totpFixture) model.TotpSettings{
		"plaintext": func(*testing.T, *totpFixture) model.TotpSettings {
			return model.TotpSettings{Enabled: true, Secret: legacySecret, BackupCodes: []string{code}}
		},
		"migrated": func(t *testing.T, f *totpFixture) model.TotpSettings {
			sealed, err := f.keeper.SealSecret(f.id, legacySecret)
			require.NoError(t, err)
			return model.TotpSettings{
				Enabled: true, SecretSealed: sealed, BackupCodesLegacy: true,
				BackupCodeHashes: []string{f.keeper.HashBackupCode(f.id, code)},
			}
		},
	}
	ctx := context.Background()
	for name, record := range records {
		t.Run(name+"/reset", func(t *testing.T) {
			f := newTOTPFixture(t, testStorageKey(1))
			f.setLegacy(record(t, f))
			acc, err := f.repo.GetAccountById(ctx, f.id)
			require.NoError(t, err)

			require.ErrorIs(t, f.svc.MfaCheck(ctx, acc, &model.MfaData{OTP: strings.ToUpper(code)}), account.ErrInvalidTOTPCode)
			require.NoError(t, f.svc.MfaCheck(ctx, acc, &model.MfaData{OTP: code}))
			require.ErrorIs(t, f.svc.MfaCheck(ctx, acc, &model.MfaData{OTP: code}), account.ErrInvalidTOTPCode, "single use")
		})
		t.Run(name+"/disable", func(t *testing.T) {
			if name == "plaintext" {
				t.Skip("turning 2FA off needs a converted record: TestTotpChangesNeedAConvertedRecord")
			}
			f := newTOTPFixture(t, testStorageKey(1))
			f.setLegacy(record(t, f))

			_, err := f.svc.TotpDisable(ctx, f.id, f.disable(strings.ToLower(code)))
			require.ErrorIs(t, err, account.ErrInvalidTOTPCode)
			require.True(t, f.repo.stored(f.id).Enabled)

			acc, err := f.svc.TotpDisable(ctx, f.id, f.disable(code))
			require.NoError(t, err)
			require.Equal(t, model.TotpSettings{Enabled: false}, acc.MFA.TOTP)
			require.Equal(t, model.TotpSettings{Enabled: false}, f.repo.stored(f.id))
		})
	}
}

// A secret the configured key cannot open is an operator error, not a wrong
// code: verification fails with an internal error and nothing is consumed.
// specRef: api-endpoint-behaviour.md E10
func TestVerifyTotp_UnreadableSecretFailsClosed(t *testing.T) {
	other := newTOTPFixture(t, testStorageKey(2))
	sealed, err := other.keeper.SealSecret(other.id, legacySecret)
	require.NoError(t, err)

	f := newTOTPFixture(t, testStorageKey(1))
	f.repo.accounts[other.id] = model.Account{ID: mustObjectID(t, other.id), MFA: model.MFASettings{TOTP: model.TotpSettings{Enabled: true, SecretSealed: sealed}}}
	code, err := totp.GenerateCode(legacySecret, time.Now())
	require.NoError(t, err)

	_, err = f.svc.VerifyTotp(context.Background(), other.id, code)
	require.Error(t, err)
	var totpErr *account.TOTPError
	require.False(t, errors.As(err, &totpErr), "must not look like a wrong code: %v", err)
}

// specRef: api-endpoint-behaviour.md E7
func TestTotpDisable_RemovesAllMaterial(t *testing.T) {
	f := newTOTPFixture(t, testStorageKey(1))
	_, codes := f.enable(t)

	acc, err := f.svc.TotpDisable(context.Background(), f.id, f.disable(codes[2]))
	require.NoError(t, err)
	require.False(t, acc.MFA.TOTP.Enabled)
	require.Equal(t, model.TotpSettings{Enabled: false}, acc.MFA.TOTP)
	require.Equal(t, model.TotpSettings{Enabled: false}, f.repo.stored(f.id))
}

// specRef: api-endpoint-behaviour.md E10
func TestTotpWithoutStorageKeyFailsClosed(t *testing.T) {
	f := newTOTPFixture(t, nil)
	_, err := f.svc.TotpEnable(context.Background(), f.id, f.enableReq())
	require.Error(t, err)
	require.Empty(t, f.cache.data)
}

func mustObjectID(t *testing.T, hex string) primitive.ObjectID {
	t.Helper()
	id, err := primitive.ObjectIDFromHex(hex)
	require.NoError(t, err)
	return id
}

// specRef: api-endpoint-behaviour.md E2, E3
func TestVerifyTotp_Preconditions(t *testing.T) {
	ctx := context.Background()
	f := newTOTPFixture(t, testStorageKey(1))

	_, err := f.svc.VerifyTotp(ctx, f.id, "123456")
	require.ErrorIs(t, err, account.ErrTOTPNotConfigured)
	_, err = f.svc.TotpDisable(ctx, f.id, f.disable("123456"))
	require.ErrorIs(t, err, account.ErrTOTPAlreadyDisabled)

	f.enable(t)
	_, err = f.svc.VerifyTotp(ctx, primitive.NewObjectID().Hex(), "123456")
	require.ErrorIs(t, err, dbErrors.ErrAccountNotFound)
}

// Past the limit every code is refused, valid ones included, and no backup
// code is consumed. specRef: api-endpoint-behaviour.md E12
func TestVerifyTotp_LockoutRefusesValidCodes(t *testing.T) {
	ctx := context.Background()
	f := newTOTPFixture(t, testStorageKey(1))
	secret, codes := f.enable(t)

	for range 5 {
		_, err := f.svc.VerifyTotp(ctx, f.id, "000000")
		require.ErrorIs(t, err, account.ErrInvalidTOTPCode)
	}

	code, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)
	_, err = f.svc.VerifyTotp(ctx, f.id, code)
	require.ErrorIs(t, err, account.ErrTOTPTooManyAttempts)

	_, err = f.svc.VerifyTotp(ctx, f.id, codes[0])
	require.ErrorIs(t, err, account.ErrTOTPTooManyAttempts)
	require.Len(t, f.repo.stored(f.id).BackupCodeHashes, 8, "no backup code consumed while locked")
}

// specRef: api-endpoint-behaviour.md E12
func TestVerifyTotp_SuccessResetsFailureCount(t *testing.T) {
	ctx := context.Background()
	f := newTOTPFixture(t, testStorageKey(1))
	secret, codes := f.enable(t)

	for range 4 {
		_, err := f.svc.VerifyTotp(ctx, f.id, "000000")
		require.ErrorIs(t, err, account.ErrInvalidTOTPCode)
	}
	code, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)
	_, err = f.svc.VerifyTotp(ctx, f.id, code)
	require.NoError(t, err)
	require.Zero(t, f.cache.ctr["totp_fails:"+f.id])

	for range 4 {
		_, err := f.svc.VerifyTotp(ctx, f.id, "000000")
		require.ErrorIs(t, err, account.ErrInvalidTOTPCode)
	}
	_, err = f.svc.VerifyTotp(ctx, f.id, codes[0])
	require.NoError(t, err)
	require.Zero(t, f.cache.ctr["totp_fails:"+f.id])
}

// specRef: api-endpoint-behaviour.md E12
func TestTotpConfirm_LockoutRefusesValidCode(t *testing.T) {
	ctx := context.Background()
	f := newTOTPFixture(t, testStorageKey(1))
	newTOTP, err := f.svc.TotpEnable(ctx, f.id, f.enableReq())
	require.NoError(t, err)

	for range 5 {
		_, err := f.svc.TotpConfirm(ctx, f.id, "000000")
		require.ErrorIs(t, err, account.ErrIncorrectOTP)
	}
	code, err := totp.GenerateCode(newTOTP.Secret, time.Now())
	require.NoError(t, err)
	_, err = f.svc.TotpConfirm(ctx, f.id, code)
	require.ErrorIs(t, err, account.ErrTOTPTooManyAttempts)
	require.False(t, f.repo.stored(f.id).Enabled)
}

// specRef: api-endpoint-behaviour.md E2
func TestTotpConfirm_Errors(t *testing.T) {
	ctx := context.Background()
	f := newTOTPFixture(t, testStorageKey(1))

	_, err := f.svc.TotpConfirm(ctx, f.id, "123456")
	require.ErrorIs(t, err, account.ErrTOTPSetupNotStarted)

	_, err = f.svc.TotpEnable(ctx, f.id, f.enableReq())
	require.NoError(t, err)
	_, err = f.svc.TotpConfirm(ctx, f.id, "000000")
	require.ErrorIs(t, err, account.ErrIncorrectOTP)
	require.Equal(t, int64(1), f.cache.ctr["totp_confirm_fails:"+f.id])
	require.False(t, f.repo.stored(f.id).Enabled)

	f.enable(t)
	_, err = f.svc.TotpEnable(ctx, f.id, f.enableReq())
	require.ErrorIs(t, err, account.ErrTOTPAlreadyConfigured)
	f.cache.data["totp:"+f.id] = "pending"
	_, err = f.svc.TotpConfirm(ctx, f.id, "000000")
	require.ErrorIs(t, err, account.ErrTOTPAlreadyConfigured)
}

// Concurrent attempts are counted before any code is checked, so a burst
// cannot test more codes than the limit. specRef: api-endpoint-behaviour.md E12
func TestVerifyTotp_ConcurrentAttemptsBoundedByLimit(t *testing.T) {
	f := newTOTPFixture(t, testStorageKey(1))
	f.enable(t)

	var checked, refused atomic.Int32
	var wg sync.WaitGroup
	for range 30 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.svc.VerifyTotp(context.Background(), f.id, "000000")
			switch {
			case errors.Is(err, account.ErrInvalidTOTPCode):
				checked.Add(1)
			case errors.Is(err, account.ErrTOTPTooManyAttempts):
				refused.Add(1)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(5), checked.Load())
	require.Equal(t, int32(25), refused.Load())
}

// Without a working failure counter no code is checked.
// specRef: api-endpoint-behaviour.md E12
func TestVerifyTotp_CounterErrorRefusesValidCode(t *testing.T) {
	f := newTOTPFixture(t, testStorageKey(1))
	secret, codes := f.enable(t)
	f.cache.incrErr = errors.New("redis: connection refused")

	code, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)
	_, err = f.svc.VerifyTotp(context.Background(), f.id, code)
	require.Error(t, err)
	_, err = f.svc.VerifyTotp(context.Background(), f.id, codes[0])
	require.Error(t, err)
	require.Len(t, f.repo.stored(f.id).BackupCodeHashes, 8)
}

// An enabled account with no stored secret never validates a code.
// specRef: api-endpoint-behaviour.md E10
func TestVerifyTotp_EnabledWithoutSecretFailsClosed(t *testing.T) {
	f := newTOTPFixture(t, testStorageKey(1))
	f.setLegacy(model.TotpSettings{Enabled: true})

	code, err := totp.GenerateCode("", time.Now())
	require.NoError(t, err)
	_, err = f.svc.VerifyTotp(context.Background(), f.id, code)
	require.Error(t, err)
	var totpErr *account.TOTPError
	require.False(t, errors.As(err, &totpErr), "must not look like a wrong code: %v", err)
}

// A backup code that authorised turning 2FA off is used up only together with
// the change: a failed write leaves both the code and 2FA in place.
// specRef: api-endpoint-behaviour.md E3, E7
func TestTotpDisable_FailedWriteKeepsTheBackupCode(t *testing.T) {
	f := newTOTPFixture(t, testStorageKey(1))
	_, codes := f.enable(t)
	ctx := context.Background()

	f.repo.writeErr = errors.New("write failed")
	_, err := f.svc.TotpDisable(ctx, f.id, f.disable(codes[0]))
	require.ErrorIs(t, err, f.repo.writeErr)
	stored := f.repo.stored(f.id)
	require.True(t, stored.Enabled)
	require.Contains(t, stored.BackupCodeHashes, f.keeper.HashBackupCode(f.id, codes[0]))

	f.repo.writeErr = nil
	_, err = f.svc.TotpDisable(ctx, f.id, f.disable(codes[0]))
	require.NoError(t, err)
	require.False(t, f.repo.stored(f.id).Enabled)
}

// specRef: api-endpoint-behaviour.md E13, E5
func TestTotpRegenerate_ReplacesEveryCode(t *testing.T) {
	for name, authorise := range map[string]func(t *testing.T, secret string, codes []string) string{
		"authenticator code": func(t *testing.T, secret string, _ []string) string {
			code, err := totp.GenerateCode(secret, time.Now())
			require.NoError(t, err)
			return code
		},
		"backup code": func(_ *testing.T, _ string, codes []string) string { return codes[3] },
	} {
		t.Run(name, func(t *testing.T) {
			f := newTOTPFixture(t, testStorageKey(1))
			secret, oldCodes := f.enable(t)
			ctx := context.Background()
			sealedBefore := f.repo.stored(f.id).SecretSealed

			backup, err := f.svc.TotpRegenerateBackupCodes(ctx, f.id, f.regenerate(authorise(t, secret, oldCodes)))
			require.NoError(t, err)
			require.Len(t, backup.BackupCodes, 8)
			for _, code := range backup.BackupCodes {
				require.Len(t, code, 25)
				require.Empty(t, strings.Trim(code, utils.AlphaNumericUserFriendlyUppercase), code)
				require.NotContains(t, oldCodes, code)
			}

			stored := f.repo.stored(f.id)
			require.True(t, stored.Enabled)
			require.Equal(t, sealedBefore, stored.SecretSealed, "the authenticator keeps working")
			require.Len(t, stored.BackupCodeHashes, 8)

			for _, old := range oldCodes {
				_, err = f.svc.VerifyTotp(ctx, f.id, old)
				require.ErrorIs(t, err, account.ErrInvalidTOTPCode, old)
				f.cache.ctr = map[string]int64{}
			}
			_, err = f.svc.VerifyTotp(ctx, f.id, backup.BackupCodes[0])
			require.NoError(t, err)
		})
	}
}

// Regenerating replaces a code set of an earlier format and clears its marker.
// specRef: api-endpoint-behaviour.md E13, E14
func TestTotpRegenerate_ClearsLegacyMarker(t *testing.T) {
	const legacyCode = "LegacyCodeAAAAA1"
	f := newTOTPFixture(t, testStorageKey(1))
	sealed, err := f.keeper.SealSecret(f.id, legacySecret)
	require.NoError(t, err)
	f.setLegacy(model.TotpSettings{
		Enabled: true, SecretSealed: sealed, BackupCodesLegacy: true,
		BackupCodeHashes: []string{f.keeper.HashBackupCode(f.id, legacyCode), f.keeper.HashBackupCode(f.id, "LegacyCodeBBBBB2")},
	})
	ctx := context.Background()

	backup, err := f.svc.TotpRegenerateBackupCodes(ctx, f.id, f.regenerate(legacyCode))
	require.NoError(t, err)

	stored := f.repo.stored(f.id)
	require.False(t, stored.BackupCodesLegacy)
	require.Len(t, stored.BackupCodeHashes, len(backup.BackupCodes))
	_, err = f.svc.VerifyTotp(ctx, f.id, "LegacyCodeBBBBB2")
	require.ErrorIs(t, err, account.ErrInvalidTOTPCode)
}

// specRef: api-endpoint-behaviour.md E13, E12
func TestTotpRegenerate_Refusals(t *testing.T) {
	ctx := context.Background()

	t.Run("2FA off", func(t *testing.T) {
		f := newTOTPFixture(t, testStorageKey(1))
		_, err := f.svc.TotpRegenerateBackupCodes(ctx, f.id, f.regenerate("123456"))
		require.ErrorIs(t, err, account.ErrTOTPNotConfigured)
	})

	t.Run("wrong code", func(t *testing.T) {
		f := newTOTPFixture(t, testStorageKey(1))
		f.enable(t)
		before := f.repo.stored(f.id)
		for _, code := range []string{"000000", "ZZZZZ-ZZZZZ-ZZZZZ-ZZZZZ-ZZZZZ", strings.Repeat("A", 65)} {
			_, err := f.svc.TotpRegenerateBackupCodes(ctx, f.id, f.regenerate(code))
			require.ErrorIs(t, err, account.ErrInvalidTOTPCode, code)
		}
		require.Equal(t, before, f.repo.stored(f.id))
	})

	t.Run("locked", func(t *testing.T) {
		f := newTOTPFixture(t, testStorageKey(1))
		secret, codes := f.enable(t)
		for range 5 {
			_, err := f.svc.TotpRegenerateBackupCodes(ctx, f.id, f.regenerate("000000"))
			require.ErrorIs(t, err, account.ErrInvalidTOTPCode)
		}
		before := f.repo.stored(f.id)
		code, err := totp.GenerateCode(secret, time.Now())
		require.NoError(t, err)
		for _, valid := range []string{code, codes[0]} {
			_, err = f.svc.TotpRegenerateBackupCodes(ctx, f.id, f.regenerate(valid))
			require.ErrorIs(t, err, account.ErrTOTPTooManyAttempts)
		}
		require.Equal(t, before, f.repo.stored(f.id), "nothing consumed or replaced")
	})

	t.Run("failed write keeps the old set", func(t *testing.T) {
		f := newTOTPFixture(t, testStorageKey(1))
		_, codes := f.enable(t)
		f.repo.writeErr = errors.New("write failed")
		_, err := f.svc.TotpRegenerateBackupCodes(ctx, f.id, f.regenerate(codes[0]))
		require.ErrorIs(t, err, f.repo.writeErr)
		f.repo.writeErr = nil
		_, err = f.svc.VerifyTotp(ctx, f.id, codes[0])
		require.NoError(t, err)
	})
}

// A backup code may authorise one change only, however many requests race.
// specRef: api-endpoint-behaviour.md E13, E6
func TestTotpRegenerate_ConcurrentUseOfOneBackupCodeSucceedsOnce(t *testing.T) {
	f := newTOTPFixture(t, testStorageKey(1))
	_, codes := f.enable(t)

	var wins atomic.Int32
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.svc.TotpRegenerateBackupCodes(context.Background(), f.id, f.regenerate(codes[0]))
			switch {
			case err == nil:
				wins.Add(1)
			case !errors.Is(err, account.ErrInvalidTOTPCode):
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(1), wins.Load())
}

// Turning 2FA off and regenerating write against the sealed secret, so a
// record the storage migration has not converted is refused as an internal
// error and left unchanged.
// specRef: api-endpoint-behaviour.md E8, E13
func TestTotpChangesNeedAConvertedRecord(t *testing.T) {
	ctx := context.Background()
	code, err := totp.GenerateCode(legacySecret, time.Now())
	require.NoError(t, err)
	record := model.TotpSettings{Enabled: true, Secret: legacySecret, BackupCodes: []string{"LegacyCodeAAAAA1"}}

	for name, change := range map[string]func(f *totpFixture, otp string) error{
		"disable": func(f *totpFixture, otp string) error {
			_, err := f.svc.TotpDisable(ctx, f.id, f.disable(otp))
			return err
		},
		"regenerate": func(f *totpFixture, otp string) error {
			_, err := f.svc.TotpRegenerateBackupCodes(ctx, f.id, f.regenerate(otp))
			return err
		},
	} {
		for _, otp := range []string{code, "LegacyCodeAAAAA1"} {
			t.Run(name+"/"+otp, func(t *testing.T) {
				f := newTOTPFixture(t, testStorageKey(1))
				f.setLegacy(record)
				err := change(f, otp)
				require.Error(t, err)
				var totpErr *account.TOTPError
				require.False(t, errors.As(err, &totpErr), "must not look like a wrong code: %v", err)
				require.Equal(t, record, f.repo.stored(f.id))
			})
		}
	}
}

// Each check that needs the current password and a 2FA code checks the
// password first: a wrong password leaves a backup code unused and is not
// counted as a failed 2FA attempt. Without a code the 2FA prompt comes first.
// specRef: api-endpoint-behaviour.md E6, E12
func TestPasswordIsCheckedBeforeTheCode(t *testing.T) {
	ctx := context.Background()
	changes := map[string]func(f *totpFixture, password string, mfa *model.MfaData) error{
		"update password": func(f *totpFixture, password string, mfa *model.MfaData) error {
			return f.svc.UpdateAccount(ctx, f.id, []model.AccountUpdate{
				{Operation: model.UpdateOperationTest, Path: "/password", Value: password},
				{Operation: model.UpdateOperationReplace, Path: "/password", Value: "N3w-Passw0rd-Long!"},
			}, mfa)
		},
		"change email": func(f *totpFixture, password string, mfa *model.MfaData) error {
			return f.svc.UpdateAccount(ctx, f.id, []model.AccountUpdate{
				{Operation: model.UpdateOperationReplace, Path: "/email", Value: map[string]any{"current_password": password, "new_email": "new@example.com"}},
			}, mfa)
		},
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			f := newTOTPFixture(t, testStorageKey(1))
			_, codes := f.enable(t)
			f.repo.mu.Lock()
			acc := f.repo.accounts[f.id]
			require.NoError(t, acc.SetPassword("Curr3nt-Passw0rd!"))
			f.repo.accounts[f.id] = acc
			f.repo.mu.Unlock()

			require.ErrorIs(t, change(f, "wrong", &model.MfaData{}), account.ErrTOTPRequired)

			for range 6 { // past ID_LIMITER_MAX
				require.Error(t, change(f, "wrong", &model.MfaData{OTP: codes[0]}))
			}
			require.Len(t, f.repo.stored(f.id).BackupCodeHashes, 8, "a wrong password uses no code up")

			require.NoError(t, change(f, "Curr3nt-Passw0rd!", &model.MfaData{OTP: codes[0]}), "no 2FA lockout")
			require.Len(t, f.repo.stored(f.id).BackupCodeHashes, 7)
		})
	}
}

// A password change checks the code once, after every other check: a refused
// new password leaves a backup code unused, and two test/replace pairs in one
// request use one code.
// specRef: api-endpoint-behaviour.md B3a, E6
func TestPasswordChangeChecksTheCodeOnceAndLast(t *testing.T) {
	ctx := context.Background()
	f := newTOTPFixture(t, testStorageKey(1))
	_, codes := f.enable(t)
	f.repo.mu.Lock()
	acc := f.repo.accounts[f.id]
	require.NoError(t, acc.SetPassword("Curr3nt-Passw0rd!"))
	f.repo.accounts[f.id] = acc
	f.repo.mu.Unlock()
	mfa := &model.MfaData{OTP: codes[0]}

	err := f.svc.UpdateAccount(ctx, f.id, []model.AccountUpdate{
		{Operation: model.UpdateOperationTest, Path: "/password", Value: "Curr3nt-Passw0rd!"},
		{Operation: model.UpdateOperationReplace, Path: "/password", Value: "abc"},
	}, mfa)
	require.ErrorIs(t, err, account.ErrPasswordTooSimple)
	require.Len(t, f.repo.stored(f.id).BackupCodeHashes, 8)

	require.NoError(t, f.svc.UpdateAccount(ctx, f.id, []model.AccountUpdate{
		{Operation: model.UpdateOperationTest, Path: "/password", Value: "Curr3nt-Passw0rd!"},
		{Operation: model.UpdateOperationReplace, Path: "/password", Value: "N3w-Passw0rd-Long!"},
		{Operation: model.UpdateOperationTest, Path: "/password", Value: "N3w-Passw0rd-Long!"},
		{Operation: model.UpdateOperationReplace, Path: "/password", Value: "Th1rd-Passw0rd-Long!"},
	}, mfa))
	require.Len(t, f.repo.stored(f.id).BackupCodeHashes, 7)
}

// Turning 2FA off and regenerating first verify the account owner, with the
// current password or a passkey reauth token of the change's own purpose. A
// refused reauth checks no code: none is used up and none counts toward the
// lockout.
// specRef: api-endpoint-behaviour.md E3, E13, E12
func TestTotpChangesNeedReauth(t *testing.T) {
	ctx := context.Background()
	type change struct {
		tokenType string
		run       func(f *totpFixture, otp string, password, token *string) error
	}
	changes := map[string]change{
		"disable": {auth.TokenTypeReauthTotpDisable, func(f *totpFixture, otp string, password, token *string) error {
			_, err := f.svc.TotpDisable(ctx, f.id, requests.TotpDisableReq{OTP: otp, CurrentPassword: password, ReauthToken: token})
			return err
		}},
		"regenerate": {auth.TokenTypeReauthBackupCodesRegen, func(f *totpFixture, otp string, password, token *string) error {
			_, err := f.svc.TotpRegenerateBackupCodes(ctx, f.id, requests.TotpRegenerateReq{OTP: otp, CurrentPassword: password, ReauthToken: token})
			return err
		}},
	}
	wrong, correct, tokenValue := "wrong-password", testPassword, "reauth-token"
	token := func(tokenType string, expiresIn time.Duration) *model.Token {
		return &model.Token{Type: tokenType, Value: tokenValue, ExpiresAt: time.Now().Add(expiresIn)}
	}

	for name, ch := range changes {
		refusals := map[string]struct {
			password, token *string
			stored          *model.Token
			err             error
		}{
			"no reauth":                {err: reauth.ErrMissingAuthMethod},
			"wrong password":           {password: &wrong, err: reauth.ErrInvalidPassword},
			"token of another purpose": {token: &tokenValue, stored: token(auth.TokenTypeReauthPasskeyDelete, time.Minute), err: reauth.ErrInvalidReauthToken},
			"expired token":            {token: &tokenValue, stored: token(ch.tokenType, -time.Minute), err: reauth.ErrReauthTokenExpired},
		}
		for refusal, tc := range refusals {
			t.Run(name+"/"+refusal, func(t *testing.T) {
				f := newTOTPFixture(t, testStorageKey(1))
				_, codes := f.enable(t)
				if tc.stored != nil {
					f.addToken(*tc.stored)
				}
				before := f.repo.stored(f.id)
				for range 6 { // past ID_LIMITER_MAX
					require.ErrorIs(t, ch.run(f, codes[0], tc.password, tc.token), tc.err)
				}
				require.Equal(t, before, f.repo.stored(f.id), "no code used up, nothing changed")
				require.NoError(t, ch.run(f, codes[0], &correct, nil), "no 2FA lockout")
			})
		}

		t.Run(name+"/passkey reauth token", func(t *testing.T) {
			f := newTOTPFixture(t, testStorageKey(1))
			_, codes := f.enable(t)
			f.addToken(*token(ch.tokenType, time.Minute))
			require.ErrorIs(t, ch.run(f, "000000", nil, &tokenValue), account.ErrInvalidTOTPCode, "the code is still required")
			require.ErrorIs(t, ch.run(f, codes[0], nil, &tokenValue), reauth.ErrInvalidReauthToken, "the token is single-use")
			f.addToken(*token(ch.tokenType, time.Minute))
			require.NoError(t, ch.run(f, codes[0], nil, &tokenValue))
		})

		t.Run(name+"/a valid password still needs the code", func(t *testing.T) {
			f := newTOTPFixture(t, testStorageKey(1))
			f.enable(t)
			before := f.repo.stored(f.id)
			require.ErrorIs(t, ch.run(f, "000000", &correct, nil), account.ErrInvalidTOTPCode)
			require.Equal(t, before, f.repo.stored(f.id))
		})
	}
}

// Set-up starts only after the account owner is verified with the current
// password or a single-use passkey reauth token of purpose totp_enable. A
// refused reauth issues no secret. specRef: api-endpoint-behaviour.md E1
func TestTotpEnableNeedsReauth(t *testing.T) {
	ctx := context.Background()
	wrong, tokenValue := "wrong-password", "reauth-token"
	token := func(tokenType string, expiresIn time.Duration) model.Token {
		return model.Token{Type: tokenType, Value: tokenValue, ExpiresAt: time.Now().Add(expiresIn)}
	}
	refusals := map[string]struct {
		req    requests.TotpEnableReq
		stored *model.Token
		err    error
	}{
		"no reauth":                {err: reauth.ErrMissingAuthMethod},
		"wrong password":           {req: requests.TotpEnableReq{CurrentPassword: &wrong}, err: reauth.ErrInvalidPassword},
		"token of another purpose": {req: requests.TotpEnableReq{ReauthToken: &tokenValue}, stored: ptr(token(auth.TokenTypeReauthTotpDisable, time.Minute)), err: reauth.ErrInvalidReauthToken},
		"expired token":            {req: requests.TotpEnableReq{ReauthToken: &tokenValue}, stored: ptr(token(auth.TokenTypeReauthTotpEnable, -time.Minute)), err: reauth.ErrReauthTokenExpired},
	}
	for name, tc := range refusals {
		t.Run(name, func(t *testing.T) {
			f := newTOTPFixture(t, testStorageKey(1))
			if tc.stored != nil {
				f.addToken(*tc.stored)
			}
			_, err := f.svc.TotpEnable(ctx, f.id, tc.req)
			require.ErrorIs(t, err, tc.err)
			require.Empty(t, f.cache.data, "no secret issued")
		})
	}

	t.Run("2FA already on: refused before the reauth", func(t *testing.T) {
		f := newTOTPFixture(t, testStorageKey(1))
		f.enable(t)
		f.addToken(token(auth.TokenTypeReauthTotpEnable, time.Minute))
		before := f.repo.stored(f.id)
		_, err := f.svc.TotpEnable(ctx, f.id, requests.TotpEnableReq{CurrentPassword: &wrong})
		require.ErrorIs(t, err, account.ErrTOTPAlreadyConfigured, "the password is not checked without the code")
		_, err = f.svc.TotpEnable(ctx, f.id, requests.TotpEnableReq{ReauthToken: &tokenValue})
		require.ErrorIs(t, err, account.ErrTOTPAlreadyConfigured)
		require.Equal(t, before, f.repo.stored(f.id))
		f.repo.mu.Lock()
		defer f.repo.mu.Unlock()
		require.Len(t, f.repo.accounts[f.id].Tokens, 1, "the passkey token is not used up")
	})

	t.Run("passkey reauth token", func(t *testing.T) {
		f := newTOTPFixture(t, testStorageKey(1))
		f.addToken(token(auth.TokenTypeReauthTotpEnable, time.Minute))
		req := requests.TotpEnableReq{ReauthToken: &tokenValue}
		newTOTP, err := f.svc.TotpEnable(ctx, f.id, req)
		require.NoError(t, err)
		require.NotEmpty(t, newTOTP.Secret)
		_, err = f.svc.TotpEnable(ctx, f.id, req)
		require.ErrorIs(t, err, reauth.ErrInvalidReauthToken, "the token is single-use")
	})
}

func ptr[T any](v T) *T { return &v }

func (f *totpFixture) addToken(token model.Token) {
	f.repo.mu.Lock()
	defer f.repo.mu.Unlock()
	acc := f.repo.accounts[f.id]
	acc.Tokens = append(acc.Tokens, token)
	f.repo.accounts[f.id] = acc
}
